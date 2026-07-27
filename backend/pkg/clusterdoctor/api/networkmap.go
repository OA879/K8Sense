package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

// This file builds the data behind the Network Map: a mesh-free view of what is
// allowed to talk to what, derived purely from the Kubernetes API (workloads +
// NetworkPolicies). It does NOT show live traffic — that needs a service mesh's
// sidecar telemetry. It answers "what can reach what, and what is exposed?".

// netNode is one workload in the map.
type netNode struct {
	ID        string `json:"id"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	// exposure summarises ingress reachability:
	//   open       — no ingress policy selects it (default-allow) OR a policy
	//                 explicitly allows ingress from anywhere
	//   restricted — an ingress policy limits who may connect
	//   isolated   — an ingress policy selects it but allows no ingress at all
	Exposure string `json:"exposure"`
	// Protected is true when at least one ingress NetworkPolicy selects it.
	Protected bool `json:"protected"`
}

// netEdge is one allowed ingress connection (source workload -> target workload).
type netEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Ports  string `json:"ports,omitempty"`
}

// netMap is the whole graph plus the namespaces present (for the UI filter).
// Edges are policy-allowed connections; Traffic (present only when a mesh is
// detected) are live observed flows.
type netMap struct {
	Nodes      []netNode     `json:"nodes"`
	Edges      []netEdge     `json:"edges"`
	Traffic    []trafficEdge `json:"traffic"`
	Mesh       meshInfo      `json:"mesh"`
	Namespaces []string      `json:"namespaces"`
}

// workload is the internal unit the analyzer reasons about — a controller whose
// pod template labels are what NetworkPolicies select on.
type workload struct {
	namespace string
	name      string
	kind      string
	labels    map[string]string
}

func (w workload) id() string { return w.kind + "/" + w.namespace + "/" + w.name }

// NetworkMap handles GET /cluster-doctor/network-map?cluster=&namespace= .
func (s *Server) NetworkMap(w http.ResponseWriter, r *http.Request) {
	cluster := r.URL.Query().Get("cluster")

	clientset, err := s.getClient(r, cluster)
	if err != nil {
		http.Error(w, `{"error": "cluster not found"}`, http.StatusNotFound)
		return
	}

	m, err := buildNetworkMap(r.Context(), clientset, r.URL.Query().Get("namespace"))
	if err != nil {
		http.Error(w, `{"error": "could not build network map"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(m)
}

// buildNetworkMap gathers workloads and NetworkPolicies and derives the
// exposure of each workload and the allowed ingress edges between them. If
// namespace is empty it covers the whole cluster.
func buildNetworkMap(ctx context.Context, clientset kubernetes.Interface, namespace string) (netMap, error) {
	ns := namespace
	if ns == "" {
		ns = metav1.NamespaceAll
	}

	workloads, err := listWorkloads(ctx, clientset, ns)
	if err != nil {
		return netMap{}, err
	}

	policies, err := clientset.NetworkingV1().NetworkPolicies(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return netMap{}, err
	}

	nodes := make([]netNode, 0, len(workloads))
	edges := make([]netEdge, 0)
	nsSet := map[string]bool{}

	for _, wl := range workloads {
		nsSet[wl.namespace] = true

		node := netNode{
			ID: wl.id(), Namespace: wl.namespace, Name: wl.name,
			Kind: wl.kind, Exposure: "open",
		}

		selecting := selectingIngressPolicies(wl, policies.Items)
		if len(selecting) > 0 {
			node.Protected = true
			node.Exposure = ingressExposure(selecting)

			edges = append(edges, ingressEdges(wl, selecting, workloads)...)
		}

		nodes = append(nodes, node)
	}

	// Live-traffic overlay — present only when a mesh's Prometheus is reachable.
	mesh, traffic, extraNodes := meshTraffic(ctx, clientset, namespace, workloads)
	if traffic == nil {
		traffic = []trafficEdge{} // marshal as [] not null, so the UI can always iterate
	}

	nodes = append(nodes, extraNodes...)

	for _, n := range extraNodes {
		nsSet[n.Namespace] = true
	}

	namespaces := make([]string, 0, len(nsSet))
	for n := range nsSet {
		namespaces = append(namespaces, n)
	}

	sort.Strings(namespaces)

	return netMap{
		Nodes: nodes, Edges: edges, Traffic: traffic,
		Mesh: mesh, Namespaces: namespaces,
	}, nil
}

// listWorkloads returns the Deployments, StatefulSets and DaemonSets in ns as
// analyzer workloads (keyed on their pod template labels).
func listWorkloads(ctx context.Context, clientset kubernetes.Interface, ns string) ([]workload, error) {
	var out []workload

	deploys, err := clientset.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, d := range deploys.Items {
		out = append(out, workload{d.Namespace, d.Name, "Deployment", d.Spec.Template.Labels})
	}

	stateful, err := clientset.AppsV1().StatefulSets(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, s := range stateful.Items {
		out = append(out, workload{s.Namespace, s.Name, "StatefulSet", s.Spec.Template.Labels})
	}

	daemon, err := clientset.AppsV1().DaemonSets(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, d := range daemon.Items {
		out = append(out, workload{d.Namespace, d.Name, "DaemonSet", d.Spec.Template.Labels})
	}

	return out, nil
}

// policyAppliesToIngress reports whether a policy governs ingress. Per the spec,
// Ingress is in effect unless PolicyTypes is explicitly set to Egress-only.
func policyAppliesToIngress(p networkingv1.NetworkPolicy) bool {
	if len(p.Spec.PolicyTypes) == 0 {
		return true
	}

	for _, t := range p.Spec.PolicyTypes {
		if t == networkingv1.PolicyTypeIngress {
			return true
		}
	}

	return false
}

// selectingIngressPolicies returns the ingress policies in the workload's
// namespace whose podSelector matches it.
func selectingIngressPolicies(wl workload, policies []networkingv1.NetworkPolicy) []networkingv1.NetworkPolicy {
	var out []networkingv1.NetworkPolicy

	for _, p := range policies {
		if p.Namespace != wl.namespace || !policyAppliesToIngress(p) {
			continue
		}

		if selectorMatches(&p.Spec.PodSelector, wl.labels) {
			out = append(out, p)
		}
	}

	return out
}

// ingressExposure classifies a selected workload's reachability.
func ingressExposure(selecting []networkingv1.NetworkPolicy) string {
	anyRule := false

	for _, p := range selecting {
		for _, rule := range p.Spec.Ingress {
			anyRule = true
			if len(rule.From) == 0 { // an ingress rule with no "from" allows all sources
				return "open"
			}
		}
	}

	if !anyRule {
		return "isolated" // selected by a policy but no ingress rule -> deny all
	}

	return "restricted"
}

// ingressEdges resolves the concrete source workloads allowed to reach wl and
// returns one edge per source.
func ingressEdges(wl workload, selecting []networkingv1.NetworkPolicy, all []workload) []netEdge {
	var edges []netEdge

	seen := map[string]bool{}

	for _, p := range selecting {
		for _, rule := range p.Spec.Ingress {
			ports := summarisePorts(rule.Ports)

			for _, peer := range rule.From {
				for _, src := range matchingSources(peer, wl.namespace, all) {
					if src.id() == wl.id() {
						continue // ignore self-edges
					}

					key := src.id() + "->" + wl.id()
					if seen[key] {
						continue
					}

					seen[key] = true

					edges = append(edges, netEdge{
						ID: key, Source: src.id(), Target: wl.id(), Ports: ports,
					})
				}
			}
		}
	}

	return edges
}

// matchingSources resolves a NetworkPolicyPeer to the workloads it selects.
// podSelector without namespaceSelector is scoped to the target's namespace;
// with a namespaceSelector it spans every namespace (label-based namespace
// matching is left to a future pass). ipBlock peers describe external CIDRs and
// have no workload node, so they are skipped.
func matchingSources(peer networkingv1.NetworkPolicyPeer, targetNS string, all []workload) []workload {
	if peer.IPBlock != nil {
		return nil
	}

	var out []workload

	for _, wl := range all {
		if peer.NamespaceSelector == nil && wl.namespace != targetNS {
			continue
		}

		if peer.PodSelector != nil && !selectorMatches(peer.PodSelector, wl.labels) {
			continue
		}

		out = append(out, wl)
	}

	return out
}

// selectorMatches reports whether pod labels satisfy a LabelSelector. A nil or
// empty selector matches everything (the NetworkPolicy convention).
func selectorMatches(selector *metav1.LabelSelector, podLabels map[string]string) bool {
	if selector == nil {
		return true
	}

	sel, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return false
	}

	return sel.Matches(labels.Set(podLabels))
}

// summarisePorts renders a compact "tcp/80, tcp/443" label for an edge, or ""
// when the rule allows all ports.
func summarisePorts(ports []networkingv1.NetworkPolicyPort) string {
	if len(ports) == 0 {
		return ""
	}

	parts := make([]string, 0, len(ports))

	for _, p := range ports {
		proto := "tcp"
		if p.Protocol != nil {
			proto = strings.ToLower(string(*p.Protocol))
		}

		if p.Port != nil {
			parts = append(parts, fmt.Sprintf("%s/%s", proto, p.Port.String()))
		} else {
			parts = append(parts, proto)
		}
	}

	return strings.Join(parts, ", ")
}
