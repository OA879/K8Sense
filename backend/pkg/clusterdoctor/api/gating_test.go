package api_test

import (
	"net/http"
	"testing"

	"github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor"
)

// gatedEndpoint describes one request and the gate it is expected to enforce.
type gatedEndpoint struct {
	name   string
	method string
	target string
	body   any
}

// proGatedEndpoints must all return 402 on a Free licence. If someone adds a
// Pro feature and forgets requirePaid, adding it here catches it.
func proGatedEndpoints() []gatedEndpoint {
	return []gatedEndpoint{
		{
			"guided fix", http.MethodPost, "/cluster-doctor/guided-fix",
			map[string]any{
				"cluster": testCluster, "action": "delete_pod", "namespace": "demo",
				"resourceName": "p", "confirmed": true,
			},
		},
		{
			"notification config", http.MethodPut, "/cluster-doctor/notifications",
			map[string]any{"cluster": testCluster, "notifyCritical": true, "intervalMinutes": 60},
		},
		{
			"branding", http.MethodPut, "/cluster-doctor/branding",
			map[string]any{"productName": "AcmeOps"},
		},
	}
}

func TestProFeaturesBlockedOnFreeLicence(t *testing.T) {
	t.Parallel()

	for _, ep := range proGatedEndpoints() {
		t.Run(ep.name, func(t *testing.T) {
			t.Parallel()

			env := newTestEnv(t)

			rec := env.do(ep.method, ep.target, ep.body)
			if rec.Code != http.StatusPaymentRequired {
				t.Errorf("%s on Free: got %d, want 402 (body: %s)",
					ep.name, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestProFeaturesAllowedOnTrial(t *testing.T) {
	t.Parallel()

	for _, ep := range proGatedEndpoints() {
		t.Run(ep.name, func(t *testing.T) {
			t.Parallel()

			env := newTestEnv(t)
			env.grantPro()

			rec := env.do(ep.method, ep.target, ep.body)
			// The request may still fail on its own merits (e.g. the fake
			// cluster has no such pod), but it must not be blocked by licence.
			if rec.Code == http.StatusPaymentRequired {
				t.Errorf("%s on trial: still 402 (body: %s)", ep.name, rec.Body.String())
			}
		})
	}
}

// roleGatedEndpoints maps each write endpoint to the minimum role it needs.
func roleGatedEndpoints() []struct {
	gatedEndpoint
	required clusterdoctor.Role
} {
	return []struct {
		gatedEndpoint
		required clusterdoctor.Role
	}{
		{
			gatedEndpoint{
				"suppress", http.MethodPost, "/cluster-doctor/findings/suppress",
				map[string]any{
					"cluster": testCluster, "ruleId": "POD-001",
					"resourceKind": "Pod", "resourceName": "p", "reason": "known",
				},
			},
			clusterdoctor.RoleOperator,
		},
		{
			gatedEndpoint{
				"unsuppress", http.MethodPost, "/cluster-doctor/findings/unsuppress",
				map[string]any{
					"cluster": testCluster, "ruleId": "POD-001",
					"resourceKind": "Pod", "resourceName": "p",
				},
			},
			clusterdoctor.RoleOperator,
		},
		{
			gatedEndpoint{
				"comment", http.MethodPut, "/cluster-doctor/findings/comment",
				map[string]any{
					"cluster": testCluster, "ruleId": "POD-001",
					"resourceKind": "Pod", "resourceName": "p", "comment": "note",
				},
			},
			clusterdoctor.RoleOperator,
		},
		{
			gatedEndpoint{
				"rule toggle", http.MethodPut,
				"/cluster-doctor/rules/POD-001/toggle?cluster=" + testCluster + "&enabled=false", nil,
			},
			clusterdoctor.RoleAdmin,
		},
		{
			gatedEndpoint{
				"rule severity", http.MethodPut,
				"/cluster-doctor/rules/POD-001/severity?cluster=" + testCluster + "&severity=INFO", nil,
			},
			clusterdoctor.RoleAdmin,
		},
		{
			gatedEndpoint{
				"set role", http.MethodPut, "/cluster-doctor/role",
				map[string]any{"role": "admin"},
			},
			clusterdoctor.RoleAdmin,
		},
	}
}

func TestViewerCannotWrite(t *testing.T) {
	t.Parallel()

	for _, ep := range roleGatedEndpoints() {
		t.Run(ep.name, func(t *testing.T) {
			t.Parallel()

			env := newTestEnv(t)
			env.grantPro() // isolate the role gate from the licence gate
			env.setRole(clusterdoctor.RoleViewer)

			rec := env.do(ep.method, ep.target, ep.body)
			if rec.Code != http.StatusForbidden {
				t.Errorf("viewer %s: got %d, want 403 (body: %s)",
					ep.name, rec.Code, rec.Body.String())
			}
		})
	}
}

// TestOperatorCannotDoAdminActions guards the middle rung of the ladder — the
// easiest one to get wrong.
func TestOperatorCannotDoAdminActions(t *testing.T) {
	t.Parallel()

	for _, ep := range roleGatedEndpoints() {
		if ep.required != clusterdoctor.RoleAdmin {
			continue
		}

		t.Run(ep.name, func(t *testing.T) {
			t.Parallel()

			env := newTestEnv(t)
			env.grantPro()
			env.setRole(clusterdoctor.RoleOperator)

			rec := env.do(ep.method, ep.target, ep.body)
			if rec.Code != http.StatusForbidden {
				t.Errorf("operator %s: got %d, want 403 (body: %s)",
					ep.name, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestOperatorCanDoOperatorActions(t *testing.T) {
	t.Parallel()

	for _, ep := range roleGatedEndpoints() {
		if ep.required != clusterdoctor.RoleOperator {
			continue
		}

		t.Run(ep.name, func(t *testing.T) {
			t.Parallel()

			env := newTestEnv(t)
			env.grantPro()
			env.setRole(clusterdoctor.RoleOperator)

			rec := env.do(ep.method, ep.target, ep.body)
			if rec.Code == http.StatusForbidden {
				t.Errorf("operator %s was forbidden but should be allowed (body: %s)",
					ep.name, rec.Body.String())
			}
		})
	}
}

// TestViewerCannotPromoteItself is the privilege-escalation case: a read-only
// install must not be able to grant itself write access through the API.
func TestViewerCannotPromoteItself(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.setRole(clusterdoctor.RoleViewer)

	rec := env.do(http.MethodPut, "/cluster-doctor/role", map[string]any{"role": "admin"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer self-promotion: got %d, want 403", rec.Code)
	}

	// And the role on disk must be unchanged.
	var got struct {
		Role string `json:"role"`
	}

	env.decode(env.do(http.MethodGet, "/cluster-doctor/role", nil), &got)

	if got.Role != string(clusterdoctor.RoleViewer) {
		t.Errorf("role after failed promotion = %q, want viewer", got.Role)
	}
}

// TestReadsAlwaysAllowedForViewer ensures the role gate never blocks
// diagnostics — a read-only install must still be fully useful.
func TestReadsAlwaysAllowedForViewer(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.setRole(clusterdoctor.RoleViewer)

	scanID := env.seedScan(finding("POD-001", clusterdoctor.SeverityCritical, "Pod", "demo", "p1"))

	reads := []string{
		"/cluster-doctor/rules?cluster=" + testCluster,
		"/cluster-doctor/history?cluster=" + testCluster,
		"/cluster-doctor/findings/" + scanID,
		"/cluster-doctor/audit-log?cluster=" + testCluster,
		"/cluster-doctor/notifications?cluster=" + testCluster,
		"/cluster-doctor/branding",
		"/cluster-doctor/role",
		"/cluster-doctor/licence",
		"/cluster-doctor/storage",
	}

	for _, target := range reads {
		rec := env.do(http.MethodGet, target, nil)
		if rec.Code != http.StatusOK {
			t.Errorf("viewer GET %s: got %d, want 200 (body: %s)",
				target, rec.Code, rec.Body.String())
		}
	}
}
