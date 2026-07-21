package clusterdoctor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Role is K8sense's in-app permission level. It gates which K8sense actions
// the UI offers and the server accepts.
//
// IMPORTANT — what this is and isn't: until SSO lands, the role is read from a
// local config file, so it is an *operational guardrail*, not an identity-based
// security boundary. It stops an operator from fat-fingering a destructive
// action and lets a shop run K8sense read-only, but a determined local user can
// edit the file. Real enforcement needs the cluster's own RBAC (which still
// applies to every request K8sense makes) plus SSO-backed identity.
type Role string

const (
	// RoleViewer can scan and read findings, but change nothing.
	RoleViewer Role = "viewer"
	// RoleOperator can additionally apply guided fixes and suppress findings.
	RoleOperator Role = "operator"
	// RoleAdmin can additionally manage rules, licence, branding and settings.
	RoleAdmin Role = "admin"
)

// DefaultRole is what an un-configured install runs as. Admin keeps a fresh
// single-user install fully functional; shops that want restriction opt in.
const DefaultRole = RoleAdmin

var roleRank = map[Role]int{
	RoleViewer:   0,
	RoleOperator: 1,
	RoleAdmin:    2,
}

// Valid reports whether r is a known role.
func (r Role) Valid() bool {
	_, ok := roleRank[r]
	return ok
}

// AtLeast reports whether r meets or exceeds the required role.
func (r Role) AtLeast(required Role) bool {
	have, ok := roleRank[r]
	if !ok {
		return false
	}

	need, ok := roleRank[required]
	if !ok {
		return false
	}

	return have >= need
}

type roleConfig struct {
	Role Role `json:"role"`
}

// LoadRole reads the configured role from path, defaulting to DefaultRole when
// the file is missing or names an unknown role.
func LoadRole(path string) (Role, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return DefaultRole, nil
	}

	if err != nil {
		return DefaultRole, fmt.Errorf("reading role config: %w", err)
	}

	var cfg roleConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DefaultRole, fmt.Errorf("parsing role config: %w", err)
	}

	if !cfg.Role.Valid() {
		return DefaultRole, nil
	}

	return cfg.Role, nil
}

// SaveRole persists the in-app role.
func SaveRole(path string, r Role) error {
	if !r.Valid() {
		return fmt.Errorf("unknown role %q", r)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating role config directory: %w", err)
	}

	data, err := json.MarshalIndent(roleConfig{Role: r}, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding role config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing role config: %w", err)
	}

	return nil
}
