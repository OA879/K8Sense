package clusterdoctor

// ScanDiff is the result of comparing two scans' findings: which findings are
// new in the current scan, which were present before but are now resolved, and
// which persist across both. A finding's identity across scans is
// (ruleId, namespace, resourceKind, resourceName) — the same rule firing on the
// same resource is treated as the same finding even if its unique ID differs.
type ScanDiff struct {
	Added     []Finding `json:"added"`
	Resolved  []Finding `json:"resolved"`
	Persisted []Finding `json:"persisted"`
}

// findingKey builds the cross-scan identity key for a finding.
func findingKey(f Finding) string {
	return f.RuleID + "|" + f.Namespace + "|" + f.ResourceKind + "|" + f.ResourceName
}

// DiffFindings compares current against previous. Added holds findings in
// current but not previous; Resolved holds findings in previous but not
// current; Persisted holds findings present in both (the current copy).
func DiffFindings(current, previous []Finding) ScanDiff {
	prevKeys := make(map[string]struct{}, len(previous))
	for _, f := range previous {
		prevKeys[findingKey(f)] = struct{}{}
	}

	currKeys := make(map[string]struct{}, len(current))
	for _, f := range current {
		currKeys[findingKey(f)] = struct{}{}
	}

	diff := ScanDiff{
		Added:     []Finding{},
		Resolved:  []Finding{},
		Persisted: []Finding{},
	}

	for _, f := range current {
		if _, ok := prevKeys[findingKey(f)]; ok {
			diff.Persisted = append(diff.Persisted, f)
		} else {
			diff.Added = append(diff.Added, f)
		}
	}

	for _, f := range previous {
		if _, ok := currKeys[findingKey(f)]; !ok {
			diff.Resolved = append(diff.Resolved, f)
		}
	}

	return diff
}
