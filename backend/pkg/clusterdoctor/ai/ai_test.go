package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChat_ParsesReply(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		var req chatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "qwen2.5" {
			t.Errorf("model = %q", req.Model)
		}
		_ = json.NewEncoder(w).Encode(chatResponse{
			Choices: []struct {
				Message Message `json:"message"`
			}{{Message: Message{Role: "assistant", Content: "  scale the deployment  "}}},
		})
	}))
	defer srv.Close()

	c := New(Config{Enabled: true, Endpoint: srv.URL, Model: "qwen2.5"})
	reply, err := c.Chat(context.Background(), []Message{{Role: "user", Content: "help"}})
	if err != nil {
		t.Fatal(err)
	}
	if reply != "scale the deployment" {
		t.Errorf("reply = %q, want trimmed content", reply)
	}
}

func TestChat_DisabledAndUnreachable(t *testing.T) {
	if _, err := New(Config{Enabled: false}).Chat(context.Background(), []Message{{Role: "user"}}); err == nil {
		t.Error("disabled client should error")
	}
	c := New(Config{Enabled: true, Endpoint: "http://127.0.0.1:1", Model: "x"})
	if c.Reachable(context.Background()) {
		t.Error("dead endpoint should not be reachable")
	}
}

func TestConfigFromEnv_Defaults(t *testing.T) {
	t.Setenv("K8SENSE_AI_ENDPOINT", "")
	t.Setenv("K8SENSE_AI_MODEL", "")
	t.Setenv("K8SENSE_AI_DISABLED", "")
	cfg := ConfigFromEnv()
	if cfg.Endpoint != defaultEndpoint || cfg.Model != defaultModel || !cfg.Enabled {
		t.Errorf("defaults wrong: %+v", cfg)
	}
	t.Setenv("K8SENSE_AI_DISABLED", "1")
	if ConfigFromEnv().Enabled {
		t.Error("K8SENSE_AI_DISABLED=1 should disable")
	}
}
