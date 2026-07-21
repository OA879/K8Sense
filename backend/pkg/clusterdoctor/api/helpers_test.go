package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor"
	cdapi "github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor/api"
	cddb "github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor/db"
)

const testCluster = "test-cluster"

var errUnknownCluster = errors.New("cluster not found")

// testEnv is a fully wired Cluster Doctor server backed by a throwaway SQLite
// file and a fake Kubernetes clientset, routed through the real
// RegisterRoutes table so tests can't drift from production routing.
type testEnv struct {
	t      *testing.T
	db     *sql.DB
	router *mux.Router
	dir    string
	scans  int
	fake   *k8sfake.Clientset
}

// newTestEnv builds an isolated server. Everything (database, licence,
// branding, role) lives under t.TempDir(), so tests never touch the
// developer's real K8sense config.
func newTestEnv(t *testing.T, objects ...runtime.Object) *testEnv {
	t.Helper()

	dir := t.TempDir()

	database, err := cddb.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	clientset := k8sfake.NewSimpleClientset(objects...)

	getClient := func(_ *http.Request, cluster string) (kubernetes.Interface, error) {
		if cluster != testCluster {
			return nil, errUnknownCluster
		}

		return clientset, nil
	}

	server := cdapi.NewServer(database, testRules(), getClient,
		filepath.Join(dir, "licence.k8sense-licence"))

	router := mux.NewRouter()
	server.RegisterRoutes(router)

	return &testEnv{t: t, db: database, router: router, dir: dir, fake: clientset}
}

// clientset exposes the fake cluster so tests can assert on real side effects
// (a pod actually deleted, a deployment actually scaled).
func (e *testEnv) clientset() *k8sfake.Clientset { return e.fake }

// do issues a request through the real router and returns the recorder.
func (e *testEnv) do(method, target string, body any) *httptest.ResponseRecorder {
	e.t.Helper()

	var payload []byte

	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			e.t.Fatalf("marshalling request body: %v", err)
		}

		payload = raw
	}

	req := httptest.NewRequest(method, target, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)

	return rec
}

// decode unmarshals a recorder body into v, failing the test on bad JSON.
func (e *testEnv) decode(rec *httptest.ResponseRecorder, v any) {
	e.t.Helper()

	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		e.t.Fatalf("decoding response %q: %v", rec.Body.String(), err)
	}
}

// setRole writes the role config directly, simulating an already-configured
// install without going through the (admin-gated) endpoint.
func (e *testEnv) setRole(role clusterdoctor.Role) {
	e.t.Helper()

	if err := clusterdoctor.SaveRole(filepath.Join(e.dir, "role.json"), role); err != nil {
		e.t.Fatalf("saving role: %v", err)
	}
}

// grantPro activates the built-in trial so Pro-gated endpoints are reachable.
func (e *testEnv) grantPro() {
	e.t.Helper()

	if rec := e.do(http.MethodPost, "/cluster-doctor/licence/trial", nil); rec.Code != http.StatusOK {
		e.t.Fatalf("starting trial: got %d, body %s", rec.Code, rec.Body.String())
	}
}

// seedScan persists a completed scan with the given findings and returns its id.
func (e *testEnv) seedScan(findings ...clusterdoctor.Finding) string {
	e.t.Helper()

	e.scans++
	id := fmt.Sprintf("scan-%d", e.scans)

	for i := range findings {
		findings[i].ScanID = id
		if findings[i].ID == "" {
			findings[i].ID = fmt.Sprintf("%s-f%d", id, i)
		}
	}

	result := clusterdoctor.ScanResult{
		ID:      id,
		Cluster: testCluster,
		// Space scans apart so "most recent" ordering is deterministic.
		StartedAt:   time.Now().Add(time.Duration(e.scans) * time.Second),
		CompletedAt: time.Now().Add(time.Duration(e.scans) * time.Second),
		Status:      clusterdoctor.ScanCompleted,
		Findings:    findings,
	}

	if err := cddb.SaveScan(context.Background(), e.db, &result); err != nil {
		e.t.Fatalf("seeding scan: %v", err)
	}

	return id
}

// finding builds a Finding with sensible defaults for tests.
func finding(ruleID, severity, kind, namespace, name string) clusterdoctor.Finding {
	return clusterdoctor.Finding{
		RuleID:       ruleID,
		RuleName:     ruleID + " name",
		Severity:     severity,
		Category:     "pods",
		Namespace:    namespace,
		ResourceKind: kind,
		ResourceName: name,
		Description:  "description",
		Remediation:  "remediation",
		DetectedAt:   time.Now(),
	}
}

// testRules is a small deterministic rule set: one rule with a guided fix and
// one without, so guided-fix enrichment and gating are both exercisable.
func testRules() []clusterdoctor.Rule {
	return []clusterdoctor.Rule{
		{
			ID: "POD-001", Name: "CrashLoopBackOff", Severity: clusterdoctor.SeverityCritical,
			Category: "pods", CheckFn: "check_crashloop", Enabled: true,
			GuidedFix: clusterdoctor.GuidedFix{Action: "delete_pod", Warning: "Pod will restart."},
		},
		{
			ID: "POD-008", Name: "Missing Resource Limits", Severity: clusterdoctor.SeverityWarning,
			Category: "pods", CheckFn: "check_missing_limits", Enabled: true,
		},
	}
}
