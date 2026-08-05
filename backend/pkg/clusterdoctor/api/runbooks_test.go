package api

import (
	"context"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestRunbookTemplates_WellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, tmpl := range runbookTemplates() {
		if tmpl.ID == "" || tmpl.Name == "" || tmpl.playbook == "" {
			t.Errorf("template %+v incomplete", tmpl)
		}
		if !strings.Contains(tmpl.playbook, "connection: local") {
			t.Errorf("%s must run connection: local", tmpl.ID)
		}
		if seen[tmpl.ID] {
			t.Errorf("duplicate template id %q", tmpl.ID)
		}
		seen[tmpl.ID] = true

		got, ok := findRunbook(tmpl.ID)
		if !ok || got.Name != tmpl.Name {
			t.Errorf("findRunbook(%q) did not round-trip", tmpl.ID)
		}
	}
}

func TestRunnerJob_Shape(t *testing.T) {
	labels := map[string]string{"k8sense.io/runbook": "x"}
	job := runnerJob("k8sense-rb-x-1", "img:1", true, labels)

	spec := job.Spec.Template.Spec
	if spec.ServiceAccountName != runnerSAName {
		t.Errorf("SA = %q, want %q", spec.ServiceAccountName, runnerSAName)
	}
	if spec.RestartPolicy != "Never" {
		t.Errorf("restartPolicy = %q", spec.RestartPolicy)
	}
	if len(spec.Containers) != 1 || spec.Containers[0].Image != "img:1" {
		t.Fatalf("bad container: %+v", spec.Containers)
	}
	cmd := strings.Join(spec.Containers[0].Command, " ")
	if !strings.Contains(cmd, "ansible-playbook") || !strings.Contains(cmd, "--check") {
		t.Errorf("check-mode command wrong: %s", cmd)
	}
	// non-check job must NOT carry --check
	plain := runnerJob("k8sense-rb-x-2", "img:1", false, labels)
	if strings.Contains(strings.Join(plain.Spec.Template.Spec.Containers[0].Command, " "), "--check") {
		t.Error("non-check run should not pass --check")
	}
}

func TestBootstrapRunner_CreatesRBAC_Idempotent(t *testing.T) {
	clientset := k8sfake.NewSimpleClientset()
	req := reqWithCtx()

	if err := bootstrapRunner(req, clientset); err != nil {
		t.Fatal(err)
	}
	// Running twice must not error (AlreadyExists is success).
	if err := bootstrapRunner(req, clientset); err != nil {
		t.Fatalf("second enable should be idempotent: %v", err)
	}

	if _, err := clientset.CoreV1().ServiceAccounts(runbookNamespace).Get(context.Background(), runnerSAName, metav1.GetOptions{}); err != nil {
		t.Errorf("runner SA not created: %v", err)
	}
	if _, err := clientset.RbacV1().ClusterRoleBindings().Get(context.Background(), runnerRoleName, metav1.GetOptions{}); err != nil {
		t.Errorf("runner ClusterRoleBinding not created: %v", err)
	}
}
