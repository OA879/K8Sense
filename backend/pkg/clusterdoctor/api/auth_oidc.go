package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/OA879/K8Sense/backend/pkg/clusterdoctor"
	cddb "github.com/OA879/K8Sense/backend/pkg/clusterdoctor/db"
)

// OIDC / directory provider. When K8SENSE_AUTH=oidc, K8sense reuses the cluster's
// own OIDC identity instead of local accounts: the Headlamp base already runs the
// OIDC login flow and obtains an id_token for cluster access (and it works
// air-gapped with an on-prem issuer — AD FS, Keycloak). This bridge turns that
// id_token into the K8sense app user, mapping OIDC group claims to K8sense roles.
//
// Trust model (same as the audit actor): the id_token on the request is the token
// used against the Kubernetes API, so if the cluster accepts it, the identity is
// as trustworthy as the cluster's own authentication — and every K8sense action
// still hits the cluster, where RBAC is the real boundary. The K8sense role only
// gates K8sense's own UI actions. Cryptographic verification of the token
// signature against the issuer is the hardening step to add before production
// (the base already builds a per-cluster verifier that can be reused here).

// authMode returns the configured authentication mode: "local", "oidc", or ""
// (off — the single-user desktop default).
func authMode() string {
	return strings.ToLower(strings.TrimSpace(os.Getenv("K8SENSE_AUTH")))
}

func oidcMode() bool { return authMode() == "oidc" }

// oidcClaims are the identity claims K8sense reads from the id_token.
type oidcClaims struct {
	Email             string   `json:"email"`
	PreferredUsername string   `json:"preferred_username"`
	Name              string   `json:"name"`
	Subject           string   `json:"sub"`
	Groups            []string `json:"groups"`
}

// parseOIDCBearer decodes the (unverified) claims from a JWT bearer token.
// ok is false when the header is absent or not a parseable JWT.
func parseOIDCBearer(authHeader string) (oidcClaims, bool) {
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return oidcClaims{}, false
	}

	parts := strings.Split(strings.TrimSpace(strings.TrimPrefix(authHeader, bearerPrefix)), ".")
	if len(parts) != 3 { //nolint:mnd // a JWT has exactly three segments
		return oidcClaims{}, false
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return oidcClaims{}, false
	}

	var claims oidcClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return oidcClaims{}, false
	}

	return claims, true
}

// oidcUsername picks the most human-friendly identity claim.
func oidcUsername(c oidcClaims) string {
	for _, v := range []string{c.Email, c.PreferredUsername, c.Name, c.Subject} {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}

	return ""
}

// oidcRole maps the user's OIDC groups to a K8sense role using the configured
// group lists, defaulting to K8SENSE_OIDC_DEFAULT_ROLE (or viewer). Admin wins
// over operator wins over the default.
//
//	K8SENSE_OIDC_ADMIN_GROUPS     comma-separated groups granted admin
//	K8SENSE_OIDC_OPERATOR_GROUPS  comma-separated groups granted operator
//	K8SENSE_OIDC_DEFAULT_ROLE     role for everyone else (default: viewer)
func oidcRole(groups []string) clusterdoctor.Role {
	has := func(env string) bool {
		return intersects(groups, splitCSV(os.Getenv(env)))
	}

	if has("K8SENSE_OIDC_ADMIN_GROUPS") {
		return clusterdoctor.RoleAdmin
	}

	if has("K8SENSE_OIDC_OPERATOR_GROUPS") {
		return clusterdoctor.RoleOperator
	}

	if def := clusterdoctor.Role(strings.TrimSpace(os.Getenv("K8SENSE_OIDC_DEFAULT_ROLE"))); def.Valid() {
		return def
	}

	return clusterdoctor.RoleViewer
}

// resolveOIDCUser derives the K8sense user from the request's OIDC id_token.
func resolveOIDCUser(r *http.Request) (cddb.User, bool) {
	claims, ok := parseOIDCBearer(r.Header.Get("Authorization"))
	if !ok {
		return cddb.User{}, false
	}

	name := oidcUsername(claims)
	if name == "" {
		return cddb.User{}, false
	}

	id := claims.Subject
	if id == "" {
		id = name
	}

	return cddb.User{ID: id, Username: name, Role: string(oidcRole(claims.Groups))}, true
}

func splitCSV(s string) []string {
	var out []string

	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}

	return out
}

func intersects(a, b []string) bool {
	set := make(map[string]struct{}, len(b))
	for _, x := range b {
		set[x] = struct{}{}
	}

	for _, x := range a {
		if _, ok := set[x]; ok {
			return true
		}
	}

	return false
}
