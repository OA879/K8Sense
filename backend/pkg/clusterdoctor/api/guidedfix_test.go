package api_test

import (
	"context"
	"net/http"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cddb "github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor/db"
)

func testPod(namespace, name string) *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
}

func testNode(name string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       corev1.NodeSpec{Unschedulable: true},
	}
}

func testDeployment(namespace, name string) *appsv1.Deployment {
	replicas := int32(1)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
	}
}

// TestGuidedFixRejectsUnconfirmed is the "explicit human intent" gate: the
// server must never act on a request that didn't come from the confirmation
// modal, even if everything else about it is valid.
func TestGuidedFixRejectsUnconfirmed(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t, testPod("demo", "doomed"))
	env.grantPro()

	rec := env.do(http.MethodPost, "/cluster-doctor/guided-fix", map[string]any{
		"cluster": testCluster, "action": "delete_pod",
		"namespace": "demo", "resourceName": "doomed",
		// confirmed deliberately omitted
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unconfirmed guided fix: got %d, want 400", rec.Code)
	}

	// The pod must still exist — an unconfirmed request must have no effect.
	if _, err := env.clientset().CoreV1().Pods("demo").
		Get(context.Background(), "doomed", metav1.GetOptions{}); err != nil {
		t.Errorf("pod was deleted despite unconfirmed request: %v", err)
	}
}

// TestGuidedFixRejectsDisallowedActions locks the allowlist. Anything not
// explicitly safe (drain, etcd surgery, secret edits, arbitrary delete) must
// be refused outright rather than attempted.
func TestGuidedFixRejectsDisallowedActions(t *testing.T) {
	t.Parallel()

	dangerous := []string{
		"drain_node", "delete_pvc", "delete_namespace", "edit_secret",
		"rotate_certs", "delete_node", "exec", "", "DELETE_POD",
	}

	for _, action := range dangerous {
		t.Run("action="+action, func(t *testing.T) {
			t.Parallel()

			env := newTestEnv(t, testPod("demo", "p"))
			env.grantPro()

			rec := env.do(http.MethodPost, "/cluster-doctor/guided-fix", map[string]any{
				"cluster": testCluster, "action": action,
				"namespace": "demo", "resourceName": "p", "confirmed": true,
			})

			if rec.Code != http.StatusForbidden {
				t.Errorf("action %q: got %d, want 403", action, rec.Code)
			}
		})
	}
}

func TestGuidedFixDeletesPodAndWritesAudit(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t, testPod("demo", "doomed"))
	env.grantPro()

	rec := env.do(http.MethodPost, "/cluster-doctor/guided-fix", map[string]any{
		"cluster": testCluster, "action": "delete_pod",
		"namespace": "demo", "resourceName": "doomed", "confirmed": true,
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("guided fix: got %d, body %s", rec.Code, rec.Body.String())
	}

	// The pod is really gone from the cluster.
	_, err := env.clientset().CoreV1().Pods("demo").
		Get(context.Background(), "doomed", metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		t.Errorf("expected pod to be deleted, got err=%v", err)
	}

	// And a success audit entry was written.
	entries, err := cddb.ListAudit(context.Background(), env.db, testCluster, 10)
	if err != nil {
		t.Fatalf("listing audit: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("got %d audit entries, want 1", len(entries))
	}

	if entries[0].Result != "success" || entries[0].Action != "delete_pod" {
		t.Errorf("audit entry = %+v, want success/delete_pod", entries[0])
	}

	if entries[0].ResourceName != "doomed" {
		t.Errorf("audit resourceName = %q, want doomed", entries[0].ResourceName)
	}
}

// TestGuidedFixWritesAuditOnFailure is the compliance-critical case: a failed
// action must still leave a trail. Silent failures are worse than loud ones.
func TestGuidedFixWritesAuditOnFailure(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t) // no pods at all
	env.grantPro()

	rec := env.do(http.MethodPost, "/cluster-doctor/guided-fix", map[string]any{
		"cluster": testCluster, "action": "delete_pod",
		"namespace": "demo", "resourceName": "ghost", "confirmed": true,
	})

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("deleting a missing pod: got %d, want 422 (body %s)", rec.Code, rec.Body.String())
	}

	entries, err := cddb.ListAudit(context.Background(), env.db, testCluster, 10)
	if err != nil {
		t.Fatalf("listing audit: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("got %d audit entries after a failure, want 1", len(entries))
	}

	if entries[0].Result != "failed" {
		t.Errorf("audit result = %q, want failed", entries[0].Result)
	}

	if entries[0].Error == "" {
		t.Error("failed audit entry should record the error")
	}
}

func TestGuidedFixUncordonsNode(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t, testNode("node-1"))
	env.grantPro()

	rec := env.do(http.MethodPost, "/cluster-doctor/guided-fix", map[string]any{
		"cluster": testCluster, "action": "uncordon_node",
		"resourceName": "node-1", "confirmed": true,
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("uncordon: got %d, body %s", rec.Code, rec.Body.String())
	}

	node, err := env.clientset().CoreV1().Nodes().
		Get(context.Background(), "node-1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("getting node: %v", err)
	}

	if node.Spec.Unschedulable {
		t.Error("node is still cordoned after uncordon_node")
	}
}

func TestGuidedFixScalesDeployment(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t, testDeployment("demo", "web"))
	env.grantPro()

	rec := env.do(http.MethodPost, "/cluster-doctor/guided-fix", map[string]any{
		"cluster": testCluster, "action": "scale_deployment",
		"namespace": "demo", "resourceName": "web", "replicas": 3, "confirmed": true,
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("scale: got %d, body %s", rec.Code, rec.Body.String())
	}

	dep, err := env.clientset().AppsV1().Deployments("demo").
		Get(context.Background(), "web", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("getting deployment: %v", err)
	}

	if dep.Spec.Replicas == nil || *dep.Spec.Replicas != 3 {
		t.Errorf("replicas = %v, want 3", dep.Spec.Replicas)
	}
}

// TestGuidedFixScaleRequiresReplicas guards against a nil-pointer scale that
// would otherwise silently scale a deployment to zero.
func TestGuidedFixScaleRequiresReplicas(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t, testDeployment("demo", "web"))
	env.grantPro()

	rec := env.do(http.MethodPost, "/cluster-doctor/guided-fix", map[string]any{
		"cluster": testCluster, "action": "scale_deployment",
		"namespace": "demo", "resourceName": "web", "confirmed": true,
	})

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("scale without replicas: got %d, want 422", rec.Code)
	}

	dep, err := env.clientset().AppsV1().Deployments("demo").
		Get(context.Background(), "web", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("getting deployment: %v", err)
	}

	if dep.Spec.Replicas == nil || *dep.Spec.Replicas != 1 {
		t.Errorf("replicas changed to %v; should be untouched at 1", dep.Spec.Replicas)
	}
}

func TestGuidedFixUnknownClusterIsNotFound(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.grantPro()

	rec := env.do(http.MethodPost, "/cluster-doctor/guided-fix", map[string]any{
		"cluster": "no-such-cluster", "action": "delete_pod",
		"namespace": "demo", "resourceName": "p", "confirmed": true,
	})

	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown cluster: got %d, want 404", rec.Code)
	}
}
