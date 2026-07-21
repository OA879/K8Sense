package clusterdoctor

import (
	"bytes"
	"fmt"
	"html/template"
	"time"
)

// ReportData is everything the HTML report template needs. Scan metadata is
// passed as plain fields (rather than importing the db package here, which
// would create an import cycle) so the reporter stays dependency-free.
type ReportData struct {
	Cluster       string
	GeneratedAt   string
	Status        string
	TotalFindings int
	CriticalCount int
	WarningCount  int
	InfoCount     int
	SkippedChecks int
	Findings      []Finding
}

// RenderHTMLReport produces a fully self-contained HTML report — no external
// stylesheets, scripts, fonts, or images, so it renders identically on an
// air-gapped machine and is safe to email as a single file. Findings are
// assumed already severity-sorted by the caller (db.GetFindings does this).
func RenderHTMLReport(data ReportData) ([]byte, error) {
	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"sevClass": severityCSSClass,
	}).Parse(reportTemplate)
	if err != nil {
		return nil, fmt.Errorf("parsing report template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("executing report template: %w", err)
	}

	return buf.Bytes(), nil
}

// BuildReportData assembles a ReportData from a cluster name, the scan's
// summary counts, and its findings. Kept as a helper so the API handler
// doesn't hand-build the struct.
func BuildReportData(
	cluster, status string,
	total, critical, warning, info, skipped int,
	findings []Finding,
) ReportData {
	return ReportData{
		Cluster:       cluster,
		GeneratedAt:   time.Now().Format("2006-01-02 15:04:05 MST"),
		Status:        status,
		TotalFindings: total,
		CriticalCount: critical,
		WarningCount:  warning,
		InfoCount:     info,
		SkippedChecks: skipped,
		Findings:      findings,
	}
}

func severityCSSClass(severity string) string {
	switch severity {
	case SeverityCritical:
		return "critical"
	case SeverityWarning:
		return "warning"
	default:
		return "info"
	}
}

// reportTemplate is inlined (not an embedded file) so the report generator is
// a single compiled unit with no runtime file dependency. Uses a system font
// stack and inline CSS — zero external requests.
const reportTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>K8sense Cluster Doctor Report — {{.Cluster}}</title>
<style>
  :root { color-scheme: light; }
  * { box-sizing: border-box; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
    margin: 0; background: #F8FAFC; color: #0F172A; line-height: 1.5;
  }
  .wrap { max-width: 1000px; margin: 0 auto; padding: 32px 24px 64px; }
  header { display: flex; align-items: center; gap: 12px; margin-bottom: 4px; }
  .logo {
    width: 34px; height: 34px; border-radius: 8px; background: #0F172A; color: #3B82F6;
    font-weight: 800; font-size: 20px; display: flex; align-items: center; justify-content: center;
  }
  h1 { font-size: 22px; margin: 0; font-weight: 800; }
  h1 .accent { color: #3B82F6; }
  .sub { color: #475569; margin: 2px 0 24px; font-size: 14px; }
  .cards { display: flex; flex-wrap: wrap; gap: 12px; margin-bottom: 28px; }
  .card {
    flex: 1 1 120px; background: #fff; border: 1px solid #E2E8F0; border-radius: 10px;
    padding: 14px 16px;
  }
  .card .n { font-size: 26px; font-weight: 800; }
  .card .l { font-size: 12px; text-transform: uppercase; letter-spacing: .04em; color: #64748B; }
  .card.critical .n { color: #EF4444; }
  .card.warning .n { color: #F59E0B; }
  .card.info .n { color: #3B82F6; }
  table { width: 100%; border-collapse: collapse; background: #fff; border-radius: 10px; overflow: hidden; border: 1px solid #E2E8F0; }
  th, td { text-align: left; padding: 10px 12px; font-size: 13px; border-bottom: 1px solid #F1F5F9; vertical-align: top; }
  th { background: #F1F5F9; font-size: 11px; text-transform: uppercase; letter-spacing: .04em; color: #475569; }
  .badge { display: inline-block; padding: 2px 8px; border-radius: 999px; color: #fff; font-weight: 700; font-size: 11px; }
  .badge.critical { background: #EF4444; }
  .badge.warning { background: #F59E0B; }
  .badge.info { background: #60A5FA; }
  .rule-id { font-family: ui-monospace, SFMono-Regular, "JetBrains Mono", Menlo, monospace; color: #475569; }
  details { margin-top: 6px; }
  summary { cursor: pointer; color: #3B82F6; font-size: 12px; }
  pre { white-space: pre-wrap; font-family: ui-monospace, SFMono-Regular, "JetBrains Mono", Menlo, monospace;
        font-size: 12px; background: #F8FAFC; padding: 10px; border-radius: 6px; margin: 6px 0 0; }
  .empty { text-align: center; padding: 48px; color: #10B981; font-weight: 600; }
  footer { margin-top: 32px; font-size: 12px; color: #94A3B8; }

  .actions { margin: 0 0 20px; }
  .btn {
    font: inherit; font-size: 13px; font-weight: 600; cursor: pointer;
    background: #0F172A; color: #fff; border: 0; border-radius: 6px; padding: 8px 14px;
  }

  /* Print / "Save as PDF". The browser renders the PDF from this same HTML, so
     there is no second report engine to keep in sync. */
  @media print {
    @page { margin: 14mm; }
    body { background: #fff; }
    .wrap { max-width: none; padding: 0; }
    .actions { display: none; }
    /* Severity colours must survive the printer's colour-stripping. */
    * { -webkit-print-color-adjust: exact; print-color-adjust: exact; }
    /* Repeat the table header on every printed page. */
    thead { display: table-header-group; }
    /* Never split a finding across a page break. */
    tr, .card { break-inside: avoid; page-break-inside: avoid; }
    table, .cards { break-inside: auto; }
    /* Remediation is collapsed on screen; in print it must all be visible,
       otherwise the PDF silently omits the fix instructions. */
    details { display: block; }
    details > summary { display: none; }
    details > pre { display: block; background: #F8FAFC; }
    footer { margin-top: 16px; }
  }
</style>
</head>
<body>
<div class="wrap">
  <header>
    <div class="logo">8</div>
    <h1>K<span class="accent">8</span>sense — Cluster Doctor Report</h1>
  </header>
  <div class="sub">
    Cluster <strong>{{.Cluster}}</strong> · Generated {{.GeneratedAt}} · Status: {{.Status}}
    {{if gt .SkippedChecks 0}} · {{.SkippedChecks}} checks skipped{{end}}
  </div>

  <div class="actions">
    <button class="btn" onclick="window.print()">Print / Save as PDF</button>
  </div>

  <div class="cards">
    <div class="card critical"><div class="n">{{.CriticalCount}}</div><div class="l">Critical</div></div>
    <div class="card warning"><div class="n">{{.WarningCount}}</div><div class="l">Warning</div></div>
    <div class="card info"><div class="n">{{.InfoCount}}</div><div class="l">Info</div></div>
    <div class="card"><div class="n">{{.TotalFindings}}</div><div class="l">Total</div></div>
  </div>

  {{if .Findings}}
  <table>
    <thead>
      <tr><th>Severity</th><th>Rule</th><th>Name</th><th>Kind</th><th>Namespace</th><th>Resource &amp; Remediation</th></tr>
    </thead>
    <tbody>
      {{range .Findings}}
      <tr>
        <td><span class="badge {{sevClass .Severity}}">{{.Severity}}</span></td>
        <td class="rule-id">{{.RuleID}}</td>
        <td>{{.RuleName}}</td>
        <td>{{.ResourceKind}}</td>
        <td>{{if .Namespace}}{{.Namespace}}{{else}}—{{end}}</td>
        <td>
          <strong>{{.ResourceName}}</strong>
          <details>
            <summary>Remediation</summary>
            <pre>{{.Remediation}}</pre>
          </details>
        </td>
      </tr>
      {{end}}
    </tbody>
  </table>
  {{else}}
  <div class="empty">✓ No findings — this cluster looks healthy.</div>
  {{end}}

  <footer>
    Generated by K8sense Cluster Doctor. This report was produced locally; no cluster data left your machine.
  </footer>
</div>
<script>
  // Expand all remediation blocks for printing so the PDF contains the fix
  // instructions, then restore the collapsed state afterwards.
  (function () {
    var opened = [];
    window.addEventListener('beforeprint', function () {
      opened = [];
      document.querySelectorAll('details').forEach(function (d) {
        if (!d.open) { opened.push(d); d.open = true; }
      });
    });
    window.addEventListener('afterprint', function () {
      opened.forEach(function (d) { d.open = false; });
      opened = [];
    });
  })();
</script>
</body>
</html>`
