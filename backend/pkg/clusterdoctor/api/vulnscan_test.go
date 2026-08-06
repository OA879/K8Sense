package api

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestBuildVulnReport_ParsesAndRanks(t *testing.T) {
	// Two image blocks: one with vulns, one that failed to scan.
	logs := `@@IMAGE nginx:1.0
{"Results":[{"Vulnerabilities":[
  {"VulnerabilityID":"CVE-1","PkgName":"openssl","InstalledVersion":"1.0","FixedVersion":"1.1","Severity":"CRITICAL","Title":"bad"},
  {"VulnerabilityID":"CVE-2","PkgName":"zlib","InstalledVersion":"1.2","Severity":"MEDIUM"}
]}]}
@@END
@@IMAGE broken:latest
{"__scan_error__":true}
@@END
`
	ns := map[string][]string{"nginx:1.0": {"prod", "web"}}

	report := buildVulnReport(logs, ns)

	if report.Totals["CRITICAL"] != 1 || report.Totals["MEDIUM"] != 1 {
		t.Errorf("totals wrong: %+v", report.Totals)
	}
	if len(report.Images) != 2 {
		t.Fatalf("expected 2 image results, got %d", len(report.Images))
	}
	// Most-critical image should sort first.
	first := report.Images[0]
	if first.Image != "nginx:1.0" {
		t.Errorf("expected nginx first, got %s", first.Image)
	}
	if first.Counts["CRITICAL"] != 1 || len(first.Vulns) != 2 {
		t.Errorf("nginx result wrong: %+v", first)
	}
	if first.Vulns[0].Severity != "CRITICAL" {
		t.Errorf("vulns should be severity-sorted, got %s first", first.Vulns[0].Severity)
	}
	if len(first.Namespaces) != 2 {
		t.Errorf("namespaces not attached: %v", first.Namespaces)
	}
	// The broken image should carry an error, no vulns.
	broken := report.Images[1]
	if broken.Error == "" || len(broken.Vulns) != 0 {
		t.Errorf("broken image should report an error: %+v", broken)
	}
}

func TestGatherImages_DedupesAcrossNamespaces(t *testing.T) {
	mk := func(ns, name, image string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
			Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: image}}},
		}
	}
	cs := k8sfake.NewSimpleClientset(
		mk("prod", "a", "nginx:1"),
		mk("web", "b", "nginx:1"),
		mk("prod", "c", "redis:7"),
	)

	images, err := gatherImages(context.Background(), cs)
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 2 {
		t.Fatalf("expected 2 unique images, got %d (%v)", len(images), images)
	}
	if len(images["nginx:1"]) != 2 {
		t.Errorf("nginx:1 should run in 2 namespaces, got %v", images["nginx:1"])
	}
}

func TestPodTrouble_DetectsImagePullBackOff(t *testing.T) {
	stuck := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: vulnNamespace, Name: "job-pod",
			Labels: map[string]string{"job-name": "k8sense-vuln-1"},
		},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "trivy", Image: "aquasec/trivy:latest"}}},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"}},
			}},
		},
	}
	cs := k8sfake.NewSimpleClientset(stuck)

	msg := podTrouble(context.Background(), cs, vulnNamespace, "k8sense-vuln-1")
	if msg == "" || !strings.Contains(msg, "aquasec/trivy:latest") || !strings.Contains(msg, "ImagePullBackOff") {
		t.Errorf("expected an ImagePullBackOff message naming the image, got %q", msg)
	}

	// A healthy/running pod must report no trouble.
	healthy := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: vulnNamespace, Name: "ok", Labels: map[string]string{"job-name": "k8sense-vuln-2"}},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "trivy", Image: "x"}}},
		Status:     corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}},
	}
	if m := podTrouble(context.Background(), k8sfake.NewSimpleClientset(healthy), vulnNamespace, "k8sense-vuln-2"); m != "" {
		t.Errorf("healthy pod should report no trouble, got %q", m)
	}
}
