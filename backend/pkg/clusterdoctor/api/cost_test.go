package api

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestBuildCostReport_WasteAndTotals(t *testing.T) {
	// LoadBalancer WITH a ready endpoint -> not wasteful.
	activeLB := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "web"},
		Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
	}
	activeEP := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "web"},
		Subsets:    []corev1.EndpointSubset{{Addresses: []corev1.EndpointAddress{{IP: "10.0.0.1"}}}},
	}
	// LoadBalancer with NO endpoints -> $18 wasted.
	idleLB := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "stale-lb"},
		Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
	}
	// ClusterIP -> ignored.
	internal := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "internal"},
		Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP},
	}

	// PVC mounted by a pod -> not wasteful.
	usedPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "db-data"},
		Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "db-0"},
		Spec: corev1.PodSpec{Volumes: []corev1.Volume{{
			Name:         "d",
			VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "db-data"}},
		}}},
	}
	// Bound PVC, 10Gi, NOT mounted -> $1.00 (10 * 0.10).
	orphanPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "lost-data"},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase:    corev1.ClaimBound,
			Capacity: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")},
		},
	}
	// Released PV, 50Gi -> $5.00.
	releasedPV := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pv-released"},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("50Gi")},
		},
		Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeReleased},
	}

	cs := k8sfake.NewSimpleClientset(activeLB, activeEP, idleLB, internal, usedPVC, pod, orphanPVC, releasedPV)

	report, err := buildCostReport(context.Background(), cs, "")
	if err != nil {
		t.Fatal(err)
	}

	cats := map[string]costItem{}
	for _, it := range report.Items {
		cats[it.Category] = it
	}

	if _, ok := cats["idle-loadbalancer"]; !ok || cats["idle-loadbalancer"].ResourceName != "stale-lb" {
		t.Errorf("expected idle-loadbalancer for stale-lb, got %+v", report.Items)
	}
	if _, ok := cats["unused-pvc"]; !ok || cats["unused-pvc"].ResourceName != "lost-data" {
		t.Errorf("expected unused-pvc for lost-data, got %+v", report.Items)
	}
	if _, ok := cats["orphaned-pv"]; !ok {
		t.Errorf("expected orphaned-pv, got %+v", report.Items)
	}
	if len(report.Items) != 3 {
		t.Fatalf("got %d waste items, want 3 (active LB + used PVC must be excluded)", len(report.Items))
	}

	// 18 (LB) + 1.00 (10Gi PVC) + 5.00 (50Gi PV) = 24.00
	if report.EstMonthlyWasteUSD < 23.99 || report.EstMonthlyWasteUSD > 24.01 {
		t.Fatalf("est waste = %.2f, want ~24.00", report.EstMonthlyWasteUSD)
	}
	// Items sorted by cost desc: LB ($18) first.
	if report.Items[0].Category != "idle-loadbalancer" {
		t.Errorf("items should be cost-sorted desc; first = %s", report.Items[0].Category)
	}
}
