package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	cddb "github.com/OA879/K8Sense/backend/pkg/clusterdoctor/db"
)

func authServer(t *testing.T) *Server {
	t.Helper()
	t.Setenv("K8SENSE_AUTH", "local")
	database, err := cddb.Open(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return &Server{db: database}
}

func postJSON(t *testing.T, h http.HandlerFunc, body interface{}, sessionTok string) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewReader(b))
	if sessionTok != "" {
		req.Header.Set(sessionHeader, sessionTok)
	}
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func TestAuth_BootstrapLoginFlow(t *testing.T) {
	s := authServer(t)

	// status: needs bootstrap
	rec := httptest.NewRecorder()
	s.AuthStatus(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	var st map[string]bool
	_ = json.NewDecoder(rec.Body).Decode(&st)
	if !st["authEnabled"] || !st["needsBootstrap"] {
		t.Fatalf("expected authEnabled+needsBootstrap, got %+v", st)
	}

	// bootstrap first admin -> returns a session token
	rec = postJSON(t, s.Bootstrap, credentials{Username: "admin", Password: "supersecret"}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("bootstrap = %d: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Token string    `json:"token"`
		User  cddb.User `json:"user"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if out.Token == "" || out.User.Role != "admin" {
		t.Fatalf("bad bootstrap response: %+v", out)
	}

	// the token resolves to the admin user
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(sessionHeader, out.Token)
	if u, ok := s.resolveSessionUser(req); !ok || u.Username != "admin" {
		t.Errorf("session should resolve to admin, got ok=%v u=%+v", ok, u)
	}

	// second bootstrap is refused
	if rec := postJSON(t, s.Bootstrap, credentials{Username: "x", Password: "supersecret"}, ""); rec.Code != http.StatusConflict {
		t.Errorf("second bootstrap should 409, got %d", rec.Code)
	}

	// login with wrong password -> 401
	if rec := postJSON(t, s.Login, credentials{Username: "admin", Password: "wrong"}, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("bad password should 401, got %d", rec.Code)
	}
	// login with right password -> 200
	if rec := postJSON(t, s.Login, credentials{Username: "admin", Password: "supersecret"}, ""); rec.Code != http.StatusOK {
		t.Errorf("good login should 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAuth_ShortPasswordAndDupUser(t *testing.T) {
	s := authServer(t)
	// short password rejected at bootstrap
	if rec := postJSON(t, s.Bootstrap, credentials{Username: "a", Password: "short"}, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("short password should 400, got %d", rec.Code)
	}
	// create the admin, then a dup username via admin create
	_ = postJSON(t, s.Bootstrap, credentials{Username: "admin", Password: "supersecret"}, "")
	// build an admin-context request for CreateUser
	adminReq := func(body interface{}) *httptest.ResponseRecorder {
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewReader(b))
		u, _ := cddb.GetUserAuthByName(context.Background(), s.db, "admin")
		req = req.WithContext(context.WithValue(req.Context(), userCtxKey, u))
		rec := httptest.NewRecorder()
		s.CreateUser(rec, req)
		return rec
	}
	if rec := adminReq(createUserRequest{Username: "bob", Password: "supersecret", Role: "viewer"}); rec.Code != http.StatusOK {
		t.Fatalf("create user = %d: %s", rec.Code, rec.Body.String())
	}
	if rec := adminReq(createUserRequest{Username: "bob", Password: "supersecret", Role: "viewer"}); rec.Code != http.StatusConflict {
		t.Errorf("dup username should 409, got %d", rec.Code)
	}
}

func TestAuth_RequireAdminBlocksViewer(t *testing.T) {
	s := authServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req = req.WithContext(context.WithValue(req.Context(), userCtxKey, cddb.User{ID: "1", Username: "v", Role: "viewer"}))
	if s.requireAdmin(rec, req) {
		t.Error("viewer must not pass requireAdmin")
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}
