package checks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/OA879/K8Sense/backend/pkg/clusterdoctor"
)

// This file holds the "third wave" of checks — the ones that were deferred
// because they need data the plain object-listing API can't give: the API
// server's own Prometheus /metrics, its /readyz self-report, and the
// VerticalPodAutoscaler CRD. They all still go through the standard clientset
// via Discovery().RESTClient().AbsPath(...), so no dynamic client, metrics
// server, or extra config plumbing is required.
//
// Every check here degrades gracefully: if the endpoint is forbidden (managed
// control planes usually don't expose /metrics to a normal ServiceAccount) it
// returns (nil, err) so the scanner counts it as *skipped*, and if the endpoint
// is reachable but the specific metric / CRD isn't present it returns
// (nil, nil) — "not applicable here", not a finding.

const (
	// apiServerErrorRateThreshold is the fraction of 5xx responses (out of all
	// apiserver responses since it started) above which CP-007 fires.
	apiServerErrorRateThreshold = 0.05
	// apiServerErrorRateMinSamples avoids alerting on a freshly-started API
	// server that has only served a handful of requests.
	apiServerErrorRateMinSamples = 200.0

	// etcdSlowLatencySeconds is the request-duration bucket boundary above which
	// an etcd request counts as slow (apiserver-observed round trip to etcd).
	etcdSlowLatencySeconds = 1.0
	// etcdSlowFraction is how much of etcd traffic may exceed that latency before
	// CP-006 fires, and etcdSlowMinSamples floors the sample size.
	etcdSlowFraction  = 0.01
	etcdSlowMinSamples = 100.0

	// clientCertWarnSeconds mirrors certExpiryWarnWindow (30 days): CERT-005
	// fires when the API server sees client certificates this close to expiry.
	clientCertWarnSeconds = 30 * 24 * 60 * 60

	// vpaDivergenceRatio is how far a VPA recommendation may drift from the
	// workload's configured request (in either direction) before RES-006 flags
	// it as significantly mis-sized.
	vpaDivergenceRatio = 1.5
)

func init() {
	clusterdoctor.RegisterCheck("check_apiserver_readyz_degraded", checkAPIServerReadyzDegraded)
	clusterdoctor.RegisterCheck("check_apiserver_error_rate", checkAPIServerErrorRate)
	clusterdoctor.RegisterCheck("check_etcd_slow", checkEtcdSlow)
	clusterdoctor.RegisterCheck("check_client_cert_expiring", checkClientCertExpiring)
	clusterdoctor.RegisterCheck("check_vpa_divergence", checkVPADivergence)
}

// fetchAPIServerMetrics scrapes and parses the API server's /metrics endpoint.
// A non-nil error means the check couldn't run (forbidden / unreachable) and
// should be reported as skipped, not as "healthy".
func fetchAPIServerMetrics(ctx context.Context, clientset kubernetes.Interface) (map[string]*dto.MetricFamily, error) {
	raw, err := clientset.Discovery().RESTClient().Get().AbsPath("/metrics").DoRaw(ctx)
	if err != nil {
		return nil, err
	}

	// NewTextParser (not a bare TextParser{}) is required: the zero-value parser
	// has an unset name-validation scheme and panics on the first metric line.
	parser := expfmt.NewTextParser(model.UTF8Validation)

	return parser.TextToMetricFamilies(bytes.NewReader(raw))
}

// CP-001 — the API server's /readyz reports one or more failing subsystems.
// A scan can always *reach* the API server (it's talking to it), so the
// observable form of "API server unhealthy" is readyz listing failed checks
// (etcd, informer-sync, poststarthooks, ...). Each failing subcheck is a finding.
func checkAPIServerReadyzDegraded(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	// /readyz returns 500 (and thus DoRaw returns an error) precisely when it is
	// degraded — but the body still carries the [+]/[-] report, so parse it
	// whenever we got a body back.
	raw, err := clientset.Discovery().RESTClient().Get().
		AbsPath("/readyz").Param("verbose", "true").DoRaw(ctx)

	if len(raw) == 0 {
		// No body at all → we genuinely couldn't assess it (RBAC, network).
		return nil, err
	}

	return parseReadyzReport(raw), nil
}

// parseReadyzReport turns a verbose /readyz body into one finding per failing
// subcheck (lines prefixed "[-]"). Healthy checks ("[+]") produce nothing.
func parseReadyzReport(raw []byte) []clusterdoctor.RawFinding {
	var findings []clusterdoctor.RawFinding

	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "[-]") {
			continue
		}

		// Lines look like "[-]etcd failed: reason" or "[-]informer-sync failed".
		// The subcheck name is everything before " failed".
		name := strings.TrimSpace(strings.TrimPrefix(line, "[-]"))
		name = strings.TrimSpace(strings.SplitN(name, " failed", 2)[0])

		findings = append(findings, clusterdoctor.RawFinding{
			Namespace:    "kube-system",
			ResourceKind: "APIServer",
			ResourceName: "kube-apiserver",
			RawObject:    fmt.Sprintf(`{"failingCheck": %q}`, name),
		})
	}

	return findings
}

// CP-007 — a meaningful share of API server responses are 5xx. Counted from
// apiserver_request_total, which is labelled by response code.
func checkAPIServerErrorRate(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	families, err := fetchAPIServerMetrics(ctx, clientset)
	if err != nil {
		return nil, err
	}

	mf, ok := families["apiserver_request_total"]
	if !ok {
		return nil, nil
	}

	rate, serverErrors, total := serverErrorRate(mf)
	if total < apiServerErrorRateMinSamples || rate < apiServerErrorRateThreshold {
		return nil, nil
	}

	return []clusterdoctor.RawFinding{{
		Namespace:    "kube-system",
		ResourceKind: "APIServer",
		ResourceName: "kube-apiserver",
		RawObject: fmt.Sprintf(`{"errorRatePercent": %.1f, "serverErrors": %.0f, "totalRequests": %.0f}`,
			rate*100, serverErrors, total),
	}}, nil
}

// CP-006 — etcd is responding slowly, as seen by the API server. Uses the
// etcd_request_duration_seconds histogram: if more than etcdSlowFraction of
// requests land above etcdSlowLatencySeconds, etcd (disk or network) is a drag.
func checkEtcdSlow(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	families, err := fetchAPIServerMetrics(ctx, clientset)
	if err != nil {
		return nil, err
	}

	mf, ok := families["etcd_request_duration_seconds"]
	if !ok {
		return nil, nil
	}

	total, withinLatency := aggregateHistogram(mf, etcdSlowLatencySeconds)
	if total < etcdSlowMinSamples {
		return nil, nil
	}

	slow := total - withinLatency

	fraction := slow / total
	if fraction < etcdSlowFraction {
		return nil, nil
	}

	return []clusterdoctor.RawFinding{{
		Namespace:    "kube-system",
		ResourceKind: "Etcd",
		ResourceName: "etcd",
		RawObject: fmt.Sprintf(`{"slowRequestPercent": %.2f, "thresholdSeconds": %g, "totalRequests": %.0f}`,
			fraction*100, etcdSlowLatencySeconds, total),
	}}, nil
}

// CERT-005 — the API server is presented client certificates that expire soon
// (kubelet, controller-manager, scheduler, or admin kubeconfig certs). Read
// from the apiserver_client_certificate_expiration_seconds histogram, whose
// buckets are seconds-until-expiry.
func checkClientCertExpiring(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	families, err := fetchAPIServerMetrics(ctx, clientset)
	if err != nil {
		return nil, err
	}

	mf, ok := families["apiserver_client_certificate_expiration_seconds"]
	if !ok {
		return nil, nil
	}

	total, expiringSoon := aggregateHistogram(mf, clientCertWarnSeconds)
	// The le=+Inf/total count includes requests with no client cert (recorded at
	// 0 expiry) as well as healthy long-lived certs. We only care whether any
	// authenticated request used a cert inside the warning window — that shows up
	// as a non-zero cumulative count at the window boundary, above the always-
	// present "no cert" baseline. Require a clear signal.
	if total < 1 || expiringSoon < 1 {
		return nil, nil
	}

	return []clusterdoctor.RawFinding{{
		Namespace:    "kube-system",
		ResourceKind: "APIServer",
		ResourceName: "kube-apiserver",
		RawObject: fmt.Sprintf(`{"clientCertsExpiringWithinDays": %d, "observedRequests": %.0f}`,
			clientCertWarnSeconds/86400, expiringSoon),
	}}, nil
}

// serverErrorRate sums the apiserver_request_total counter across all label
// sets, returning the fraction that are 5xx, the 5xx count and the grand total.
func serverErrorRate(mf *dto.MetricFamily) (rate, serverErrors, total float64) {
	for _, m := range mf.GetMetric() {
		v := m.GetCounter().GetValue()
		total += v

		for _, lbl := range m.GetLabel() {
			if lbl.GetName() == "code" && strings.HasPrefix(lbl.GetValue(), "5") {
				serverErrors += v
			}
		}
	}

	if total > 0 {
		rate = serverErrors / total
	}

	return rate, serverErrors, total
}

// aggregateHistogram sums, across every label combination of a histogram
// family, the total observation count and the cumulative count at or below the
// bucket whose upper bound is nearest to (but not exceeding) boundary. Bucket
// boundaries are consistent within a family, so per-boundary cumulative counts
// can be summed directly.
func aggregateHistogram(mf *dto.MetricFamily, boundary float64) (total, withinBoundary float64) {
	perBound := map[float64]float64{}

	var bounds []float64

	for _, m := range mf.GetMetric() {
		h := m.GetHistogram()
		total += float64(h.GetSampleCount())

		for _, b := range h.GetBucket() {
			ub := b.GetUpperBound()
			if _, seen := perBound[ub]; !seen {
				bounds = append(bounds, ub)
			}

			perBound[ub] += float64(b.GetCumulativeCount())
		}
	}

	sort.Float64s(bounds)

	for _, ub := range bounds {
		if ub > boundary {
			break
		}

		withinBoundary = perBound[ub]
	}

	return total, withinBoundary
}

// vpaList is the slice of the VerticalPodAutoscaler CRD we need — target
// workload and per-container recommended requests.
type vpaList struct {
	Items []struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
		Spec struct {
			TargetRef struct {
				Kind string `json:"kind"`
				Name string `json:"name"`
			} `json:"targetRef"`
		} `json:"spec"`
		Status struct {
			Recommendation struct {
				ContainerRecommendations []struct {
					ContainerName string            `json:"containerName"`
					Target        map[string]string `json:"target"`
				} `json:"containerRecommendations"`
			} `json:"recommendation"`
		} `json:"status"`
	} `json:"items"`
}

// RES-006 — a VerticalPodAutoscaler's recommendation diverges significantly
// from the target Deployment's configured requests, i.e. the workload is
// materially over- or under-provisioned versus what the VPA has learned.
// Silently not-applicable when the VPA CRD isn't installed.
func checkVPADivergence(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	raw, err := clientset.Discovery().RESTClient().Get().
		AbsPath("/apis/autoscaling.k8s.io/v1/verticalpodautoscalers").DoRaw(ctx)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil // VPA CRD not installed → not applicable
		}

		return nil, err
	}

	var list vpaList
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, vpa := range list.Items {
		if vpa.Spec.TargetRef.Kind != "Deployment" {
			continue // only Deployments are compared, to keep the lookup simple
		}

		deploy, err := clientset.AppsV1().
			Deployments(vpa.Metadata.Namespace).
			Get(ctx, vpa.Spec.TargetRef.Name, metav1.GetOptions{})
		if err != nil {
			continue // target gone or unreadable — skip this VPA, not the scan
		}

		requests := map[string]corev1.ResourceList{}
		for _, c := range deploy.Spec.Template.Spec.Containers {
			requests[c.Name] = c.Resources.Requests
		}

		for _, cr := range vpa.Status.Recommendation.ContainerRecommendations {
			reqs, ok := requests[cr.ContainerName]
			if !ok {
				continue
			}

			for _, res := range []string{"cpu", "memory"} {
				reqQty := reqs[corev1.ResourceName(res)]

				diverged, factor := quantityDiverges(reqQty, cr.Target[res])
				if !diverged {
					continue
				}

				findings = append(findings, clusterdoctor.RawFinding{
					Namespace:    vpa.Metadata.Namespace,
					ResourceKind: "Deployment",
					ResourceName: vpa.Spec.TargetRef.Name,
					RawObject: fmt.Sprintf(
						`{"vpa": %q, "container": %q, "resource": %q, "request": %q, "recommended": %q, "factor": %.2f}`,
						vpa.Metadata.Name, cr.ContainerName, res,
						reqQty.String(), cr.Target[res], factor),
				})
			}
		}
	}

	return findings, nil
}

// quantityDiverges reports whether a configured request and a VPA-recommended
// value differ by more than vpaDivergenceRatio in either direction. A missing
// or unparseable value on either side means "can't compare", not a divergence.
func quantityDiverges(request resource.Quantity, recommended string) (bool, float64) {
	if recommended == "" || request.IsZero() {
		return false, 0
	}

	recQty, err := resource.ParseQuantity(recommended)
	if err != nil || recQty.IsZero() {
		return false, 0
	}

	reqVal := request.MilliValue()
	recVal := recQty.MilliValue()

	if reqVal <= 0 || recVal <= 0 {
		return false, 0
	}

	factor := float64(recVal) / float64(reqVal)
	if factor < 1 {
		factor = 1 / factor
	}

	return factor > vpaDivergenceRatio, factor
}
