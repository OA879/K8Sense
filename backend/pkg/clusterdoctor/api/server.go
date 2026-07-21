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
}

// NewServer builds a Server. rules is the fully loaded rule set (built-in +
// custom, in a later phase); getClient resolves clusters by name.
func NewServer(database *sql.DB, rules []clusterdoctor.Rule, getClient ClientProvider) *Server {
	byID := make(map[string]clusterdoctor.Rule, len(rules))
	for _, rule := range rules {
		byID[rule.ID] = rule
	}

	return &Server{
		db:        database,
		rules:     rules,
		rulesByID: byID,
		getClient: getClient,
		active:    map[string]*liveScan{},
	}
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
	scanner := clusterdoctor.NewScanner(s.rules)
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
