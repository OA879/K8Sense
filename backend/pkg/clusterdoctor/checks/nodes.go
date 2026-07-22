// Package checks implements clusterdoctor.CheckFunc functions and registers
// them under the check_fn name used by rules/*.yaml. Importing this package
// for its side effects (via a blank import) is what makes rules runnable.
package checks

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/OA879/K8Sense/backend/pkg/clusterdoctor"
)

const overCommitThresholdPercent = 90

// wellKnownTaints are taints the control plane or cluster autoscaler applies
// as part of normal operation. NODE-007 only flags taints outside this set,
// since flagging every control-plane node's own taint would be pure noise.
var wellKnownTaints = map[string]bool{
	"node-role.kubernetes.io/control-plane": true,
	"node-role.kubernetes.io/master":        true,
	"node.kubernetes.io/unschedulable":      true,
	"node.kubernetes.io/not-ready":          true,
	"node.kubernetes.io/unreachable":        true,
	"node.kubernetes.io/memory-pressure":    true,
	"node.kubernetes.io/disk-pressure":      true,
	"node.kubernetes.io/pid-pressure":       true,
	"node.kubernetes.io/network-unavailable": true,
}

func init() {
	clusterdoctor.RegisterCheck("check_node_not_ready", checkNodeCondition(corev1.NodeReady, corev1.ConditionFalse))
	clusterdoctor.RegisterCheck("check_node_unknown", checkNodeCondition(corev1.NodeReady, corev1.ConditionUnknown))
	clusterdoctor.RegisterCheck("check_node_memory_pressure", checkNodeCondition(corev1.NodeMemoryPressure, corev1.ConditionTrue))
	clusterdoctor.RegisterCheck("check_node_disk_pressure", checkNodeCondition(corev1.NodeDiskPressure, corev1.ConditionTrue))
	clusterdoctor.RegisterCheck("check_node_pid_pressure", checkNodeCondition(corev1.NodePIDPressure, corev1.ConditionTrue))
	clusterdoctor.RegisterCheck("check_node_network_unavailable", checkNodeCondition(corev1.NodeNetworkUnavailable, corev1.ConditionTrue))
	clusterdoctor.RegisterCheck("check_node_unexpected_taint", checkNodeUnexpectedTaint)
	clusterdoctor.RegisterCheck("check_node_cordoned", checkNodeCordoned)
	clusterdoctor.RegisterCheck("check_node_cpu_overcommit", checkNodeOvercommit(corev1.ResourceCPU))
	clusterdoctor.RegisterCheck("check_node_memory_overcommit", checkNodeOvercommit(corev1.ResourceMemory))
	clusterdoctor.RegisterCheck("check_node_kubelet_version_skew", checkNodeKubeletVersionSkew)
	clusterdoctor.RegisterCheck("check_node_pod_capacity", checkNodePodCapacity)
}

// checkNodeCondition returns a CheckFunc that flags every node whose
// condition condType currently reports status.
func checkNodeCondition(condType corev1.NodeConditionType, status corev1.ConditionStatus) clusterdoctor.CheckFunc {
	return func(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
		nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		var findings []clusterdoctor.RawFinding

		for _, node := range nodes.Items {
			for _, cond := range node.Status.Conditions {
				if cond.Type == condType && cond.Status == status {
					findings = append(findings, clusterdoctor.RawFinding{
						ResourceKind: "Node",
						ResourceName: node.Name,
					})

					break
				}
			}
		}

		return findings, nil
	}
}

func checkNodeUnexpectedTaint(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, node := range nodes.Items {
		for _, taint := range node.Spec.Taints {
			if wellKnownTaints[taint.Key] {
				continue
			}

			if taint.Effect == corev1.TaintEffectNoSchedule || taint.Effect == corev1.TaintEffectNoExecute {
				findings = append(findings, clusterdoctor.RawFinding{
					ResourceKind: "Node",
					ResourceName: node.Name,
				})

				break
			}
		}
	}

	return findings, nil
}

func checkNodeCordoned(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, node := range nodes.Items {
		if node.Spec.Unschedulable {
			findings = append(findings, clusterdoctor.RawFinding{
				ResourceKind: "Node",
				ResourceName: node.Name,
			})
		}
	}

	return findings, nil
}

// checkNodeOvercommit compares the sum of every non-terminal pod's resource
// *requests* on a node against that node's allocatable capacity. This needs
// no metrics-server: requests are always present on the API objects
// Cluster Doctor already lists.
func checkNodeOvercommit(resourceName corev1.ResourceName) clusterdoctor.CheckFunc {
	return func(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
		nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		pods, err := clientset.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		requestedByNode := map[string]int64{}

		for _, pod := range pods.Items {
			if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
				continue
			}

			for _, c := range pod.Spec.Containers {
				if qty, ok := c.Resources.Requests[resourceName]; ok {
					requestedByNode[pod.Spec.NodeName] += qty.MilliValue()
				}
			}
		}

		var findings []clusterdoctor.RawFinding

		for _, node := range nodes.Items {
			allocatable, ok := node.Status.Allocatable[resourceName]
			if !ok || allocatable.MilliValue() == 0 {
				continue
			}

			requested := requestedByNode[node.Name]
			percent := requested * 100 / allocatable.MilliValue()

			if percent > overCommitThresholdPercent {
				findings = append(findings, clusterdoctor.RawFinding{
					ResourceKind: "Node",
					ResourceName: node.Name,
					RawObject:    fmt.Sprintf(`{"requestedPercent": %d}`, percent),
				})
			}
		}

		return findings, nil
	}
}

const kubeletVersionSkewMinorLimit = 2

func checkNodeKubeletVersionSkew(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	serverVersion, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	serverMinor := parseMinorVersion(serverVersion.Minor)

	var findings []clusterdoctor.RawFinding

	for _, node := range nodes.Items {
		kubeletMinor := parseMinorVersion(extractMinor(node.Status.NodeInfo.KubeletVersion))
		if kubeletMinor < 0 || serverMinor < 0 {
			continue
		}

		if serverMinor-kubeletMinor > kubeletVersionSkewMinorLimit {
			findings = append(findings, clusterdoctor.RawFinding{
				ResourceKind: "Node",
				ResourceName: node.Name,
				RawObject: fmt.Sprintf(
					`{"kubeletVersion": %q, "serverVersion": %q}`,
					node.Status.NodeInfo.KubeletVersion, serverVersion.GitVersion,
				),
			})
		}
	}

	return findings, nil
}

const nodePodCapacityThresholdPercent = 85

func checkNodePodCapacity(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	pods, err := clientset.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	countByNode := map[string]int{}

	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}

		countByNode[pod.Spec.NodeName]++
	}

	var findings []clusterdoctor.RawFinding

	for _, node := range nodes.Items {
		capacity, ok := node.Status.Allocatable[corev1.ResourcePods]
		if !ok || capacity.Value() == 0 {
			continue
		}

		percent := int64(countByNode[node.Name]) * 100 / capacity.Value()
		if percent > nodePodCapacityThresholdPercent {
			findings = append(findings, clusterdoctor.RawFinding{
				ResourceKind: "Node",
				ResourceName: node.Name,
			})
		}
	}

	return findings, nil
}

// extractMinor pulls the numeric-ish minor version out of a kubelet version
// string like "v1.29.4" -> "29".
func extractMinor(kubeletVersion string) string {
	v := strings.TrimPrefix(kubeletVersion, "v")

	parts := strings.SplitN(v, ".", 3) //nolint:mnd // major.minor.patch
	if len(parts) < 2 {                //nolint:mnd
		return ""
	}

	return parts[1]
}

func parseMinorVersion(s string) int {
	s = strings.TrimFunc(s, func(r rune) bool { return r < '0' || r > '9' })
	if s == "" {
		return -1
	}

	n := 0
	for _, r := range s {
		n = n*10 + int(r-'0') //nolint:mnd
	}

	return n
}
