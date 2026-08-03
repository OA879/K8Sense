package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCatalogApps_WellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, a := range catalogApps() {
		if a.ID == "" || a.Name == "" || a.repoURL == "" || a.chart == "" || a.releaseName == "" || a.Namespace == "" {
			t.Errorf("app %+v is missing required install coordinates", a)
		}
		if seen[a.ID] {
			t.Errorf("duplicate app id %q", a.ID)
		}
		seen[a.ID] = true

		got, ok := findCatalogApp(a.ID)
		if !ok || got.chart != a.chart {
			t.Errorf("findCatalogApp(%q) did not round-trip", a.ID)
		}
	}
	if _, ok := findCatalogApp("does-not-exist"); ok {
		t.Error("unknown app id should not resolve")
	}
}

// Catalog without a wired config provider must still render (all not-installed),
// never 500 — the page should work even before Helm is reachable.
func TestCatalog_NoProviderStillLists(t *testing.T) {
	s := &Server{} // getClientConfig nil
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/cluster-doctor/catalog?cluster=x", nil)

	s.Catalog(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Apps []catalogAppView `json:"apps"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Apps) != len(catalogApps()) {
		t.Errorf("returned %d apps, want %d", len(body.Apps), len(catalogApps()))
	}
	for _, a := range body.Apps {
		if a.Installed {
			t.Errorf("%s should be not-installed with no provider", a.ID)
		}
	}
}
