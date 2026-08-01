package api

import (
	"strings"
	"testing"

	"github.com/OA879/K8Sense/backend/pkg/clusterdoctor"
)

func TestRenderGroundingPrompt_SortsAndSummarises(t *testing.T) {
	findings := []clusterdoctor.Finding{
		{Severity: "medium", RuleName: "cpu-noise", ResourceKind: "Deployment", Namespace: "app", ResourceName: "api", Description: "no cpu limit"},
		{Severity: "critical", RuleName: "priv-pod", ResourceKind: "Pod", Namespace: "app", ResourceName: "root-shell", Description: "runs privileged", Remediation: "drop privileged"},
	}

	prompt := renderGroundingPrompt("prod", findings)

	if !strings.Contains(prompt, "K8sense Copilot") {
		t.Error("missing role framing")
	}
	if !strings.Contains(prompt, "CLUSTER: prod") {
		t.Error("missing cluster name")
	}
	// Critical must be summarised and listed before the medium finding.
	if !strings.Contains(prompt, "1 critical, 1 medium") {
		t.Errorf("bad severity summary in:\n%s", prompt)
	}
	crit := strings.Index(prompt, "root-shell")
	med := strings.Index(prompt, "api")
	if crit == -1 || med == -1 || crit > med {
		t.Errorf("critical finding should precede medium; crit=%d med=%d", crit, med)
	}
	if !strings.Contains(prompt, "drop privileged") {
		t.Error("remediation not included")
	}
}

func TestRenderGroundingPrompt_NoContext(t *testing.T) {
	if !strings.Contains(renderGroundingPrompt("", nil), "No specific cluster") {
		t.Error("empty cluster should be stated")
	}
	if !strings.Contains(renderGroundingPrompt("prod", nil), "none available") {
		t.Error("no-findings case should be stated")
	}
}
