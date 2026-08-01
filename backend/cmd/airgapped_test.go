package main

import (
	"testing"

	"github.com/OA879/K8Sense/backend/pkg/headlampconfig"
)

func TestApplyAirGappedMode(t *testing.T) {
	newConfig := func() *HeadlampConfig {
		on := true
		c := &HeadlampConfig{HeadlampConfig: &headlampconfig.HeadlampConfig{
			HeadlampCFG: &headlampconfig.HeadlampCFG{},
		}}
		c.ProxyURLs = []string{"https://artifacthub.io/*"}
		c.TelemetryConfig.TracingEnabled = &on
		c.TelemetryConfig.MetricsEnabled = &on
		return c
	}

	t.Run("off by default", func(t *testing.T) {
		t.Setenv("K8SENSE_AIRGAPPED", "")
		c := newConfig()
		if applyAirGappedMode(c) || c.AirGapped {
			t.Fatal("air-gapped should be off without the env var")
		}
		if len(c.ProxyURLs) != 1 {
			t.Error("proxy URLs should be untouched when not air-gapped")
		}
	})

	t.Run("enabled drops proxy + telemetry", func(t *testing.T) {
		t.Setenv("K8SENSE_AIRGAPPED", "1")
		c := newConfig()
		if !applyAirGappedMode(c) || !c.AirGapped {
			t.Fatal("air-gapped should be on with K8SENSE_AIRGAPPED=1")
		}
		if c.ProxyURLs != nil || c.compiledProxyURLs != nil {
			t.Errorf("proxy allowlist must be cleared, got %v", c.ProxyURLs)
		}
		if *c.TelemetryConfig.TracingEnabled || *c.TelemetryConfig.MetricsEnabled {
			t.Error("telemetry tracing/metrics must be forced off")
		}
	})
}
