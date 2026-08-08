package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"

	"github.com/OA879/K8Sense/backend/pkg/clusterdoctor"
	cddb "github.com/OA879/K8Sense/backend/pkg/clusterdoctor/db"
	"github.com/OA879/K8Sense/backend/pkg/logger"
)

// Local authentication for the multi-user web deployment. Accounts live in
// K8sense's own database (bcrypt-hashed passwords, opaque session tokens), so it
// works fully offline — no external identity provider. It is opt-in via
// K8SENSE_AUTH=local; unset, the single-user desktop app behaves exactly as
// before. The design is provider-shaped: "local" is the first provider, and
// OIDC/AD can be added later behind the same session + current-user plumbing
// without touching the handlers.

const (
	sessionHeader  = "X-K8SENSE-SESSION"
	sessionTTL     = 12 * time.Hour
	minPasswordLen = 8
	bcryptCost     = 12
)

type userCtxKeyType struct{}

var userCtxKey = userCtxKeyType{}

// authEnabled reports whether authentication is on (any provider) — the web
// deployment. Off by default, so the single-user desktop app is unaffected.
func authEnabled() bool {
	m := authMode()
	return m == "local" || m == "oidc"
}

// hashPassword returns a bcrypt hash.
func hashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcryptCost)

	return string(b), err
}

// newSessionToken returns a 256-bit random, URL-safe token.
func newSessionToken() (string, error) {
	b := make([]byte, 32) //nolint:mnd // 256-bit token
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}

// currentUser returns the authenticated user for the request, if any. When auth
// is disabled it returns a synthetic admin so single-user installs keep full
// access and audit still has an actor.
func currentUser(r *http.Request) cddb.User {
	if u, ok := r.Context().Value(userCtxKey).(cddb.User); ok {
		return u
	}

	return cddb.User{ID: "local", Username: "local", Role: string(clusterdoctor.DefaultRole)}
}

// resolveSessionUser looks up the request's session token and returns the live,
// non-disabled user it belongs to.
func (s *Server) resolveSessionUser(r *http.Request) (cddb.User, bool) {
	token := strings.TrimSpace(r.Header.Get(sessionHeader))
	if token == "" {
		return cddb.User{}, false
	}

	userID, ok, err := cddb.SessionUser(r.Context(), s.db, token, time.Now().Unix())
	if err != nil || !ok {
		return cddb.User{}, false
	}

	user, ok, err := cddb.GetUser(r.Context(), s.db, userID)
	if err != nil || !ok || user.Disabled {
		return cddb.User{}, false
	}

	return user, true
}

// authPublicPaths are reachable without a session (so you can log in / set up).
func isAuthPublicPath(path string) bool {
	switch path {
	case "/cluster-doctor/auth/status", "/cluster-doctor/auth/login", "/cluster-doctor/auth/bootstrap":
		return true
	default:
		return false
	}
}

// authMiddleware enforces a valid session on every /cluster-doctor endpoint when
// auth is enabled, stashing the resolved user in the request context. It is a
// no-op when auth is disabled, so the desktop app is unaffected.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !authEnabled() || isAuthPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// OIDC mode derives identity from the cluster's id_token; local mode from
		// a K8sense session.
		var (
			user cddb.User
			ok   bool
		)

		if oidcMode() {
			user, ok = resolveOIDCUser(r)
		} else {
			user, ok = s.resolveSessionUser(r)
		}

		if !ok {
			http.Error(w, `{"error":"Please sign in.","code":"unauthenticated"}`, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userCtxKey, user)))
	})
}

// ---- handlers ----

// AuthStatus handles GET /cluster-doctor/auth/status — tells the UI whether auth
// is on and whether the install still needs its first admin.
func (s *Server) AuthStatus(w http.ResponseWriter, r *http.Request) {
	mode := authMode()
	resp := map[string]interface{}{"authEnabled": authEnabled(), "mode": mode, "needsBootstrap": false}

	// Bootstrap only applies to local accounts; OIDC identities come from the IdP.
	if mode == "local" {
		n, err := cddb.CountUsers(r.Context(), s.db)
		if err == nil {
			resp["needsBootstrap"] = n == 0
		}
	}

	writeJSON(w, resp)
}

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Bootstrap handles POST /cluster-doctor/auth/bootstrap — creates the first admin
// account. Allowed only while no users exist, so it can't be used to add admins
// later.
func (s *Server) Bootstrap(w http.ResponseWriter, r *http.Request) {
	if !authEnabled() {
		http.Error(w, `{"error":"Authentication is not enabled on this server."}`, http.StatusBadRequest)
		return
	}

	n, err := cddb.CountUsers(r.Context(), s.db)
	if err != nil {
		http.Error(w, `{"error":"could not read users"}`, http.StatusInternalServerError)
		return
	}

	if n > 0 {
		http.Error(w, `{"error":"Setup is already complete — sign in instead."}`, http.StatusConflict)
		return
	}

	var req credentials
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := s.createUser(r, req, string(clusterdoctor.RoleAdmin)); err != nil {
		writeAuthError(w, err)
		return
	}

	logger.Log(logger.LevelInfo, map[string]string{"username": req.Username}, nil, "auth: first admin created")
	s.issueSession(w, r, req.Username, req.Password)
}

// Login handles POST /cluster-doctor/auth/login.
func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	var req credentials
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	s.issueSession(w, r, req.Username, req.Password)
}

// issueSession verifies credentials and, on success, creates a session and
// returns the token + user. The same generic error is used for unknown user and
// bad password so it can't be used to enumerate accounts.
func (s *Server) issueSession(w http.ResponseWriter, r *http.Request, username, password string) {
	user, hash, ok, err := cddb.GetUserAuth(r.Context(), s.db, strings.TrimSpace(username))
	if err != nil {
		http.Error(w, `{"error":"could not sign in"}`, http.StatusInternalServerError)
		return
	}

	if !ok || user.Disabled || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		http.Error(w, `{"error":"Incorrect username or password."}`, http.StatusUnauthorized)
		return
	}

	token, err := newSessionToken()
	if err != nil {
		http.Error(w, `{"error":"could not sign in"}`, http.StatusInternalServerError)
		return
	}

	now := time.Now()
	if err := cddb.CreateSession(r.Context(), s.db, token, user.ID, now.Unix(), now.Add(sessionTTL).Unix()); err != nil {
		http.Error(w, `{"error":"could not sign in"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]interface{}{"token": token, "user": user})
}

// Logout handles POST /cluster-doctor/auth/logout — revokes the current session.
func (s *Server) Logout(w http.ResponseWriter, r *http.Request) {
	if token := strings.TrimSpace(r.Header.Get(sessionHeader)); token != "" {
		_ = cddb.DeleteSession(r.Context(), s.db, token)
	}

	writeJSON(w, map[string]string{"result": "ok"})
}

// Me handles GET /cluster-doctor/auth/me — the current user.
func (s *Server) Me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]interface{}{"user": currentUser(r), "authEnabled": authEnabled()})
}

// ---- user management (admin) ----

// ListUsers handles GET /cluster-doctor/users (admin only).
func (s *Server) ListUsers(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}

	users, err := cddb.ListUsers(r.Context(), s.db)
	if err != nil {
		http.Error(w, `{"error":"could not list users"}`, http.StatusInternalServerError)
		return
	}

	if users == nil {
		users = []cddb.User{}
	}

	writeJSON(w, map[string]interface{}{"users": users})
}

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// CreateUser handles POST /cluster-doctor/users (admin only).
func (s *Server) CreateUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}

	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	role := req.Role
	if !clusterdoctor.Role(role).Valid() {
		role = string(clusterdoctor.RoleViewer)
	}

	if err := s.createUser(r, credentials{Username: req.Username, Password: req.Password}, role); err != nil {
		writeAuthError(w, err)
		return
	}

	writeJSON(w, map[string]string{"result": "ok"})
}

type updateUserRequest struct {
	Role     *string `json:"role,omitempty"`
	Disabled *bool   `json:"disabled,omitempty"`
}

// UpdateUser handles PUT /cluster-doctor/users/{id} (admin only) — change role or
// enable/disable. An admin cannot disable or demote their own account (so an
// install can't be locked out of administration).
func (s *Server) UpdateUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}

	id := mux.Vars(r)["id"]

	if id == currentUser(r).ID {
		http.Error(w, `{"error":"You can't change your own admin account here."}`, http.StatusBadRequest)
		return
	}

	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Role != nil {
		if !clusterdoctor.Role(*req.Role).Valid() {
			http.Error(w, `{"error":"unknown role"}`, http.StatusBadRequest)
			return
		}

		if err := cddb.SetUserRole(r.Context(), s.db, id, *req.Role); err != nil {
			http.Error(w, `{"error":"could not update role"}`, http.StatusInternalServerError)
			return
		}
	}

	if req.Disabled != nil {
		if err := cddb.SetUserDisabled(r.Context(), s.db, id, *req.Disabled); err != nil {
			http.Error(w, `{"error":"could not update status"}`, http.StatusInternalServerError)
			return
		}

		if *req.Disabled {
			_ = cddb.DeleteUserSessions(r.Context(), s.db, id) // force logout
		}
	}

	writeJSON(w, map[string]string{"result": "ok"})
}

// createUser validates and inserts an account with the given role.
func (s *Server) createUser(r *http.Request, req credentials, role string) error {
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		return authErr{http.StatusBadRequest, "A username is required."}
	}

	if len(req.Password) < minPasswordLen {
		return authErr{http.StatusBadRequest, "Password must be at least 8 characters."}
	}

	hash, err := hashPassword(req.Password)
	if err != nil {
		return authErr{http.StatusInternalServerError, "could not hash password"}
	}

	err = cddb.CreateUser(r.Context(), s.db, uuid.NewString(), req.Username, hash, role, time.Now().Unix())
	if err != nil {
		if err == cddb.ErrUserExists {
			return authErr{http.StatusConflict, "That username is taken."}
		}

		return authErr{http.StatusInternalServerError, "could not create user"}
	}

	return nil
}

// requireAdmin writes a 403 and returns false unless the current user is an admin.
func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if !clusterdoctor.Role(currentUser(r).Role).AtLeast(clusterdoctor.RoleAdmin) {
		http.Error(w, `{"error":"Admins only."}`, http.StatusForbidden)
		return false
	}

	return true
}

type authErr struct {
	code int
	msg  string
}

func (e authErr) Error() string { return e.msg }

func writeAuthError(w http.ResponseWriter, err error) {
	if ae, ok := err.(authErr); ok {
		body, _ := json.Marshal(map[string]string{"error": ae.msg})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(ae.code)
		_, _ = w.Write(body)

		return
	}

	http.Error(w, `{"error":"authentication error"}`, http.StatusInternalServerError)
}
