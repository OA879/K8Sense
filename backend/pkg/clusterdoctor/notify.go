package clusterdoctor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// webhookTimeout bounds how long a notification POST may take. Alerting must
// never stall or fail a scan, so this is deliberately short.
const webhookTimeout = 10 * time.Second

// maxListedFindings caps how many findings are named in a notification before
// it switches to a "+N more" summary — chat webhooks reject very large bodies.
const maxListedFindings = 10

// NotificationPayload is the scan outcome an alert describes.
type NotificationPayload struct {
	Cluster      string
	NewCritical  []Finding
	TotalCritial int
	ScanID       string
}

// SlackMessage renders the payload as Slack Block Kit text. Slack accepts a
// simple {"text": ...} body, which renders reliably in every workspace
// without needing app-specific block permissions.
func SlackMessage(p NotificationPayload) []byte {
	var b strings.Builder

	fmt.Fprintf(&b, "*K8sense — %d new critical finding(s) on `%s`*\n", len(p.NewCritical), p.Cluster)

	for i, f := range p.NewCritical {
		if i >= maxListedFindings {
			fmt.Fprintf(&b, "_…and %d more._\n", len(p.NewCritical)-maxListedFindings)
			break
		}

		fmt.Fprintf(&b, "• `%s` %s — %s\n", f.RuleID, f.RuleName, resourceLabel(f))
	}

	body, _ := json.Marshal(map[string]string{"text": b.String()})

	return body
}

// TeamsMessage renders the payload as a Microsoft Teams MessageCard, the
// format Teams incoming webhooks accept.
func TeamsMessage(p NotificationPayload) []byte {
	var facts []map[string]string

	for i, f := range p.NewCritical {
		if i >= maxListedFindings {
			break
		}

		facts = append(facts, map[string]string{
			"name":  f.RuleID,
			"value": fmt.Sprintf("%s — %s", f.RuleName, resourceLabel(f)),
		})
	}

	card := map[string]any{
		"@type":      "MessageCard",
		"@context":   "http://schema.org/extensions",
		"themeColor": "EF4444",
		"summary":    fmt.Sprintf("K8sense: %d new critical findings on %s", len(p.NewCritical), p.Cluster),
		"title":      fmt.Sprintf("K8sense — %d new critical finding(s)", len(p.NewCritical)),
		"sections": []map[string]any{{
			"activityTitle": fmt.Sprintf("Cluster **%s**", p.Cluster),
			"facts":         facts,
		}},
	}

	body, _ := json.Marshal(card)

	return body
}

func resourceLabel(f Finding) string {
	if f.Namespace != "" {
		return fmt.Sprintf("%s/%s", f.Namespace, f.ResourceName)
	}

	return f.ResourceName
}

// PostWebhook delivers a rendered notification body to url. Any non-2xx is
// reported as an error so the caller can log it; delivery is best-effort and
// never blocks or fails the scan that triggered it.
func PostWebhook(ctx context.Context, url string, body []byte) error {
	ctx, cancel := context.WithTimeout(ctx, webhookTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building webhook request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("posting webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned %d", resp.StatusCode)
	}

	return nil
}

// NewCriticalFindings returns the critical findings present in current but not
// in previous, keyed on resource identity — the "what just broke" set that
// warrants waking someone up.
func NewCriticalFindings(current, previous []Finding) []Finding {
	diff := DiffFindings(current, previous)

	var out []Finding

	for _, f := range diff.Added {
		if f.Severity == SeverityCritical {
			out = append(out, f)
		}
	}

	return out
}
