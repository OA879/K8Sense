package clusterdoctor

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// GuidedFix describes a safe, single-action remediation a Pro-tier user can
// trigger with confirmation. Action is empty when no automated fix exists
// for the rule; the finding is still shown with its remediation guide.
type GuidedFix struct {
	Action  string `yaml:"action" json:"action"`
	Warning string `yaml:"warning" json:"warning"`
}

// Rule is one entry from a rules/*.yaml file. CheckFn names the Go function
// (registered in the checks registry) that evaluates this rule against the
// cluster.
type Rule struct {
	ID             string    `yaml:"id" json:"id"`
	Name           string    `yaml:"name" json:"name"`
	Severity       string    `yaml:"severity" json:"severity"`
	Category       string    `yaml:"category" json:"category"`
	MinK8sVersion  string    `yaml:"min_k8s_version,omitempty" json:"minK8sVersion,omitempty"`
	ClusterTypes   []string  `yaml:"cluster_types,omitempty" json:"clusterTypes,omitempty"`
	CheckFn        string    `yaml:"check_fn" json:"checkFn"`
	Description    string    `yaml:"description" json:"description"`
	Remediation    string    `yaml:"remediation" json:"remediation"`
	GuidedFix      GuidedFix `yaml:"guided_fix,omitempty" json:"guidedFix,omitempty"`
	References     []string  `yaml:"references,omitempty" json:"references,omitempty"`
	Enabled        bool      `yaml:"-" json:"enabled"`
}

// ruleFile is the top-level shape of a rules/*.yaml file: a plain list of
// rules (see rules/nodes.yaml for an example).
type ruleFile []Rule

// LoadRules reads every *.yaml file in dir and returns the combined rule
// list. Rules default to enabled. A rule whose check_fn has no matching
// registered check function is still loaded (so it shows up in the Rules
// UI) but Scanner skips running it and logs a warning.
func LoadRules(dir string) ([]Rule, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading rules dir %q: %w", dir, err)
	}

	var rules []Rule

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if filepath.Ext(name) != ".yaml" && filepath.Ext(name) != ".yml" {
			continue
		}

		path := filepath.Join(dir, name)

		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading rule file %q: %w", path, err)
		}

		var file ruleFile
		if err := yaml.Unmarshal(data, &file); err != nil {
			return nil, fmt.Errorf("parsing rule file %q: %w", path, err)
		}

		for _, r := range file {
			r.Enabled = true
			rules = append(rules, r)
		}
	}

	return rules, nil
}
