package checks

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor"
)

const controlPlaneRestartWindow = time.Hour

// controlPlaneComponents identifies static control-plane pods by the value of
// their `component` label (kubeadm) — the same set K8sense health-checks for
// CP-*. Managed control planes (EKS/GKE/AKS) don't expose these as pods, so
// these checks simply find nothing there, which is correct.
var controlPlaneComponents = map[string]bool{
	"etcd":                    true,
	"kube-apiserver":          true,
	"kube-scheduler":          true,
	"kube-controller-manager": true,
}

func init() {
	clusterdoctor.RegisterCheck("check_control_plane_pod_not_ready", checkControlPlanePodNotReady)
	clusterdoctor.RegisterCheck("check_control_plane_pod_restarted", checkControlPlanePodRestarted)
}

// isControlPlanePod reports whether pod is one of the well-known static
// control-plane components, matching either the `component` label or the
// conventional name prefix used by kubeadm/kind.
func isControlPlanePod(pod corev1.Pod) (string, bool) {
	if comp := pod.Labels["component"]; controlPlaneComponents[comp] {
		return comp, true
	}

	for comp := range controlPlaneComponents {
		if strings.HasPrefix(pod.Name, comp+"-") {
			return comp, true
		}
	}

	return "", false
}

func checkControlPlanePodNotReady(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	pods, err := clientset.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, pod := range pods.Items {
		comp, ok := isControlPlanePod(pod)
		if !ok {
			continue
		}

		if !podIsReady(pod) {
			findings = append(findings, clusterdoctor.RawFinding{
				Namespace:    pod.Namespace,
				ResourceKind: "Pod",
				ResourceName: pod.Name,
				RawObject:    fmt.Sprintf(`{"component": %q, "phase": %q}`, comp, pod.Status.Phase),
			})
		}
	}

	return findings, nil
}

func podIsReady(pod corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady {
			return cond.Status == corev1.ConditionTrue
		}
	}

	return false
}

func checkControlPlanePodRestarted(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	pods, err := clientset.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, pod := range pods.Items {
		comp, ok := isControlPlanePod(pod)
		if !ok {
			continue
		}

		for _, cs := range pod.Status.ContainerStatuses {
			term := cs.LastTerminationState.Terminated
			if term == nil {
				continue
			}

			if time.Since(term.FinishedAt.Time) < controlPlaneRestartWindow {
				findings = append(findings, clusterdoctor.RawFinding{
					Namespace:    pod.Namespace,
					ResourceKind: "Pod",
					ResourceName: pod.Name,
					RawObject: fmt.Sprintf(
						`{"component": %q, "restartCount": %d, "lastRestart": %q}`,
						comp, cs.RestartCount, term.FinishedAt.Format(time.RFC3339),
					),
				})

				break
			}
		}
	}

	return findings, nil
}
