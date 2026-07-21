// Package api implements the HTTP handlers behind the /cluster-doctor/*
// routes: starting a scan, streaming its progress over SSE, and reading
// back findings and scan history. It's deliberately decoupled from
// Headlamp's HeadlampConfig — the caller supplies a ClientProvider closure
// that knows how to turn a cluster name into an authenticated client-go
// clientset, so this package doesn't need to know about kubeconfig storage,
// auth tokens, or any of that.
package api

import (
	"context"
	"database/sql"
	"net/http"
	"sync"
	"time"

	"k8s.io/client-go/kubernetes"

	"github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor"
	cddb "github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor/db"
	"github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor/licence"
	"github.com/kubernetes-sigs/headlamp/backend/pkg/logger"
)

// scanTimeout bounds how long a single scan is allowed to run before its
// context is cancelled; per K8SENSE_CONTEXT.md this is meant to become a
// configurable 30-600s setting, but a fixed default is fine for Phase 1.
const scanTimeout = 120 * time.Second

// ClientProvider resolves an authenticated clientset for clusterName, using
// whatever request context (auth token, etc.) the caller's HTTP layer needs.
type ClientProvider func(r *http.Request, clusterName string) (kubernetes.Interface, error)

// Server holds everything the /cluster-doctor/* handlers need.
type Server struct {
	db        *sql.DB
	rules     []clusterdoctor.Rule
	rulesByID map[string]clusterdoctor.Rule
	getClient ClientProvider

	mu     sync.Mutex
	active map[string]*liveScan

	licenceMu   sync.RWMutex
	licenceInfo licence.Info
	licencePath string
}

// NewServer builds a Server. rules is the fully loaded rule set (built-in +
// custom); getClient resolves clusters by name; licencePath points at the
// (possibly absent) licence file, validated immediately so tier gating is
// live from the first request.
func NewServer(
	database *sql.DB,
	rules []clusterdoctor.Rule,
	getClient ClientProvider,
	licencePath string,
) *Server {
	byID := make(map[string]clusterdoctor.Rule, len(rules))
	for _, rule := range rules {
		byID[rule.ID] = rule
	}

	return &Server{
		db:          database,
		rules:       rules,
		rulesByID:   byID,
		getClient:   getClient,
		active:      map[string]*liveScan{},
		licencePath: licencePath,
		licenceInfo: licence.Validate(licencePath),
	}
}

// AddRules appends newly-imported custom rules to the in-memory rule set so
// they take effect on the next scan without a restart.
func (s *Server) AddRules(rules []clusterdoctor.Rule) {
	s.rules = append(s.rules, rules...)
	for _, rule := range rules {
		s.rulesByID[rule.ID] = rule
	}
}

// currentLicence returns the current resolved licence info (thread-safe).
func (s *Server) currentLicence() licence.Info {
	s.licenceMu.RLock()
	defer s.licenceMu.RUnlock()

	return s.licenceInfo
}

// reloadLicence re-validates the licence file (after activation / trial start).
func (s *Server) reloadLicence() licence.Info {
	info := licence.Validate(s.licencePath)

	s.licenceMu.Lock()
	s.licenceInfo = info
	s.licenceMu.Unlock()

	return info
}

// requirePaid writes a 402 and returns false if the current licence is Free
// tier. Pro-only endpoints call this first. A licence in its grace window
// still counts as paid.
func (s *Server) requirePaid(w http.ResponseWriter) bool {
	if s.currentLicence().Tier == licence.TierFree {
		http.Error(w, `{"error":"This feature requires a Pro licence","code":"upgrade_required"}`,
			http.StatusPaymentRequired)

		return false
	}

	return true
}

// enrichGuidedFix re-populates each finding's guided-fix fields from the
// current rule set. These aren't persisted in the findings table (findings
// are resource snapshots; guided-fix availability is a property of the rule),
// so they're derived here whenever findings are read back from the database.
func (s *Server) enrichGuidedFix(findings []clusterdoctor.Finding) []clusterdoctor.Finding {
	for i := range findings {
		rule, ok := s.rulesByID[findings[i].RuleID]
		if !ok || rule.GuidedFix.Action == "" {
			continue
		}

		findings[i].GuidedFixAvailable = true
		findings[i].GuidedFixAction = rule.GuidedFix.Action
		findings[i].GuidedFixWarning = rule.GuidedFix.Warning
	}

	return findings
}

// enrichSuppressions marks each finding's Suppressed/Comment state from the
// per-cluster suppressions table. Suppression state is keyed by resource
// identity (not the per-scan finding UUID), so it's derived here whenever
// findings are read back rather than stored on the finding row. Lookup errors
// are logged and swallowed — findings are still returned, just un-enriched.
func (s *Server) enrichSuppressions(ctx context.Context, cluster string, findings []clusterdoctor.Finding) []clusterdoctor.Finding {
	if cluster == "" {
		return findings
	}

	keys, err := cddb.GetSuppressionKeys(ctx, s.db, cluster)
	if err != nil {
		logger.Log(logger.LevelError, map[string]string{"cluster": cluster}, err,
			"cluster-doctor: loading suppression keys")

		return findings
	}

	comments, err := cddb.GetComments(ctx, s.db, cluster)
	if err != nil {
		logger.Log(logger.LevelError, map[string]string{"cluster": cluster}, err,
			"cluster-doctor: loading finding comments")

		return findings
	}

	for i := range findings {
		key := cddb.SuppressionKey(
			findings[i].RuleID, findings[i].Namespace,
			findings[i].ResourceKind, findings[i].ResourceName,
		)
		if keys[key] {
			findings[i].Suppressed = true
		}

		if comment, ok := comments[key]; ok {
			findings[i].Comment = comment
		}
	}

	return findings
}

func (s *Server) registerActive(scanID string, live *liveScan) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active[scanID] = live
}

func (s *Server) lookupActive(scanID string) (*liveScan, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	live, ok := s.active[scanID]

	return live, ok
}

// runScan executes the scan in the background: it relays progress events to
// live as they happen and persists the final result once Run returns.
func (s *Server) runScan(clientset kubernetes.Interface, cluster, scanID string, live *liveScan) {
	// Honor this cluster's per-rule overrides by scanning only the rules that
	// aren't disabled. If the lookup fails we log and fall back to the full set
	// rather than aborting the scan — a missing override read shouldn't cost the
	// user their diagnostics.
	rules := s.rules

	disabled, err := cddb.GetDisabledRuleIDs(context.Background(), s.db, cluster)
	if err != nil {
		logger.Log(logger.LevelError, map[string]string{"scanId": scanID, "cluster": cluster}, err,
			"cluster-doctor: loading disabled rules, scanning full rule set")
	} else if len(disabled) > 0 {
		filtered := make([]clusterdoctor.Rule, 0, len(s.rules))
		for _, rule := range s.rules {
			if !disabled[rule.ID] {
				filtered = append(filtered, rule)
			}
		}

		rules = filtered
	}

	scanner := clusterdoctor.NewScanner(rules)
	progress := make(chan clusterdoctor.ScanProgressEvent, 64) //nolint:mnd

	relayDone := make(chan struct{})

	go func() {
		for ev := range progress {
			live.broadcast(ev)
		}

		live.finish()
		close(relayDone)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), scanTimeout)
	defer cancel()

	result := scanner.Run(ctx, clientset, cluster, scanID, progress)

	<-relayDone

	if err := cddb.SaveScan(context.Background(), s.db, result); err != nil {
		logger.Log(logger.LevelError, map[string]string{"scanId": scanID, "cluster": cluster}, err,
			"cluster-doctor: saving scan result")
	}
}
