package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"k8s.io/client-go/kubernetes"
)

// This file adds the live-traffic overlay to the Network Map. Unlike the
// policy view (which is pure Kubernetes API), live flow needs a service mesh's
// telemetry: it queries a Prometheus that scrapes Istio's istio_requests_total.
// If no such Prometheus is reachable, the overlay is simply absent and the map
// falls back to the policy/topology view — nothing here ever fails the request.

// meshInfo tells the UI whether a live overlay is available and where it came from.
type meshInfo struct {
	Enabled bool   `json:"enabled"`
	Source  string `json:"source,omitempty"`
}

// trafficEdge is one observed source->target flow with its request rate.
type trafficEdge struct {
	ID     string  `json:"id"`
	Source string  `json:"source"`
	Target string  `json:"target"`
	Rps    float64 `json:"rps"`
}

// promEndpoint is a well-known in-cluster Prometheus we try, reached through the
// API server's service proxy so it works without extra networking.
type promEndpoint struct {
	namespace string
	service   string
	port      int
}

//nolint:gochecknoglobals // static list of conventional Prometheus locations
var promCandidates = []promEndpoint{
	{"istio-system", "prometheus", 9090},
	{"monitoring", "prometheus-k8s", 9090},
	{"monitoring", "prometheus-server", 80},
	{"monitoring", "prometheus-operated", 9090},
	{"prometheus", "prometheus-server", 80},
	{"istio-system", "prometheus-server", 9090},
}

const istioRateQuery = `sum by (source_workload,source_workload_namespace,destination_workload,destination_workload_namespace) (rate(istio_requests_total{reporter="destination"}[5m]))`

// meshTraffic finds a Prometheus with Istio metrics and returns the live flows,
// mapped onto the workload node IDs. extraNodes carries any traffic endpoints
// (e.g. external or mesh-only workloads) that aren't in the policy node set, so
// the graph stays consistent. A namespace filter, when set, keeps only flows
// that touch it.
func meshTraffic(
	ctx context.Context, clientset kubernetes.Interface, namespace string, workloads []workload,
) (meshInfo, []trafficEdge, []netNode) {
	ep, ok := findIstioPrometheus(ctx, clientset)
	if !ok {
		return meshInfo{Enabled: false}, nil, nil
	}

	info := meshInfo{
		Enabled: true,
		Source:  fmt.Sprintf("Istio via %s.%s", ep.service, ep.namespace),
	}

	vec, err := promQuery(ctx, clientset, ep, istioRateQuery)
	if err != nil {
		return info, nil, nil // mesh present but query failed — still report enabled
	}

	byKey := map[string]string{} // "ns/name" -> node id
	for _, wl := range workloads {
		byKey[wl.namespace+"/"+wl.name] = wl.id()
	}

	var (
		edges []trafficEdge
		extra []netNode
	)

	seenExtra := map[string]bool{}

	resolve := func(ns, name string) string {
		if id, found := byKey[ns+"/"+name]; found {
			return id
		}

		id := "External/" + ns + "/" + name
		if !seenExtra[id] {
			seenExtra[id] = true

			extra = append(extra, netNode{
				ID: id, Namespace: ns, Name: name, Kind: "External", Exposure: "external",
			})
		}

		return id
	}

	for _, sample := range vec {
		sns := sample.metric["source_workload_namespace"]
		sn := sample.metric["source_workload"]
		dns := sample.metric["destination_workload_namespace"]
		dn := sample.metric["destination_workload"]

		if sn == "" || dn == "" || sn == "unknown" || dn == "unknown" {
			continue
		}

		if namespace != "" && sns != namespace && dns != namespace {
			continue
		}

		src := resolve(sns, sn)
		dst := resolve(dns, dn)

		edges = append(edges, trafficEdge{
			ID: "live:" + src + "->" + dst, Source: src, Target: dst, Rps: sample.value,
		})
	}

	return info, edges, extra
}

// findIstioPrometheus returns the first candidate Prometheus that both responds
// and actually has Istio request metrics.
func findIstioPrometheus(ctx context.Context, clientset kubernetes.Interface) (promEndpoint, bool) {
	for _, ep := range promCandidates {
		vec, err := promQuery(ctx, clientset, ep, `count(istio_requests_total)`)
		if err != nil {
			continue
		}

		if len(vec) > 0 && vec[0].value > 0 {
			return ep, true
		}
	}

	return promEndpoint{}, false
}

// promSample is one entry of a Prometheus instant-vector result.
type promSample struct {
	metric map[string]string
	value  float64
}

// promQuery runs a PromQL instant query against ep through the API server's
// service proxy and returns the result vector.
func promQuery(
	ctx context.Context, clientset kubernetes.Interface, ep promEndpoint, query string,
) (samples []promSample, err error) {
	// The overlay is best-effort: never let a REST-client quirk (or a fake
	// clientset in tests, which has no working service proxy) crash the map.
	defer func() {
		if r := recover(); r != nil {
			samples = nil
			err = fmt.Errorf("prometheus query panicked: %v", r)
		}
	}()

	raw, err := clientset.CoreV1().RESTClient().Get().
		AbsPath(fmt.Sprintf("/api/v1/namespaces/%s/services/%s:%d/proxy/api/v1/query",
			ep.namespace, ep.service, ep.port)).
		Param("query", query).
		DoRaw(ctx)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Status string `json:"status"`
		Data   struct {
			Result []struct {
				Metric map[string]string `json:"metric"`
				Value  []any             `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}

	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}

	if resp.Status != "success" {
		return nil, fmt.Errorf("prometheus query status %q", resp.Status)
	}

	out := make([]promSample, 0, len(resp.Data.Result))

	for _, r := range resp.Data.Result {
		// value is [<unix ts>, "<number as string>"].
		if len(r.Value) != 2 {
			continue
		}

		s, ok := r.Value[1].(string)
		if !ok {
			continue
		}

		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			continue
		}

		out = append(out, promSample{metric: r.Metric, value: f})
	}

	return out, nil
}
