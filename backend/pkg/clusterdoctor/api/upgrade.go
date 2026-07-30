package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Upgrade & deprecation readiness. It answers "will this Kubernetes upgrade
// break prod?" by finding resources that were applied under an apiVersion that
// is deprecated or removed in a target version.
//
// The API server auto-converts objects to their newest served version, so
// listing live objects and reading their apiVersion misses everything. Like
// Pluto/kubent, we read the *applied* version from two honest sources: the
// last-applied-configuration annotation, and Helm release manifests (stored as
// helm.sh/release.v1 Secrets). Objects applied server-side with neither can be
// missed — the report says so rather than pretending full coverage.

// deprecation is one API removal fact.
type deprecation struct {
	APIVersion   string // group/version, e.g. "policy/v1beta1"
	Kind         string
	DeprecatedIn string // "1.21", or "" if unknown
	RemovedIn    string // "1.25", or "" if not yet scheduled
	Replacement  string
}

//nolint:gochecknoglobals // curated static table of well-known API removals
var deprecations = []deprecation{
	// Removed in 1.16 (very old, still occasionally seen in ancient manifests).
	{"extensions/v1beta1", "Deployment", "1.9", "1.16", "apps/v1"},
	{"extensions/v1beta1", "DaemonSet", "1.9", "1.16", "apps/v1"},
	{"extensions/v1beta1", "ReplicaSet", "1.9", "1.16", "apps/v1"},
	{"extensions/v1beta1", "NetworkPolicy", "1.9", "1.16", "networking.k8s.io/v1"},
	{"apps/v1beta1", "Deployment", "1.9", "1.16", "apps/v1"},
	{"apps/v1beta2", "Deployment", "1.9", "1.16", "apps/v1"},
	{"apps/v1beta1", "StatefulSet", "1.9", "1.16", "apps/v1"},
	{"apps/v1beta2", "StatefulSet", "1.9", "1.16", "apps/v1"},
	// Removed in 1.22.
	{"extensions/v1beta1", "Ingress", "1.14", "1.22", "networking.k8s.io/v1"},
	{"networking.k8s.io/v1beta1", "Ingress", "1.19", "1.22", "networking.k8s.io/v1"},
	{"networking.k8s.io/v1beta1", "IngressClass", "1.19", "1.22", "networking.k8s.io/v1"},
	{"apiextensions.k8s.io/v1beta1", "CustomResourceDefinition", "1.16", "1.22", "apiextensions.k8s.io/v1"},
	{"admissionregistration.k8s.io/v1beta1", "MutatingWebhookConfiguration", "1.16", "1.22", "admissionregistration.k8s.io/v1"},
	{"admissionregistration.k8s.io/v1beta1", "ValidatingWebhookConfiguration", "1.16", "1.22", "admissionregistration.k8s.io/v1"},
	{"rbac.authorization.k8s.io/v1beta1", "Role", "1.17", "1.22", "rbac.authorization.k8s.io/v1"},
	{"rbac.authorization.k8s.io/v1beta1", "RoleBinding", "1.17", "1.22", "rbac.authorization.k8s.io/v1"},
	{"rbac.authorization.k8s.io/v1beta1", "ClusterRole", "1.17", "1.22", "rbac.authorization.k8s.io/v1"},
	{"rbac.authorization.k8s.io/v1beta1", "ClusterRoleBinding", "1.17", "1.22", "rbac.authorization.k8s.io/v1"},
	{"certificates.k8s.io/v1beta1", "CertificateSigningRequest", "1.19", "1.22", "certificates.k8s.io/v1"},
	{"storage.k8s.io/v1beta1", "CSIDriver", "1.19", "1.22", "storage.k8s.io/v1"},
	{"storage.k8s.io/v1beta1", "CSINode", "1.17", "1.22", "storage.k8s.io/v1"},
	{"storage.k8s.io/v1beta1", "StorageClass", "1.19", "1.22", "storage.k8s.io/v1"},
	// Removed in 1.25.
	{"policy/v1beta1", "PodDisruptionBudget", "1.21", "1.25", "policy/v1"},
	{"policy/v1beta1", "PodSecurityPolicy", "1.21", "1.25", "Pod Security Admission (PSP has no replacement API)"},
	{"batch/v1beta1", "CronJob", "1.21", "1.25", "batch/v1"},
	{"discovery.k8s.io/v1beta1", "EndpointSlice", "1.21", "1.25", "discovery.k8s.io/v1"},
	{"node.k8s.io/v1beta1", "RuntimeClass", "1.20", "1.25", "node.k8s.io/v1"},
	{"autoscaling/v2beta1", "HorizontalPodAutoscaler", "1.22", "1.25", "autoscaling/v2"},
	// Removed in 1.26.
	{"autoscaling/v2beta2", "HorizontalPodAutoscaler", "1.23", "1.26", "autoscaling/v2"},
	{"flowcontrol.apiserver.k8s.io/v1beta1", "FlowSchema", "1.23", "1.26", "flowcontrol.apiserver.k8s.io/v1"},
	// Removed in 1.29 / 1.32.
	{"flowcontrol.apiserver.k8s.io/v1beta2", "FlowSchema", "1.26", "1.29", "flowcontrol.apiserver.k8s.io/v1"},
	{"flowcontrol.apiserver.k8s.io/v1beta3", "FlowSchema", "1.29", "1.32", "flowcontrol.apiserver.k8s.io/v1"},
}

type upgradeItem struct {
	Namespace    string `json:"namespace,omitempty"`
	Kind         string `json:"kind"`
	Name         string `json:"name"`
	APIVersion   string `json:"apiVersion"`
	DeprecatedIn string `json:"deprecatedIn,omitempty"`
	RemovedIn    string `json:"removedIn,omitempty"`
	Replacement  string `json:"replacement"`
	Severity     string `json:"severity"` // blocker | warning
	Source       string `json:"source"`   // applied | helm
}

type upgradeReport struct {
	CurrentVersion string        `json:"currentVersion"`
	TargetMinor    int           `json:"targetMinor"`
	Blockers       int           `json:"blockers"`
	Warnings       int           `json:"warnings"`
	Items          []upgradeItem `json:"items"`
}

// parseMinor extracts the minor number from "1.32", "v1.30.2", "30+", etc.
func parseMinor(v string) int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")

	pick := parts[0]
	if len(parts) > 1 {
		pick = parts[1] // "1.32" -> "32"
	}

	digits := strings.Builder{}

	for _, r := range pick {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		} else {
			break
		}
	}

	n, _ := strconv.Atoi(digits.String())

	return n
}

// lookupDeprecation finds the removal fact for an applied apiVersion + kind.
func lookupDeprecation(apiVersion, kind string) *deprecation {
	for i := range deprecations {
		if deprecations[i].APIVersion == apiVersion && deprecations[i].Kind == kind {
			return &deprecations[i]
		}
	}

	return nil
}

// severityForTarget returns "blocker" if the API is removed at or before the
// target minor, "warning" if only deprecated by then, else "".
func severityForTarget(d *deprecation, targetMinor int) string {
	if d.RemovedIn != "" && targetMinor >= parseMinor(d.RemovedIn) {
		return "blocker"
	}

	if d.DeprecatedIn != "" && targetMinor >= parseMinor(d.DeprecatedIn) {
		return "warning"
	}

	return ""
}

// itemFor builds an upgradeItem if the applied apiVersion+kind is a problem at
// the target minor, else nil.
func itemFor(apiVersion, kind, namespace, name, source string, targetMinor int) *upgradeItem {
	d := lookupDeprecation(apiVersion, kind)
	if d == nil {
		return nil
	}

	sev := severityForTarget(d, targetMinor)
	if sev == "" {
		return nil
	}

	return &upgradeItem{
		Namespace: namespace, Kind: kind, Name: name, APIVersion: apiVersion,
		DeprecatedIn: d.DeprecatedIn, RemovedIn: d.RemovedIn, Replacement: d.Replacement,
		Severity: sev, Source: source,
	}
}

// appliedAPIVersion pulls apiVersion+kind out of a last-applied-configuration
// annotation value.
func appliedAPIVersion(annotation string) (apiVersion, kind string) {
	var obj struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
	}

	_ = json.Unmarshal([]byte(annotation), &obj)

	return obj.APIVersion, obj.Kind
}

// manifestAPIVersions extracts (apiVersion, kind) pairs from a rendered
// multi-document YAML manifest (as found in a Helm release).
func manifestAPIVersions(manifest string) [][2]string {
	var out [][2]string

	var apiVersion string

	for _, line := range strings.Split(manifest, "\n") {
		trimmed := strings.TrimSpace(line)

		switch {
		case trimmed == "---":
			apiVersion = ""
		case strings.HasPrefix(trimmed, "apiVersion:"):
			apiVersion = strings.TrimSpace(strings.TrimPrefix(trimmed, "apiVersion:"))
		case strings.HasPrefix(trimmed, "kind:") && apiVersion != "":
			kind := strings.TrimSpace(strings.TrimPrefix(trimmed, "kind:"))
			out = append(out, [2]string{apiVersion, kind})
			apiVersion = "" // one pair per document
		}
	}

	return out
}

// decodeHelmManifest decodes a Helm v3 release Secret's data into its rendered
// manifest. The stored value is base64(gzip(json{manifest})).
func decodeHelmManifest(releaseData []byte) (string, error) {
	gzipped, err := base64.StdEncoding.DecodeString(string(releaseData))
	if err != nil {
		return "", err
	}

	gz, err := gzip.NewReader(bytes.NewReader(gzipped))
	if err != nil {
		return "", err
	}

	raw, err := io.ReadAll(gz)
	if err != nil {
		return "", err
	}

	var rel struct {
		Manifest string `json:"manifest"`
	}

	if err := json.Unmarshal(raw, &rel); err != nil {
		return "", err
	}

	return rel.Manifest, nil
}

// UpgradeReadiness handles GET /cluster-doctor/upgrade?cluster=&target= .
func (s *Server) UpgradeReadiness(w http.ResponseWriter, r *http.Request) {
	cluster := r.URL.Query().Get("cluster")

	clientset, err := s.getClient(r, cluster)
	if err != nil {
		http.Error(w, `{"error": "cluster not found"}`, http.StatusNotFound)
		return
	}

	report := buildUpgradeReport(r.Context(), clientset, r.URL.Query().Get("target"))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(report)
}

// buildUpgradeReport gathers applied-annotation and Helm sources and classifies
// against the target minor (defaulting to current + 2 if not given).
func buildUpgradeReport(ctx context.Context, clientset kubernetes.Interface, target string) upgradeReport {
	current := ""
	currentMinor := 0

	if v, err := clientset.Discovery().ServerVersion(); err == nil {
		current = v.GitVersion
		currentMinor = parseMinor(v.Major + "." + v.Minor)
	}

	targetMinor := currentMinor + 2
	if target != "" {
		targetMinor = parseMinor(target)
	}

	report := upgradeReport{CurrentVersion: current, TargetMinor: targetMinor, Items: []upgradeItem{}}

	report.Items = append(report.Items, scanHelmReleases(ctx, clientset, targetMinor)...)
	report.Items = append(report.Items, scanAppliedAnnotations(ctx, clientset, targetMinor)...)
	report.Items = dedupeUpgradeItems(report.Items)

	for _, it := range report.Items {
		if it.Severity == "blocker" {
			report.Blockers++
		} else {
			report.Warnings++
		}
	}

	sort.Slice(report.Items, func(i, j int) bool {
		if report.Items[i].Severity != report.Items[j].Severity {
			return report.Items[i].Severity == "blocker" // blockers first
		}

		return report.Items[i].Kind < report.Items[j].Kind
	})

	return report
}

func dedupeUpgradeItems(items []upgradeItem) []upgradeItem {
	seen := map[string]bool{}
	out := items[:0]

	for _, it := range items {
		key := it.Namespace + "/" + it.Kind + "/" + it.Name + "/" + it.APIVersion
		if seen[key] {
			continue
		}

		seen[key] = true

		out = append(out, it)
	}

	return out
}

// scanHelmReleases decodes helm.sh/release.v1 Secrets and checks each rendered
// resource's apiVersion.
func scanHelmReleases(ctx context.Context, clientset kubernetes.Interface, targetMinor int) []upgradeItem {
	secrets, err := clientset.CoreV1().Secrets(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}

	var items []upgradeItem

	for _, sec := range secrets.Items {
		if sec.Type != "helm.sh/release.v1" {
			continue
		}

		manifest, err := decodeHelmManifest(sec.Data["release"])
		if err != nil {
			continue
		}

		for _, pair := range manifestAPIVersions(manifest) {
			if it := itemFor(pair[0], pair[1], sec.Namespace, "(helm: "+sec.Labels["name"]+")", "helm", targetMinor); it != nil {
				items = append(items, *it)
			}
		}
	}

	return items
}

// scanAppliedAnnotations reads the last-applied-configuration annotation on the
// high-value kinds that are the classic upgrade breakers.
func scanAppliedAnnotations(ctx context.Context, clientset kubernetes.Interface, targetMinor int) []upgradeItem {
	const annKey = "kubectl.kubernetes.io/last-applied-configuration"

	var items []upgradeItem

	add := func(ns, name string, ann map[string]string) {
		a, ok := ann[annKey]
		if !ok {
			return
		}

		apiVersion, kind := appliedAPIVersion(a)
		if it := itemFor(apiVersion, kind, ns, name, "applied", targetMinor); it != nil {
			items = append(items, *it)
		}
	}

	if l, err := clientset.NetworkingV1().Ingresses(metav1.NamespaceAll).List(ctx, metav1.ListOptions{}); err == nil {
		for _, o := range l.Items {
			add(o.Namespace, o.Name, o.Annotations)
		}
	}

	if l, err := clientset.PolicyV1().PodDisruptionBudgets(metav1.NamespaceAll).List(ctx, metav1.ListOptions{}); err == nil {
		for _, o := range l.Items {
			add(o.Namespace, o.Name, o.Annotations)
		}
	}

	if l, err := clientset.BatchV1().CronJobs(metav1.NamespaceAll).List(ctx, metav1.ListOptions{}); err == nil {
		for _, o := range l.Items {
			add(o.Namespace, o.Name, o.Annotations)
		}
	}

	if l, err := clientset.AutoscalingV2().HorizontalPodAutoscalers(metav1.NamespaceAll).List(ctx, metav1.ListOptions{}); err == nil {
		for _, o := range l.Items {
			add(o.Namespace, o.Name, o.Annotations)
		}
	}

	return items
}
