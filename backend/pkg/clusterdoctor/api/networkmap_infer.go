package api

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Config-derived connection inference. NetworkPolicies show what's *allowed* and
// a mesh shows what's *observed*; this shows what apps are *configured* to talk
// to — by matching each workload's env/args/ConfigMap values against Service
// names (e.g. DATABASE_URL=postgres://orders-db:5432 → orders is wired to
// orders-db). It's inference from configuration, not observed traffic, so the
// UI draws these as dashed "wired to" edges, distinct from policy and mesh.
//
// Secret values are intentionally NOT read — connection strings in Secrets are
// sensitive; env values and ConfigMaps give enough signal for an architecture view.

// inferredEdge is a configured (not observed) connection, via a Service.
type inferredEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Via    string `json:"via"` // the Service name the source references
}

// makeWorkload builds a workload with its labels, DB classification, and the
// config text used for connection inference.
func makeWorkload(ns, name, kind string, tmpl corev1.PodTemplateSpec) workload {
	return workload{
		namespace:  ns,
		name:       name,
		kind:       kind,
		labels:     tmpl.Labels,
		dbEngine:   classifyDB(tmpl.Spec.Containers),
		configText: workloadConfigText(tmpl.Spec),
		configMaps: workloadConfigMapRefs(tmpl.Spec),
	}
}

// workloadConfigText gathers the searchable connection config: container args,
// commands, and env values (lower-cased).
func workloadConfigText(spec corev1.PodSpec) string {
	var parts []string

	all := append([]corev1.Container{}, spec.InitContainers...)
	all = append(all, spec.Containers...)

	for _, c := range all {
		parts = append(parts, c.Args...)
		parts = append(parts, c.Command...)

		for _, e := range c.Env {
			if e.Value != "" {
				parts = append(parts, e.Value)
			}
		}
	}

	return strings.ToLower(strings.Join(parts, " "))
}

// workloadConfigMapRefs lists ConfigMaps a workload pulls config from (envFrom,
// env valueFrom, and volume mounts) so their values can be searched too.
func workloadConfigMapRefs(spec corev1.PodSpec) []string {
	var names []string

	all := append([]corev1.Container{}, spec.InitContainers...)
	all = append(all, spec.Containers...)

	for _, c := range all {
		for _, ef := range c.EnvFrom {
			if ef.ConfigMapRef != nil {
				names = append(names, ef.ConfigMapRef.Name)
			}
		}

		for _, e := range c.Env {
			if e.ValueFrom != nil && e.ValueFrom.ConfigMapKeyRef != nil {
				names = append(names, e.ValueFrom.ConfigMapKeyRef.Name)
			}
		}
	}

	for _, v := range spec.Volumes {
		if v.ConfigMap != nil {
			names = append(names, v.ConfigMap.Name)
		}
	}

	return names
}

// inferredConnections draws an edge from each workload to the backend of every
// Service its configuration references.
func inferredConnections(ctx context.Context, clientset kubernetes.Interface, workloads []workload, ns string) []inferredEdge {
	services, err := clientset.CoreV1().Services(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return []inferredEdge{}
	}

	cmData := configMapText(ctx, clientset, ns)

	// Resolve each selector Service to its backend workload node IDs + ref tokens.
	type resolvedSvc struct {
		name     string
		tokens   []string
		backends []string
	}

	var svcs []resolvedSvc

	for _, svc := range services.Items {
		if len(svc.Spec.Selector) == 0 {
			continue // headless/external — no in-cluster backend to point at
		}

		var backends []string

		for _, wl := range workloads {
			if wl.namespace == svc.Namespace && selectorSubset(svc.Spec.Selector, wl.labels) {
				backends = append(backends, wl.id())
			}
		}

		if len(backends) == 0 {
			continue
		}

		svcs = append(svcs, resolvedSvc{
			name: svc.Name, tokens: serviceRefTokens(svc.Name, svc.Namespace), backends: backends,
		})
	}

	edges := []inferredEdge{}
	seen := map[string]bool{}

	for _, app := range workloads {
		text := app.configText
		for _, cm := range app.configMaps {
			text += " " + cmData[app.namespace+"/"+cm]
		}

		if strings.TrimSpace(text) == "" {
			continue
		}

		for _, s := range svcs {
			if !referencesService(text, s.name, s.tokens) {
				continue
			}

			for _, backend := range s.backends {
				key := app.id() + "->" + backend
				if backend == app.id() || seen[key] {
					continue
				}

				seen[key] = true
				edges = append(edges, inferredEdge{ID: "infer:" + key, Source: app.id(), Target: backend, Via: s.name})
			}
		}
	}

	return edges
}

// configMapText returns "namespace/name" -> lower-cased concatenated data.
func configMapText(ctx context.Context, clientset kubernetes.Interface, ns string) map[string]string {
	out := map[string]string{}

	cms, err := clientset.CoreV1().ConfigMaps(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return out
	}

	for _, cm := range cms.Items {
		var b strings.Builder

		for _, v := range cm.Data {
			b.WriteString(v)
			b.WriteByte(' ')
		}

		out[cm.Namespace+"/"+cm.Name] = strings.ToLower(b.String())
	}

	return out
}

// serviceRefTokens are the DNS-ish forms a Service is referenced by.
func serviceRefTokens(name, ns string) []string {
	n := strings.ToLower(name)

	return []string{
		n + "." + ns + ".svc.cluster.local",
		n + "." + ns + ".svc",
		n + "." + ns,
	}
}

// referencesService reports whether text refers to the Service: any FQDN form as
// a substring, or the bare name as a whole token (guarded against short names).
func referencesService(text, name string, tokens []string) bool {
	for _, tok := range tokens {
		if strings.Contains(text, tok) {
			return true
		}
	}

	return len(name) >= 3 && boundedContains(text, strings.ToLower(name))
}

// boundedContains matches word as a whole token, where a token boundary is any
// character that isn't part of a DNS label (a-z, 0-9, or '-').
func boundedContains(text, word string) bool {
	from := 0

	for {
		i := strings.Index(text[from:], word)
		if i < 0 {
			return false
		}

		pos := from + i

		var before, after byte = ' ', ' '
		if pos > 0 {
			before = text[pos-1]
		}

		if pos+len(word) < len(text) {
			after = text[pos+len(word)]
		}

		if !isLabelChar(before) && !isLabelChar(after) {
			return true
		}

		from = pos + 1
	}
}

func isLabelChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '-'
}

// selectorSubset reports whether every selector key=value is present in labels.
func selectorSubset(selector, labels map[string]string) bool {
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}

	return true
}
