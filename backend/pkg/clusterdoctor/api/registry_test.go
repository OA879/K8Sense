package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRewriteImage(t *testing.T) {
	cases := []struct{ ref, registry, want string }{
		{"quay.io/ansible/ansible-runner:latest", "reg.internal", "reg.internal/ansible/ansible-runner:latest"},
		{"aquasec/trivy:latest", "reg.internal", "reg.internal/aquasec/trivy:latest"},
		{"nginx:1.25", "reg.internal", "reg.internal/nginx:1.25"},
		{"registry.k8s.io/metrics-server/metrics-server:v0.7", "reg.internal", "reg.internal/metrics-server/metrics-server:v0.7"},
		{"quay.io/ansible/ansible-runner:latest", "", "quay.io/ansible/ansible-runner:latest"},                       // no registry = no-op
		{"reg.internal/ansible/ansible-runner:latest", "reg.internal", "reg.internal/ansible/ansible-runner:latest"}, // already internal
		{"reg.internal:5000/x/y:1", "reg.internal:5000", "reg.internal:5000/x/y:1"},                                  // registry with port, already internal
	}
	for _, c := range cases {
		if got := rewriteImage(c.ref, c.registry); got != c.want {
			t.Errorf("rewriteImage(%q,%q) = %q, want %q", c.ref, c.registry, got, c.want)
		}
	}
}

func TestFindLocalChart(t *testing.T) {
	dir := t.TempDir()
	if findLocalChart(dir, "grafana") != "" {
		t.Error("no chart present should return empty")
	}
	// Two versions present -> newest (lexically last) wins.
	for _, f := range []string{"grafana-7.0.0.tgz", "grafana-7.3.0.tgz", "other-1.0.0.tgz"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got := findLocalChart(dir, "grafana")
	if filepath.Base(got) != "grafana-7.3.0.tgz" {
		t.Errorf("findLocalChart picked %q, want grafana-7.3.0.tgz", filepath.Base(got))
	}
}

func TestChartValues_InjectsRegistry(t *testing.T) {
	dir := t.TempDir()
	s := &Server{licencePath: filepath.Join(dir, "l")}
	_ = os.WriteFile(s.registryConfigPath(), []byte(`{"registry":"reg.internal"}`), 0o600)

	app := catalogApp{values: map[string]interface{}{"foo": "bar"}}
	vals := s.chartValues(app)

	global, ok := vals["global"].(map[string]interface{})
	if !ok || global["imageRegistry"] != "reg.internal" {
		t.Errorf("global.imageRegistry not injected: %+v", vals)
	}
	if vals["foo"] != "bar" {
		t.Error("existing values must be preserved")
	}
	// Original app.values must not be mutated.
	if _, exists := app.values["global"]; exists {
		t.Error("chartValues mutated the template's values map")
	}
}
