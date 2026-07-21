package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// makeJWT builds an unsigned JWT with the given claims. Only the payload
// matters here — actorFromRequest reads claims for display and never treats
// them as an authorisation decision.
func makeJWT(t *testing.T, claims map[string]any) string {
	t.Helper()

	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}

	enc := base64.RawURLEncoding.EncodeToString

	return enc([]byte(`{"alg":"none"}`)) + "." + enc(payload) + "." + enc([]byte("sig"))
}

func requestWithAuth(t *testing.T, header string) *http.Request {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/cluster-doctor/guided-fix", nil)
	if header != "" {
		req.Header.Set("Authorization", header)
	}

	return req
}

func TestActorPrefersEmailClaim(t *testing.T) {
	t.Parallel()

	token := makeJWT(t, map[string]any{
		"email":              "olakunle@abbeymortgagebank.com",
		"preferred_username": "olakunle",
		"sub":                "abc-123",
	})

	if got := actorFromRequest(requestWithAuth(t, "Bearer "+token)); got != "olakunle@abbeymortgagebank.com" {
		t.Errorf("got %q, want the email claim", got)
	}
}

func TestActorFallsBackThroughClaims(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		claims map[string]any
		want   string
	}{
		{"preferred_username", map[string]any{"preferred_username": "olakunle", "sub": "x"}, "olakunle"},
		{"name", map[string]any{"name": "Ola K", "sub": "x"}, "Ola K"},
		{"sub only", map[string]any{"sub": "abc-123"}, "abc-123"},
		{
			"kubernetes service account",
			map[string]any{"sub": "system:serviceaccount:kube-system:deployer"},
			"system:serviceaccount:kube-system:deployer",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			token := makeJWT(t, tc.claims)
			if got := actorFromRequest(requestWithAuth(t, "Bearer "+token)); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestActorFallsBackToHeaderForDesktop covers the desktop case, where there is
// no bearer token because the app talks to the cluster with a kubeconfig.
func TestActorFallsBackToHeaderForDesktop(t *testing.T) {
	t.Parallel()

	req := requestWithAuth(t, "")
	req.Header.Set("X-K8sense-Actor", "local-operator")

	if got := actorFromRequest(req); got != "local-operator" {
		t.Errorf("got %q, want local-operator", got)
	}
}

// TestActorIsUnknownWhenUnidentifiable is the important one: rather than
// inventing a plausible-looking actor, an unidentifiable request must be
// recorded as explicitly unknown so it looks wrong to an auditor.
func TestActorIsUnknownWhenUnidentifiable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		header string
	}{
		{"no auth header", ""},
		{"opaque non-JWT token", "Bearer abc123opaque"},
		{"malformed JWT", "Bearer a.b"},
		{"undecodable payload", "Bearer aaa.!!!not-base64!!!.ccc"},
		{"non-bearer scheme", "Basic dXNlcjpwYXNz"},
		{"JWT with no identity claims", "Bearer " + makeJWT(t, map[string]any{"iss": "x"})},
		{"JWT with blank claims", "Bearer " + makeJWT(t, map[string]any{"email": "   ", "sub": ""})},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := actorFromRequest(requestWithAuth(t, tc.header)); got != UnknownActor {
				t.Errorf("got %q, want %q", got, UnknownActor)
			}
		})
	}
}

// TestActorNeverReturnsTheToken guards against leaking credentials into the
// audit log, which is stored in plain SQLite and exported as CSV.
func TestActorNeverReturnsTheToken(t *testing.T) {
	t.Parallel()

	secret := "super-secret-token-value"

	for _, header := range []string{
		"Bearer " + secret,
		"Bearer " + makeJWT(t, map[string]any{"email": "a@b.com", "access_token": secret}),
	} {
		got := actorFromRequest(requestWithAuth(t, header))
		if got == secret || len(got) > 0 && got != UnknownActor && got == secret {
			t.Errorf("actor %q leaked the token", got)
		}

		if got == secret {
			t.Error("actor must never be the raw token")
		}
	}
}

func TestActorNilRequest(t *testing.T) {
	t.Parallel()

	if got := actorFromRequest(nil); got != UnknownActor {
		t.Errorf("got %q, want %q", got, UnknownActor)
	}
}
