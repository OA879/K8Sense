package checks

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/OA879/K8Sense/backend/pkg/clusterdoctor"
)

// runCheck looks up a registered check by name and runs it against a fake
// clientset seeded with objs.
func runCheck(t *testing.T, checkFn string, objs ...runtime.Object) []clusterdoctor.RawFinding {
	t.Helper()

	fn, ok := clusterdoctor.GetCheck(checkFn)
	if !ok {
		t.Fatalf("check %q not registered", checkFn)
	}

	client := fake.NewSimpleClientset(objs...)

	findings, err := fn(context.Background(), client)
	if err != nil {
		t.Fatalf("check %q errored: %v", checkFn, err)
	}

	return findings
}

func TestCheckNodeNotReady(t *testing.T) {
	notReady := node("bad", corev1.NodeCondition{Type: corev1.NodeReady, Status: corev1.ConditionFalse})
	ready := node("good", corev1.NodeCondition{Type: corev1.NodeReady, Status: corev1.ConditionTrue})

	got := runCheck(t, "check_node_not_ready", notReady, ready)
	if len(got) != 1 || got[0].ResourceName != "bad" {
		t.Fatalf("expected 1 finding for 'bad', got %+v", got)
	}
}

func TestCheckCrashLoopBackOff(t *testing.T) {
	crashing := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "crasher", Namespace: "demo"},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:  "app",
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
			}},
		},
	}
	healthy := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "ok", Namespace: "demo"},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}

	got := runCheck(t, "check_crashloopbackoff", crashing, healthy)
	if len(got) != 1 || got[0].ResourceName != "crasher" || got[0].Namespace != "demo" {
		t.Fatalf("expected 1 finding for 'crasher', got %+v", got)
	}
}

func TestCheckPVCPending(t *testing.T) {
	pending := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "stuck", Namespace: "demo"},
		Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimPending},
	}
	bound := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "fine", Namespace: "demo"},
		Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
	}

	got := runCheck(t, "check_pvc_pending", pending, bound)
	if len(got) != 1 || got[0].ResourceName != "stuck" {
		t.Fatalf("expected 1 finding for 'stuck', got %+v", got)
	}
}

func TestCheckDeploymentZeroAvailable(t *testing.T) {
	three := int32(3)
	down := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "down", Namespace: "demo"},
		Spec:       appsv1.DeploymentSpec{Replicas: &three},
		Status:     appsv1.DeploymentStatus{AvailableReplicas: 0},
	}
	up := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "up", Namespace: "demo"},
		Spec:       appsv1.DeploymentSpec{Replicas: &three},
		Status:     appsv1.DeploymentStatus{AvailableReplicas: 3},
	}

	got := runCheck(t, "check_deployment_zero_available", down, up)
	if len(got) != 1 || got[0].ResourceName != "down" {
		t.Fatalf("expected 1 finding for 'down', got %+v", got)
	}
}

func TestCheckEvictedPods(t *testing.T) {
	evicted := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "evicted-1", Namespace: "demo"},
		Status:     corev1.PodStatus{Phase: corev1.PodFailed, Reason: "Evicted"},
	}

	got := runCheck(t, "check_evicted_pods", evicted)
	if len(got) != 1 || got[0].ResourceName != "evicted-1" {
		t.Fatalf("expected 1 evicted finding, got %+v", got)
	}
}

func TestCheckNodeCordoned(t *testing.T) {
	cordoned := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "cord"},
		Spec:       corev1.NodeSpec{Unschedulable: true},
	}

	got := runCheck(t, "check_node_cordoned", cordoned)
	if len(got) != 1 || got[0].ResourceName != "cord" {
		t.Fatalf("expected 1 cordoned finding, got %+v", got)
	}
}

func TestCheckMissingResourceLimits(t *testing.T) {
	noLimits := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "greedy", Namespace: "demo"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c"}}},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}

	got := runCheck(t, "check_missing_resource_limits", noLimits)
	if len(got) != 1 {
		t.Fatalf("expected 1 missing-limits finding, got %+v", got)
	}
}

// --- helpers ---

func node(name string, cond corev1.NodeCondition) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status:     corev1.NodeStatus{Conditions: []corev1.NodeCondition{cond}},
	}
}
