package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
)

// UnknownActor is recorded when no identity can be derived from the request.
// It is deliberately explicit rather than a friendly default: an audit row
// that cannot name its actor should look wrong to whoever reads it.
const UnknownActor = "unknown"

// actorClaims are the identity claims we look for, in preference order. These
// cover OIDC providers (email / preferred_username) and Kubernetes service
// account tokens (sub = system:serviceaccount:ns:name).
type actorClaims struct {
	Email             string `json:"email"`
	PreferredUsername string `json:"preferred_username"`
	Name              string `json:"name"`
	Subject           string `json:"sub"`
}

// actorFromRequest derives who is performing an action, for the audit trail.
//
// Trust model: K8sense does not authenticate users itself. The bearer token on
// the request is the same token used against the Kubernetes API server, so if
// the action succeeded, the API server accepted that token — the identity
// inside it is therefore as trustworthy as the cluster's own authentication.
// We parse the claims for display; we do NOT treat this as an authorisation
// decision, and we never log or store the token itself.
//
// Order: OIDC/JWT claims → explicit X-K8sense-Actor header → UnknownActor.
func actorFromRequest(r *http.Request) string {
	if r == nil {
		return UnknownActor
	}

	if actor := actorFromBearerToken(r.Header.Get("Authorization")); actor != "" {
		return actor
	}

	// Desktop installs have no bearer token; the app may name the local user.
	if header := strings.TrimSpace(r.Header.Get("X-K8sense-Actor")); header != "" {
		return header
	}

	return UnknownActor
}

// actorFromBearerToken pulls an identity claim out of a JWT bearer token.
// Returns "" when the header is absent or isn't a parseable JWT (for example
// an opaque token), so the caller can fall back.
func actorFromBearerToken(authHeader string) string {
	const bearerPrefix = "Bearer "

	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return ""
	}

	token := strings.TrimSpace(strings.TrimPrefix(authHeader, bearerPrefix))

	// header.payload.signature — we only read the payload.
	parts := strings.Split(token, ".")
	if len(parts) != 3 { //nolint:mnd // a JWT has exactly three segments
		return ""
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}

	var claims actorClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}

	for _, candidate := range []string{
		claims.Email, claims.PreferredUsername, claims.Name, claims.Subject,
	} {
		if c := strings.TrimSpace(candidate); c != "" {
			return c
		}
	}

	return ""
}
