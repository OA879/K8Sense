package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/OA879/K8Sense/backend/pkg/clusterdoctor"
	"github.com/OA879/K8Sense/backend/pkg/clusterdoctor/ai"
	cddb "github.com/OA879/K8Sense/backend/pkg/clusterdoctor/db"
)

// The K8sense Copilot: a local, offline LLM grounded in the cluster's own scan
// findings. The model never sees the internet and needs no API key (see the ai
// package). Grounding — feeding the model this cluster's real, current findings
// — is what separates a useful ops assistant from a plausible-sounding one: the
// Copilot answers about the resources actually in front of the operator, not a
// generic hallucination.

// maxGroundingFindings caps how many findings are summarised into the prompt, so
// a noisy cluster can't blow the model's context window. They are severity-sorted
// first, so the most important issues always make the cut.
const maxGroundingFindings = 40

// aiClient returns the Copilot client, honouring a test override if one was set.
func (s *Server) aiClient() *ai.Client {
	if s.aiOverride != nil {
		return s.aiOverride
	}

	return ai.NewFromEnv()
}

// aiStatusResponse is what the UI polls to decide whether to offer the Copilot.
type aiStatusResponse struct {
	Enabled   bool   `json:"enabled"`
	Reachable bool   `json:"reachable"`
	Endpoint  string `json:"endpoint"`
	Model     string `json:"model"`
}

// AIStatus handles GET /cluster-doctor/ai/status .
func (s *Server) AIStatus(w http.ResponseWriter, r *http.Request) {
	client := s.aiClient()
	cfg := client.Config()

	resp := aiStatusResponse{
		Enabled:   cfg.Enabled,
		Reachable: cfg.Enabled && client.Reachable(r.Context()),
		Endpoint:  cfg.Endpoint,
		Model:     cfg.Model,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

type aiChatRequest struct {
	Cluster  string       `json:"cluster"`
	ScanID   string       `json:"scanId,omitempty"`
	Messages []ai.Message `json:"messages"`
}

type aiChatResponse struct {
	Reply string `json:"reply"`
}

// AIChat handles POST /cluster-doctor/ai/chat . It grounds the conversation in
// the cluster's latest scan findings, then relays the exchange to the local
// model.
func (s *Server) AIChat(w http.ResponseWriter, r *http.Request) {
	var req aiChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if len(req.Messages) == 0 {
		http.Error(w, `{"error":"no messages provided"}`, http.StatusBadRequest)
		return
	}

	client := s.aiClient()
	if !client.Config().Enabled {
		http.Error(w, `{"error":"The Copilot is disabled on this server","code":"ai_disabled"}`,
			http.StatusServiceUnavailable)

		return
	}

	// Best-effort live snapshot: if the cluster is reachable, ground the model in
	// its current state too. A failure here (cluster offline, RBAC) simply falls
	// back to findings-only grounding rather than blocking the chat.
	var live *liveSnapshot
	if req.Cluster != "" {
		if clientset, err := s.getClient(r, req.Cluster); err == nil {
			live = gatherLiveSnapshot(r.Context(), clientset, "")
		}
	}

	findings := s.groundingFindings(r.Context(), req.Cluster, req.ScanID)
	system := ai.Message{Role: "system", Content: renderGroundingPrompt(req.Cluster, findings, live)}
	messages := append([]ai.Message{system}, req.Messages...)

	reply, err := client.Chat(r.Context(), messages)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q,"code":"ai_unreachable"}`, err.Error()),
			http.StatusBadGateway)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(aiChatResponse{Reply: reply})
}

// renderGroundingPrompt builds the system prompt: the Copilot's role plus a
// compact, severity-sorted summary of the cluster's current findings. Pure and
// deterministic given its inputs, so it can be tested without a database.
func renderGroundingPrompt(cluster string, findings []clusterdoctor.Finding, live *liveSnapshot) string {
	var b strings.Builder

	b.WriteString(
		"You are K8sense Copilot, an expert Kubernetes SRE assistant embedded in an " +
			"air-gapped cluster-operations tool. Answer concisely and practically. Base your " +
			"answers on the CLUSTER FINDINGS and LIVE CLUSTER STATE below and on sound general " +
			"Kubernetes knowledge. Never invent resource names, namespaces, or findings that are " +
			"not present in the context. When you suggest a fix, prefer kubectl commands or " +
			"manifest changes and call out anything risky. If the context does not contain the " +
			"answer, say so plainly.\n\n")

	if cluster == "" {
		b.WriteString("No specific cluster is selected.\n")
		return b.String()
	}

	findings = append([]clusterdoctor.Finding(nil), findings...)
	sort.SliceStable(findings, func(i, j int) bool {
		return severityRank(findings[i].Severity) < severityRank(findings[j].Severity)
	})

	fmt.Fprintf(&b, "CLUSTER: %s\n", cluster)

	if len(findings) == 0 {
		b.WriteString("CLUSTER FINDINGS: none available (no recent scan, or the last scan was clean).\n")
	} else {
		fmt.Fprintf(&b, "CLUSTER FINDINGS (%s, most severe first):\n", severitySummary(findings))

		shown := findings
		if len(shown) > maxGroundingFindings {
			shown = shown[:maxGroundingFindings]
		}

		for _, f := range shown {
			resource := strings.TrimPrefix(f.Namespace+"/"+f.ResourceName, "/")
			fmt.Fprintf(&b, "- [%s] %s — %s %q: %s",
				strings.ToUpper(f.Severity), f.RuleName, f.ResourceKind, resource, f.Description)

			if f.Remediation != "" {
				fmt.Fprintf(&b, " (remediation: %s)", f.Remediation)
			}

			b.WriteByte('\n')
		}

		if len(findings) > len(shown) {
			fmt.Fprintf(&b, "...and %d more lower-severity findings.\n", len(findings)-len(shown))
		}
	}

	renderLiveSnapshot(&b, live)

	return b.String()
}

// groundingFindings loads findings for the given scan, or the cluster's most
// recent scan when no scan is specified. Errors degrade gracefully to no
// grounding rather than failing the chat.
func (s *Server) groundingFindings(ctx context.Context, cluster, scanID string) []clusterdoctor.Finding {
	if cluster == "" {
		return nil
	}

	id := scanID
	if id == "" {
		scans, err := cddb.ListScans(ctx, s.db, cluster, 1)
		if err != nil || len(scans) == 0 {
			return nil
		}

		id = scans[0].ID
	}

	findings, err := cddb.GetFindings(ctx, s.db, id)
	if err != nil {
		return nil
	}

	findings = s.enrichSuppressions(ctx, cluster, findings)

	// Drop suppressed findings — the operator has already acknowledged them, and
	// they'd only add noise to the model's context.
	kept := findings[:0]

	for _, f := range findings {
		if !f.Suppressed {
			kept = append(kept, f)
		}
	}

	sort.SliceStable(kept, func(i, j int) bool {
		return severityRank(kept[i].Severity) < severityRank(kept[j].Severity)
	})

	return kept
}

func severityRank(sev string) int {
	switch strings.ToLower(sev) {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	case "low":
		return 3
	default:
		return 4
	}
}

// severitySummary renders e.g. "2 critical, 5 high, 3 medium".
func severitySummary(findings []clusterdoctor.Finding) string {
	counts := map[string]int{}
	for _, f := range findings {
		counts[strings.ToLower(f.Severity)]++
	}

	var parts []string

	for _, sev := range []string{"critical", "high", "medium", "low"} {
		if counts[sev] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[sev], sev))
		}
	}

	if len(parts) == 0 {
		return fmt.Sprintf("%d total", len(findings))
	}

	return strings.Join(parts, ", ")
}
