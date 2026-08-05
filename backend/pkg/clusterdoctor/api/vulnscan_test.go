package api

import (
	"context"
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
