package checks

import (
	"testing"

	dto "github.com/prometheus/client_model/go"
	"k8s.io/apimachinery/pkg/api/resource"
)

func strPtr(s string) *string { return &s }
func f64Ptr(f float64) *float64 { return &f }
func u64Ptr(u uint64) *uint64  { return &u }

// counterMetric builds one apiserver_request_total sample with a code label.
func counterMetric(code string, value float64) *dto.Metric {
	return &dto.Metric{
		Label:   []*dto.LabelPair{{Name: strPtr("code"), Value: strPtr(code)}},
		Counter: &dto.Counter{Value: f64Ptr(value)},
	}
}

func TestServerErrorRate(t *testing.T) {
	mf := &dto.MetricFamily{Metric: []*dto.Metric{
		counterMetric("200", 900),
		counterMetric("404", 40), // 4xx must NOT count as a server error
		counterMetric("500", 40),
		counterMetric("503", 20),
	}}

	rate, errs, total := serverErrorRate(mf)
	if total != 1000 {
		t.Fatalf("total = %v, want 1000", total)
	}
	if errs != 60 {
		t.Fatalf("serverErrors = %v, want 60 (only 5xx)", errs)
	}
	if rate < 0.059 || rate > 0.061 {
		t.Fatalf("rate = %v, want ~0.06", rate)
	}
}

// histogram builds a histogram metric from (upperBound -> cumulativeCount) pairs.
func histogram(sampleCount uint64, buckets [][2]float64) *dto.Metric {
	var bs []*dto.Bucket
	for _, b := range buckets {
		bs = append(bs, &dto.Bucket{
			UpperBound:      f64Ptr(b[0]),
			CumulativeCount: u64Ptr(uint64(b[1])),
		})
	}

	return &dto.Metric{Histogram: &dto.Histogram{
		SampleCount: u64Ptr(sampleCount),
		Bucket:      bs,
	}}
}

func TestAggregateHistogram(t *testing.T) {
	// Two label sets with identical bucket boundaries; counts should sum
	// per-boundary. Boundary of interest = 1.0s.
	mf := &dto.MetricFamily{Metric: []*dto.Metric{
		histogram(100, [][2]float64{{0.5, 80}, {1.0, 95}, {2.0, 100}}),
		histogram(100, [][2]float64{{0.5, 90}, {1.0, 98}, {2.0, 100}}),
	}}

	total, within := aggregateHistogram(mf, 1.0)
	if total != 200 {
		t.Fatalf("total = %v, want 200", total)
	}
	// cumulative count at le=1.0 is 95+98 = 193; 7 requests were slower than 1s.
	if within != 193 {
		t.Fatalf("withinBoundary = %v, want 193", within)
	}

	slowFraction := (total - within) / total
	if slowFraction < 0.034 || slowFraction > 0.036 {
		t.Fatalf("slow fraction = %v, want ~0.035", slowFraction)
	}
}

func TestQuantityDiverges(t *testing.T) {
	cases := []struct {
		name        string
		request     string
		recommended string
		wantDiverge bool
	}{
		{"cpu 3x higher", "100m", "300m", true},
		{"cpu 3x lower", "300m", "100m", true},
		{"within 1.5x", "200m", "250m", false},
		{"memory 4x", "128Mi", "512Mi", true},
		{"no recommendation", "100m", "", false},
		{"zero request", "0", "300m", false},
		{"unparseable recommendation", "100m", "banana", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := resource.MustParse(c.request)
			got, factor := quantityDiverges(req, c.recommended)
			if got != c.wantDiverge {
				t.Fatalf("quantityDiverges(%s, %s) = %v (factor %.2f), want %v",
					c.request, c.recommended, got, factor, c.wantDiverge)
			}
			if got && factor <= vpaDivergenceRatio {
				t.Fatalf("diverged but factor %.2f not above threshold %.2f", factor, vpaDivergenceRatio)
			}
		})
	}
}

func TestParseReadyzReport(t *testing.T) {
	degraded := []byte(`[+]ping ok
[+]log ok
[-]etcd failed: reason withheld
[+]poststarthook/start-kube-apiserver-admission-initializer ok
[-]informer-sync failed
healthz check failed`)

	findings := parseReadyzReport(degraded)
	if len(findings) != 2 {
		t.Fatalf("got %d findings, want 2 (etcd, informer-sync)", len(findings))
	}
	if findings[0].RawObject != `{"failingCheck": "etcd"}` {
		t.Errorf("first finding = %s, want etcd", findings[0].RawObject)
	}
	if findings[1].RawObject != `{"failingCheck": "informer-sync"}` {
		t.Errorf("second finding = %s, want informer-sync", findings[1].RawObject)
	}

	healthy := []byte("[+]ping ok\n[+]etcd ok\nreadyz check passed")
	if got := parseReadyzReport(healthy); len(got) != 0 {
		t.Fatalf("healthy readyz produced %d findings, want 0", len(got))
	}
}
