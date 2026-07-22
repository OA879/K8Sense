package clusterdoctor_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OA879/K8Sense/backend/pkg/clusterdoctor"
)

func sampleReport(t *testing.T) string {
	t.Helper()

	data := clusterdoctor.BuildReportData(
		"prod-1", "completed", 2, 1, 1, 0, 0,
		[]clusterdoctor.Finding{
			{
				RuleID: "POD-001", RuleName: "CrashLoopBackOff",
				Severity: clusterdoctor.SeverityCritical, Namespace: "demo",
				ResourceKind: "Pod", ResourceName: "api-7c9",
				Remediation: "kubectl logs api-7c9 -n demo --previous",
			},
			{
				RuleID: "POD-008", RuleName: "Missing Resource Limits",
				Severity: clusterdoctor.SeverityWarning, Namespace: "demo",
				ResourceKind: "Pod", ResourceName: "web-1",
				Remediation: "Add resources.limits to the container spec.",
			},
		},
	)

	html, err := clusterdoctor.RenderHTMLReport(data)
	if err != nil {
		t.Fatalf("rendering report: %v", err)
	}

	return string(html)
}

// TestReportIsSelfContained is the air-gap guarantee: the report must make no
// external requests, so it renders identically on a disconnected machine and
// is safe to email as a single file.
func TestReportIsSelfContained(t *testing.T) {
	t.Parallel()

	html := sampleReport(t)

	for _, forbidden := range []string{
		"http://", "https://", "//cdn", "fonts.googleapis", "<script src=", "<link rel=\"stylesheet\"",
	} {
		if strings.Contains(html, forbidden) {
			t.Errorf("report contains external reference %q", forbidden)
		}
	}
}

// TestReportPrintsWell covers the "Save as PDF" path. K8sense has no bundled
// PDF engine on purpose — the browser renders the PDF from this same HTML, so
// there is only one report layout to maintain. These rules are what make the
// printed output usable rather than a broken screen dump.
func TestReportPrintsWell(t *testing.T) {
	t.Parallel()

	html := sampleReport(t)

	required := map[string]string{
		"@media print":             "no print stylesheet",
		"print-color-adjust":       "severity colours would be stripped by the printer",
		"table-header-group":       "table header would not repeat across pages",
		"page-break-inside: avoid": "findings could be split across a page break",
		"window.print()":           "no way to trigger printing from the report",
		"beforeprint":              "collapsed remediation would be omitted from the PDF",
	}

	for snippet, why := range required {
		if !strings.Contains(html, snippet) {
			t.Errorf("report is missing %q — %s", snippet, why)
		}
	}
}

// TestReportIncludesRemediation guards the thing a reader actually needs: a
// finding without its fix instructions is not actionable.
func TestReportIncludesRemediation(t *testing.T) {
	t.Parallel()

	html := sampleReport(t)

	for _, want := range []string{
		"api-7c9", "CrashLoopBackOff", "kubectl logs api-7c9 -n demo --previous",
		"Add resources.limits to the container spec.",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("report is missing %q", want)
		}
	}
}

// TestReportEscapesUntrustedContent: resource names come from the cluster, so
// a hostile or careless name must not be able to inject markup into a report
// that gets emailed around.
func TestReportEscapesUntrustedContent(t *testing.T) {
	t.Parallel()

	data := clusterdoctor.BuildReportData(
		"prod-1", "completed", 1, 1, 0, 0, 0,
		[]clusterdoctor.Finding{{
			RuleID: "POD-001", RuleName: "x", Severity: clusterdoctor.SeverityCritical,
			ResourceKind: "Pod",
			ResourceName: `<img src=x onerror="alert(1)">`,
			Remediation:  `<script>alert(2)</script>`,
		}},
	)

	html, err := clusterdoctor.RenderHTMLReport(data)
	if err != nil {
		t.Fatalf("rendering: %v", err)
	}

	out := string(html)

	if strings.Contains(out, `<img src=x onerror=`) {
		t.Error("resource name was not escaped — markup injection into the report")
	}

	if strings.Contains(out, "<script>alert(2)</script>") {
		t.Error("remediation was not escaped — script injection into the report")
	}
}

// TestReportHealthyState covers the zero-findings case, which is the one a
// happy customer sees.
func TestReportHealthyState(t *testing.T) {
	t.Parallel()

	data := clusterdoctor.BuildReportData("prod-1", "completed", 0, 0, 0, 0, 0, nil)

	html, err := clusterdoctor.RenderHTMLReport(data)
	if err != nil {
		t.Fatalf("rendering: %v", err)
	}

	if !strings.Contains(string(html), "healthy") {
		t.Error("a clean report should say the cluster looks healthy")
	}
}

// dumpReport writes a rendered report when K8SENSE_DUMP_REPORT is set, so the
// print layout can be eyeballed / rendered to PDF by hand.
func TestDumpReportForManualInspection(t *testing.T) {
	dir := os.Getenv("K8SENSE_DUMP_REPORT")
	if dir == "" {
		t.Skip("set K8SENSE_DUMP_REPORT=<dir> to write a sample report")
	}

	path := filepath.Join(dir, "sample-report.html")
	if err := os.WriteFile(path, []byte(sampleReport(t)), 0o600); err != nil {
		t.Fatalf("writing sample report: %v", err)
	}

	t.Logf("wrote %s", path)
}
