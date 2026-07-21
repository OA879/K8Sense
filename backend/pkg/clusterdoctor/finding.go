// Package clusterdoctor implements K8sense's rule-driven Kubernetes health
// scanner: it loads YAML rule definitions, runs their associated check
// functions against a cluster, and produces structured Findings.
package clusterdoctor

import "time"

// Severity levels for a Finding, ordered from least to most urgent.
const (
	SeverityInfo     = "INFO"
	SeverityWarning  = "WARNING"
	SeverityCritical = "CRITICAL"
)

// Finding is one diagnostic result produced by running a Rule's check
// function against a resource in the cluster. Description and Remediation
// are copied from the Rule at scan time so historical findings keep reading
// correctly even if the rule text changes later.
type Finding struct {
	ID           string    `json:"id"`
	ScanID       string    `json:"scanId"`
	RuleID       string    `json:"ruleId"`
	RuleName     string    `json:"ruleName"`
	Severity     string    `json:"severity"`
	Category     string    `json:"category"`
	Namespace    string    `json:"namespace"`
	ResourceKind string    `json:"resourceKind"`
	ResourceName string    `json:"resourceName"`
	Description  string    `json:"description"`
	Remediation  string    `json:"remediation"`
	References   []string  `json:"references,omitempty"`
	RawObject    string    `json:"rawObject,omitempty"`
	DetectedAt   time.Time `json:"detectedAt"`

	GuidedFixAvailable bool   `json:"guidedFixAvailable"`
	GuidedFixAction    string `json:"guidedFixAction,omitempty"`
	GuidedFixWarning   string `json:"guidedFixWarning,omitempty"`

	// Suppressed and Comment are not persisted on the finding row (findings
	// are per-scan snapshots). They're derived when findings are read back,
	// from the resource-keyed suppressions table — see api.enrichSuppressions.
	Suppressed bool   `json:"suppressed"`
	Comment    string `json:"comment,omitempty"`
}

// RawFinding is what a check function returns: everything about the
// affected resource, but none of the rule-level metadata (severity,
// description, remediation) which the scanner fills in afterwards.
type RawFinding struct {
	Namespace    string
	ResourceKind string
	ResourceName string
	RawObject    string
}
