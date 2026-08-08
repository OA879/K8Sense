package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mkUnsignedJWT builds an unsigned JWT with the given payload (header.payload.sig).
func mkUnsignedJWT(payload map[string]interface{}) string {
	enc := func(v interface{}) string {
		b, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(b)
	}
	return enc(map[string]string{"alg": "none"}) + "." + enc(payload) + ".sig"
}

func TestOIDC_ResolveUserAndRoleMapping(t *testing.T) {
	t.Setenv("K8SENSE_AUTH", "oidc")
	t.Setenv("K8SENSE_OIDC_ADMIN_GROUPS", "k8s-admins, platform")
	t.Setenv("K8SENSE_OIDC_OPERATOR_GROUPS", "k8s-ops")

	req := func(payload map[string]interface{}) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/x", nil)
		r.Header.Set("Authorization", "Bearer "+mkUnsignedJWT(payload))
		return r
	}

	// admin group -> admin role, username from email
	u, ok := resolveOIDCUser(req(map[string]interface{}{
		"email": "jane@bank.internal", "sub": "u-1", "groups": []string{"k8s-admins"},
	}))
	if !ok || u.Username != "jane@bank.internal" || u.Role != "admin" {
		t.Errorf("admin mapping failed: ok=%v u=%+v", ok, u)
	}

	// operator group -> operator; preferred_username used when no email
	u, _ = resolveOIDCUser(req(map[string]interface{}{
		"preferred_username": "bob", "sub": "u-2", "groups": []string{"k8s-ops"},
	}))
	if u.Role != "operator" || u.Username != "bob" {
		t.Errorf("operator mapping failed: %+v", u)
	}

	// no matching group -> default viewer
	u, _ = resolveOIDCUser(req(map[string]interface{}{"sub": "u-3", "groups": []string{"random"}}))
	if u.Role != "viewer" || u.Username != "u-3" {
		t.Errorf("default viewer mapping failed: %+v", u)
	}
}

func TestOIDC_DefaultRoleOverride(t *testing.T) {
	t.Setenv("K8SENSE_OIDC_DEFAULT_ROLE", "operator")
	if r := oidcRole([]string{"nobody"}); r != "operator" {
		t.Errorf("default role override = %q, want operator", r)
	}
	t.Setenv("K8SENSE_OIDC_DEFAULT_ROLE", "bogus")
	if r := oidcRole([]string{"nobody"}); r != "viewer" {
		t.Errorf("invalid default should fall back to viewer, got %q", r)
	}
}

func TestOIDC_NonJWTBearerRejected(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.Header.Set("Authorization", "Bearer opaque-not-a-jwt")
	if _, ok := resolveOIDCUser(r); ok {
		t.Error("opaque bearer must not resolve an OIDC user")
	}
	// no header at all
	if _, ok := resolveOIDCUser(httptest.NewRequest(http.MethodGet, "/x", nil)); ok {
		t.Error("missing bearer must not resolve a user")
	}
}
