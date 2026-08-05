package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Internal-registry support: in a fully air-gapped install, every container
// image K8sense runs (the Ansible runner, the Trivy scanner) and every chart it
// installs must come from the customer's own registry — the cluster nodes can't
// reach the public internet. Setting one internal registry rewrites all
// K8sense-controlled image references to it, so the air-gap is configured in a
// single place rather than per feature.

func (s *Server) registryConfigPath() string {
	return filepath.Join(s.configDir(), "registry.json")
}

// internalRegistry resolves the configured internal registry host (e.g.
// "registry.bank.internal"), from the saved setting then the environment. Empty
// means "no rewrite" — images resolve to their public defaults.
func (s *Server) internalRegistry() string {
	if data, err := os.ReadFile(s.registryConfigPath()); err == nil {
		var c struct {
			Registry string `json:"registry"`
		}

		if json.Unmarshal(data, &c) == nil && strings.TrimSpace(c.Registry) != "" {
			return strings.TrimSpace(c.Registry)
		}
	}

	return strings.TrimSpace(os.Getenv("K8SENSE_INTERNAL_REGISTRY"))
}

// resolveImage rewrites an image reference onto the internal registry, if one is
// set. It's a no-op otherwise.
func (s *Server) resolveImage(ref string) string {
	return rewriteImage(ref, s.internalRegistry())
}

// rewriteImage replaces an image's registry host with the internal registry,
// preserving the repository path and tag. Docker-Hub-style refs (no host) are
// prefixed. Already-internal refs are left alone.
//
//	quay.io/ansible/ansible-runner:latest  -> reg/ansible/ansible-runner:latest
//	aquasec/trivy:latest                   -> reg/aquasec/trivy:latest
func rewriteImage(ref, registry string) string {
	registry = strings.Trim(strings.TrimSpace(registry), "/")
	if registry == "" || strings.TrimSpace(ref) == "" {
		return ref
	}

	if strings.HasPrefix(ref, registry+"/") {
		return ref
	}

	_, remainder := splitRegistry(ref)

	return registry + "/" + remainder
}

// splitRegistry separates an optional registry host from the rest of a reference,
// using Docker's rule: the first path segment is a host only if it contains a
// "." or ":", or is "localhost".
func splitRegistry(ref string) (host, remainder string) {
	i := strings.IndexByte(ref, '/')
	if i < 0 {
		return "", ref
	}

	first := ref[:i]
	if strings.ContainsAny(first, ".:") || first == "localhost" {
		return first, ref[i+1:]
	}

	return "", ref
}

type registrySetting struct {
	Registry  string `json:"registry"`
	AirGapped bool   `json:"airGapped"`
}

// GetRegistry handles GET /cluster-doctor/registry .
func (s *Server) GetRegistry(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, registrySetting{Registry: s.internalRegistry(), AirGapped: airGappedMode()})
}

// SetRegistry handles PUT /cluster-doctor/registry — persist the internal
// registry host that every feature's images are rewritten onto.
func (s *Server) SetRegistry(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Registry string `json:"registry"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	// Normalise: strip scheme and trailing slashes; empty clears the override.
	req.Registry = strings.TrimSpace(req.Registry)
	req.Registry = strings.TrimPrefix(req.Registry, "https://")
	req.Registry = strings.TrimPrefix(req.Registry, "http://")
	req.Registry = strings.Trim(req.Registry, "/")

	data, _ := json.MarshalIndent(map[string]string{"registry": req.Registry}, "", "  ")
	if err := os.WriteFile(s.registryConfigPath(), data, 0o600); err != nil {
		http.Error(w, `{"error":"could not save the registry setting"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, registrySetting{Registry: s.internalRegistry(), AirGapped: airGappedMode()})
}
