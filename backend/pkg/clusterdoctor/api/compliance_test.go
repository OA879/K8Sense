package api

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func control(report complianceReport, id string) *controlResult {
	for i := range report.Controls {
		if report.Controls[i].ID == id {
			return &report.Controls[i]
		}
	}
	return nil
}

func TestCompliance_FlagsInsecurePod(t *testing.T) {
	tru := true
	bad := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "app", Name: "bad"},
		Spec: corev1.PodSpec{
			HostNetwork: true,
			Containers: []corev1.Container{{
				Name:            "c",
				SecurityContext: &corev1.SecurityContext{Privileged: &tru},
			}},
		},
	}
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "risky"},
		RoleRef:    rbacv1.RoleRef{Name: "cluster-admin"},
	}

	report, err := buildComplianceReport(context.Background(), k8sfake.NewSimpleClientset(bad, crb), "")
	if err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"CIS-5.2.1", "CIS-5.2.4", "CIS-5.3.2", "CIS-5.1.1"} {
		c := control(report, id)
		if c == nil || c.Status != "fail" {
			t.Errorf("%s should FAIL for the insecure pod/binding, got %+v", id, c)
		}
	}
	if report.Score >= 100 || report.Failed == 0 {
		t.Errorf("score=%d failed=%d, expected failures", report.Score, report.Failed)
	}
}

func TestCompliance_HardenedPodPasses(t *testing.T) {
	tru, fal := true, false
	good := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "secure", Name: "app"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "c",
				SecurityContext: &corev1.SecurityContext{
					Privileged:               &fal,
					AllowPrivilegeEscalation: &fal,
					RunAsNonRoot:             &tru,
					Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
				},
			}},
		},
	}
	np := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Namespace: "secure", Name: "default-deny"},
	}

	report, err := buildComplianceReport(context.Background(), k8sfake.NewSimpleClientset(good, np), "")
	if err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"CIS-5.2.1", "CIS-5.2.4", "CIS-5.2.5", "CIS-5.2.6", "CIS-5.2.9", "CIS-5.3.2", "CIS-5.7.4"} {
		c := control(report, id)
		if c == nil || c.Status != "pass" {
			t.Errorf("%s should PASS for the hardened pod, got %+v", id, c)
		}
	}
}
