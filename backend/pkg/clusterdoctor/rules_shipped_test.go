package clusterdoctor_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor"
)

// repoRulesDir walks up from this package to the repo root's rules/ directory.
func repoRulesDir(t *testing.T) string {
	t.Helper()

	// backend/pkg/clusterdoctor -> backend/pkg -> backend -> repo root
	dir, err := filepath.Abs(filepath.Join("..", "..", "..", "rules"))
	if err != nil {
		t.Fatalf("resolving rules dir: %v", err)
	}

	return dir
}

// TestRuleLibraryLoadsFromRepo is the guard for the bug where a build shipped
// without the rules/ directory: the app booted fine and the entire Cluster
// Doctor engine was silently absent. This asserts the library is present and
// parses, so an accidental deletion or move fails the build rather than
// quietly disabling the product's core feature.
func TestRuleLibraryLoadsFromRepo(t *testing.T) {
	t.Parallel()

	dir := repoRulesDir(t)

	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("rules/ directory is missing at %s — Cluster Doctor would be disabled: %v", dir, err)
	}

	rules, err := clusterdoctor.LoadRules(dir)
	if err != nil {
		t.Fatalf("rule library does not parse: %v", err)
	}

	// The library is the product. A sharp drop means files went missing.
	const minExpectedRules = 40
	if len(rules) < minExpectedRules {
		t.Errorf("loaded %d rules, expected at least %d — did rule files go missing?",
			len(rules), minExpectedRules)
	}

	// Every category the product advertises must be represented.
	wantCategories := []string{
		"nodes", "pods", "control_plane", "storage",
		"network", "resources", "certificates", "workloads",
	}

	seen := map[string]int{}
	for _, r := range rules {
		seen[r.Category]++
	}

	for _, c := range wantCategories {
		if seen[c] == 0 {
			t.Errorf("no rules loaded for category %q", c)
		}
	}
}

// TestRuleLibraryHasNoDuplicateIDs guards the registry: two rules sharing an
// ID would make per-rule overrides and suppressions ambiguous.
func TestRuleLibraryHasNoDuplicateIDs(t *testing.T) {
	t.Parallel()

	rules, err := clusterdoctor.LoadRules(repoRulesDir(t))
	if err != nil {
		t.Fatalf("loading rules: %v", err)
	}

	seen := map[string]bool{}

	for _, r := range rules {
		if seen[r.ID] {
			t.Errorf("duplicate rule ID %q", r.ID)
		}

		seen[r.ID] = true
	}
}

// TestEveryRuleIsWellFormed catches a rule that would render badly in the UI
// or a report — an empty remediation is a finding a user cannot act on.
func TestEveryRuleIsWellFormed(t *testing.T) {
	t.Parallel()

	rules, err := clusterdoctor.LoadRules(repoRulesDir(t))
	if err != nil {
		t.Fatalf("loading rules: %v", err)
	}

	valid := map[string]bool{
		clusterdoctor.SeverityCritical: true,
		clusterdoctor.SeverityWarning:  true,
		clusterdoctor.SeverityInfo:     true,
	}

	for _, r := range rules {
		if r.ID == "" || r.Name == "" {
			t.Errorf("rule with empty ID or name: %+v", r)
		}

		if !valid[r.Severity] {
			t.Errorf("rule %s has invalid severity %q", r.ID, r.Severity)
		}

		if r.CheckFn == "" {
			t.Errorf("rule %s has no check_fn", r.ID)
		}

		if r.Remediation == "" {
			t.Errorf("rule %s has no remediation — a user could not act on it", r.ID)
		}
	}
}
