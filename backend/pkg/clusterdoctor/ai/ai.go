// Package ai integrates a local, offline large-language model as the K8sense
// Copilot. It speaks the OpenAI-compatible chat API that every mainstream local
// runtime (Ollama, vLLM, llama.cpp, LM Studio) exposes, so a single client
// works against whichever the operator runs. Nothing here reaches the public
// internet: the default endpoint is a loopback Ollama server, and the model
// weights live on the operator's own hardware. There are no API keys, no
// accounts, and no telemetry — a hard requirement for the air-gapped, regulated
// environments K8sense targets.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultEndpoint = "http://localhost:11434/v1" // Ollama's OpenAI-compatible API
	defaultModel    = "qwen2.5"                   // Apache-2.0, ships with the appliance
	chatTimeout     = 120 * time.Second
	pingTimeout     = 3 * time.Second
)

// Config describes where the local model lives. It is fully operator-controlled
// via environment variables so a customer can point K8sense at an existing
// internal inference server (their own vLLM/Granite box) without a rebuild.
type Config struct {
	Enabled  bool   `json:"enabled"`
	Endpoint string `json:"endpoint"`
	Model    string `json:"model"`
}

// ConfigFromEnv resolves the Copilot configuration:
//
//	K8SENSE_AI_ENDPOINT  base URL of the OpenAI-compatible server (default loopback Ollama)
//	K8SENSE_AI_MODEL     model name/tag to request (default qwen2.5)
//	K8SENSE_AI_DISABLED  set to 1/true to turn the Copilot off entirely
//
// The Copilot is on by default: a loopback model is not egress, so it is safe
// even under K8SENSE_AIRGAPPED. Operators who don't run a model simply see the
// Copilot report itself as unreachable.
func ConfigFromEnv() Config {
	cfg := Config{
		Enabled:  !truthy(os.Getenv("K8SENSE_AI_DISABLED")),
		Endpoint: strings.TrimRight(getenv("K8SENSE_AI_ENDPOINT", defaultEndpoint), "/"),
		Model:    getenv("K8SENSE_AI_MODEL", defaultModel),
	}

	return cfg
}

// Message is one turn in a chat conversation.
type Message struct {
	Role    string `json:"role"` // system | user | assistant
	Content string `json:"content"`
}

// Client talks to a local OpenAI-compatible chat endpoint.
type Client struct {
	cfg  Config
	http *http.Client
}

// New builds a Client for cfg.
func New(cfg Config) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: chatTimeout}}
}

// NewFromEnv builds a Client from the process environment.
func NewFromEnv() *Client {
	return New(ConfigFromEnv())
}

// Config returns the client's resolved configuration.
func (c *Client) Config() Config { return c.cfg }

// chat request/response mirror the subset of the OpenAI schema we rely on. Every
// compliant runtime returns at least this shape.
type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Stream      bool      `json:"stream"`
	Temperature float64   `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Chat sends messages to the local model and returns the assistant's reply.
func (c *Client) Chat(ctx context.Context, messages []Message) (string, error) {
	if !c.cfg.Enabled {
		return "", fmt.Errorf("the K8sense Copilot is disabled")
	}

	body, err := json.Marshal(chatRequest{
		Model:       c.cfg.Model,
		Messages:    messages,
		Stream:      false,
		Temperature: 0.2, //nolint:mnd // low temperature: grounded, deterministic ops answers
	})
	if err != nil {
		return "", fmt.Errorf("encoding chat request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.Endpoint+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("building chat request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("the local model is unreachable at %s: %w", c.cfg.Endpoint, err)
	}
	defer resp.Body.Close()

	var parsed chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("decoding model response: %w", err)
	}

	if parsed.Error != nil && parsed.Error.Message != "" {
		return "", fmt.Errorf("model error: %s", parsed.Error.Message)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("model returned HTTP %d", resp.StatusCode)
	}

	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("model returned no choices")
	}

	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

// Reachable reports whether the configured model server answers. It is cheap
// (GET /models with a short timeout) so the UI can show a live status dot.
func (c *Client) Reachable(ctx context.Context) bool {
	if !c.cfg.Enabled {
		return false
	}

	ctx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.Endpoint+"/models", nil)
	if err != nil {
		return false
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode >= 200 && resp.StatusCode < 500
}

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}

	return fallback
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
