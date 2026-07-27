package api

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func deployWithLabels(ns, name string, podLabels map[string]string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: podLabels},
			},
		},
	}
}

func TestBuildNetworkMap_PolicyExposureAndEdges(t *testing.T) {
	frontend := deployWithLabels("shop", "frontend", map[string]string{"app": "frontend"})
	backend := deployWithLabels("shop", "backend", map[string]string{"app": "backend"})

	// Only "backend" is protected, and only "frontend" may reach it.
	policy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "backend-allow-frontend"},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"app": "backend"}},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress: []networkingv1.NetworkPolicyIngressRule{{
				From: []networkingv1.NetworkPolicyPeer{{
					PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "frontend"}},
				}},
			}},
		},
	}

	cs := k8sfake.NewSimpleClientset(frontend, backend, policy)

	m, err := buildNetworkMap(context.Background(), cs, "")
	if err != nil {
		t.Fatal(err)
	}

	exposure := map[string]string{}
	for _, n := range m.Nodes {
		exposure[n.Name] = n.Exposure
	}

	if exposure["frontend"] != "open" {
		t.Errorf("frontend exposure = %q, want open (no policy selects it)", exposure["frontend"])
	}

	if exposure["backend"] != "restricted" {
		t.Errorf("backend exposure = %q, want restricted", exposure["backend"])
	}

	if len(m.Edges) != 1 {
		t.Fatalf("got %d policy edges, want 1 (frontend->backend)", len(m.Edges))
	}

	if m.Edges[0].Source != "Deployment/shop/frontend" || m.Edges[0].Target != "Deployment/shop/backend" {
		t.Errorf("edge = %s -> %s, want frontend -> backend", m.Edges[0].Source, m.Edges[0].Target)
	}

	// No mesh Prometheus in a fake cluster → overlay absent, request still fine.
	if m.Mesh.Enabled {
		t.Errorf("mesh should be disabled without a reachable Prometheus")
	}
}

func TestBuildNetworkMap_IsolatedAndDefaultAllow(t *testing.T) {
	// A default-deny policy (selects all, no ingress rules) isolates the workload.
	denyAll := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Namespace: "locked", Name: "default-deny"},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{}, // empty = all pods
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
		},
	}
	locked := deployWithLabels("locked", "vault", map[string]string{"app": "vault"})
	wideOpen := deployWithLabels("public", "web", map[string]string{"app": "web"})

	cs := k8sfake.NewSimpleClientset(denyAll, locked, wideOpen)

	m, err := buildNetworkMap(context.Background(), cs, "")
	if err != nil {
		t.Fatal(err)
	}

	exposure := map[string]string{}
	for _, n := range m.Nodes {
		exposure[n.Name] = n.Exposure
	}

	if exposure["vault"] != "isolated" {
		t.Errorf("vault exposure = %q, want isolated (default-deny)", exposure["vault"])
	}

	if exposure["web"] != "open" {
		t.Errorf("web exposure = %q, want open (no policy in its namespace)", exposure["web"])
	}
}
