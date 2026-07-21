package clusterdoctor

import "testing"

func TestParseRulesValid(t *testing.T) {
	yaml := `
- id: CUSTOM-001
  name: My Rule
  severity: WARNING
  category: custom
  check_fn: check_something
  description: test
  remediation: fix it
`
	rules, err := ParseRules([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseRules: %v", err)
	}

	if len(rules) != 1 || rules[0].ID != "CUSTOM-001" || !rules[0].Enabled {
		t.Fatalf("unexpected parse result: %+v", rules)
	}
}

func TestParseRulesRejectsMissingFields(t *testing.T) {
	yaml := `
- id: BAD-001
  name: Missing severity and category
`
	if _, err := ParseRules([]byte(yaml)); err == nil {
		t.Fatal("expected error for missing required fields")
	}
}

func TestParseRulesRejectsBadSeverity(t *testing.T) {
	yaml := `
- id: BAD-002
  name: Bad severity
  severity: URGENT
  category: custom
`
	if _, err := ParseRules([]byte(yaml)); err == nil {
		t.Fatal("expected error for invalid severity")
	}
}

func TestParseRulesRejectsEmpty(t *testing.T) {
	if _, err := ParseRules([]byte("[]")); err == nil {
		t.Fatal("expected error for empty document")
	}
}
