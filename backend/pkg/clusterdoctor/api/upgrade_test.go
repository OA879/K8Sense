package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestParseMinor(t *testing.T) {
	for in, want := range map[string]int{"1.32": 32, "v1.30.2": 30, "30+": 30, "1.25": 25, "": 0} {
		if got := parseMinor(in); got != want {
			t.Errorf("parseMinor(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestSeverityForTarget(t *testing.T) {
	pdb := lookupDeprecation("policy/v1beta1", "PodDisruptionBudget") // deprecated 1.21, removed 1.25
	if pdb == nil {
		t.Fatal("PDB deprecation not in table")
	}

	if got := severityForTarget(pdb, 25); got != "blocker" {
		t.Errorf("target 1.25 = %q, want blocker (removed)", got)
	}
	if got := severityForTarget(pdb, 23); got != "warning" {
		t.Errorf("target 1.23 = %q, want warning (deprecated, not yet removed)", got)
	}
	if got := severityForTarget(pdb, 20); got != "" {
		t.Errorf("target 1.20 = %q, want '' (not deprecated yet)", got)
	}
}

func TestManifestAPIVersions(t *testing.T) {
	manifest := "apiVersion: batch/v1beta1\nkind: CronJob\nmetadata:\n  name: x\n---\napiVersion: v1\nkind: Service\n"
	pairs := manifestAPIVersions(manifest)
	if len(pairs) != 2 || pairs[0] != [2]string{"batch/v1beta1", "CronJob"} || pairs[1] != [2]string{"v1", "Service"} {
		t.Fatalf("got %v", pairs)
	}
}

func encodeHelmRelease(manifest string) []byte {
	rel, _ := json.Marshal(struct {
		Manifest string `json:"manifest"`
	}{manifest})

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, _ = gz.Write(rel)
	_ = gz.Close()

	return []byte(base64.StdEncoding.EncodeToString(buf.Bytes()))
}

func TestDecodeHelmManifest(t *testing.T) {
	got, err := decodeHelmManifest(encodeHelmRelease("apiVersion: policy/v1beta1\nkind: PodDisruptionBudget\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "apiVersion: policy/v1beta1\nkind: PodDisruptionBudget\n" {
		t.Fatalf("got %q", got)
	}
}

func TestBuildUpgradeReport_HelmAndApplied(t *testing.T) {
	helm := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "sh.helm.release.v1.batch.v1", Labels: map[string]string{"name": "batch"}},
		Type:       "helm.sh/release.v1",
		Data:       map[string][]byte{"release": encodeHelmRelease("apiVersion: batch/v1beta1\nkind: CronJob\n")},
	}
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "web", Name: "site",
			Annotations: map[string]string{
				"kubectl.kubernetes.io/last-applied-configuration": `{"apiVersion":"extensions/v1beta1","kind":"Ingress"}`,
			},
		},
	}

	report := buildUpgradeReport(context.Background(), k8sfake.NewSimpleClientset(helm, ing), "1.25")

	if report.TargetMinor != 25 {
		t.Fatalf("target minor = %d, want 25", report.TargetMinor)
	}
	// CronJob (removed 1.25) + Ingress extensions/v1beta1 (removed 1.22) -> 2 blockers.
	if report.Blockers != 2 || len(report.Items) != 2 {
		t.Fatalf("blockers=%d items=%d, want 2/2: %+v", report.Blockers, len(report.Items), report.Items)
	}
	for _, it := range report.Items {
		if it.Severity != "blocker" {
			t.Errorf("item %s should be a blocker for 1.25: %+v", it.Kind, it)
		}
	}
}
