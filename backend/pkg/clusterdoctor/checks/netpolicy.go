package checks

import (
	"context"
	"fmt"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"

	"github.com/OA879/K8Sense/backend/pkg/clusterdoctor"
)

// NetworkPolicy hygiene checks — the security half of the Network Map, surfaced
// as scan findings. Both work from the plain Kubernetes API (no service mesh).

func init() {
	clusterdoctor.RegisterCheck("check_workload_no_networkpolicy", checkWorkloadNoNetworkPolicy)
	clusterdoctor.RegisterCheck("check_namespace_no_default_deny", checkNamespaceNoDefaultDeny)
}

// ingressApplies mirrors the API rule: Ingress is in effect unless PolicyTypes
// is explicitly Egress-only.
func ingressApplies(p networkingv1.NetworkPolicy) bool {
	if len(p.Spec.PolicyTypes) == 0 {
		return true
	}

	for _, t := range p.Spec.PolicyTypes {
		if t == networkingv1.PolicyTypeIngress {
			return true
		}
	}

	return false
}

func labelSelectorMatches(sel metav1.LabelSelector, l map[string]string) bool {
	s, err := metav1.LabelSelectorAsSelector(&sel)
	if err != nil {
		return false
	}

	return s.Matches(labels.Set(l))
}

// coveredByIngressPolicy reports whether some ingress policy in ns selects the
// given pod labels.
func coveredByIngressPolicy(ns string, podLabels map[string]string, policies []networkingv1.NetworkPolicy) bool {
	for _, p := range policies {
		if p.Namespace == ns && ingressApplies(p) && labelSelectorMatches(p.Spec.PodSelector, podLabels) {
			return true
		}
	}

	return false
}

// NET-007 — a workload whose pods are not selected by any ingress NetworkPolicy,
// so anything in the cluster may connect to it (default-allow).
func checkWorkloadNoNetworkPolicy(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	policies, err := clientset.NetworkingV1().NetworkPolicies(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	add := func(ns, name, kind string, podLabels map[string]string) {
		if systemNamespaces[ns] {
			return
		}

		if !coveredByIngressPolicy(ns, podLabels, policies.Items) {
			findings = append(findings, clusterdoctor.RawFinding{
				Namespace: ns, ResourceKind: kind, ResourceName: name,
				RawObject: fmt.Sprintf(`{"exposure": "default-allow"}`),
			})
		}
	}

	deploys, err := clientset.AppsV1().Deployments(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, d := range deploys.Items {
		add(d.Namespace, d.Name, "Deployment", d.Spec.Template.Labels)
	}

	stateful, err := clientset.AppsV1().StatefulSets(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, s := range stateful.Items {
		add(s.Namespace, s.Name, "StatefulSet", s.Spec.Template.Labels)
	}

	daemon, err := clientset.AppsV1().DaemonSets(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, d := range daemon.Items {
		add(d.Namespace, d.Name, "DaemonSet", d.Spec.Template.Labels)
	}

	return findings, nil
}

// isDefaultDenyIngress is true for a policy that selects every pod and permits
// no ingress — the recommended baseline that denies all inbound by default.
func isDefaultDenyIngress(p networkingv1.NetworkPolicy) bool {
	selectsAll := len(p.Spec.PodSelector.MatchLabels) == 0 && len(p.Spec.PodSelector.MatchExpressions) == 0

	return selectsAll && ingressApplies(p) && len(p.Spec.Ingress) == 0
}

// NET-008 — a namespace that runs workloads but has no default-deny ingress
// baseline, so its pods are open unless individually covered.
func checkNamespaceNoDefaultDeny(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	policies, err := clientset.NetworkingV1().NetworkPolicies(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	hasDefaultDeny := map[string]bool{}
	for _, p := range policies.Items {
		if isDefaultDenyIngress(p) {
			hasDefaultDeny[p.Namespace] = true
		}
	}

	deploys, err := clientset.AppsV1().Deployments(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	nsWithWorkloads := map[string]bool{}
	for _, d := range deploys.Items {
		if !systemNamespaces[d.Namespace] {
			nsWithWorkloads[d.Namespace] = true
		}
	}

	var findings []clusterdoctor.RawFinding

	for ns := range nsWithWorkloads {
		if !hasDefaultDeny[ns] {
			findings = append(findings, clusterdoctor.RawFinding{
				Namespace: ns, ResourceKind: "Namespace", ResourceName: ns,
				RawObject: fmt.Sprintf(`{"baseline": "missing default-deny ingress"}`),
			})
		}
	}

	return findings, nil
}
