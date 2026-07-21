package main

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gorilla/mux"
	"k8s.io/client-go/kubernetes"

	"github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor"
	cdapi "github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor/api"
	_ "github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor/checks" // registers check_fn implementations
	cddb "github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor/db"
	"github.com/kubernetes-sigs/headlamp/backend/pkg/logger"
)

// resolveRulesDir finds the rules/ directory: an explicit override, then the
// current working directory (true when running `npm run start` from the
// repo root during development), then next to the running executable
// (true for a packaged install).
func resolveRulesDir() string {
	if dir := os.Getenv("K8SENSE_RULES_DIR"); dir != "" {
		return dir
	}

	if _, err := os.Stat("rules"); err == nil {
		return "rules"
	}

	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "rules")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return "rules"
}

// setupClusterDoctor loads the rule library, opens the local SQLite store,
// and registers the /cluster-doctor/* routes on r. It's K8sense's own
// addition on top of the forked Headlamp base — kept in its own file and
// its own pkg/clusterdoctor tree so it stays easy to tell apart from
// upstream code. A failure here (bad rules, can't open the db) disables the
// diagnostics engine but must never take down the rest of the app.
func setupClusterDoctor(r *mux.Router, config *HeadlampConfig) {
	rules, err := clusterdoctor.LoadRules(resolveRulesDir())
	if err != nil {
		logger.Log(logger.LevelError, nil, err, "cluster-doctor: loading rules, diagnostics engine disabled")
		return
	}

	dbPath, err := cddb.DefaultPath()
	if err != nil {
		logger.Log(logger.LevelError, nil, err, "cluster-doctor: resolving database path, diagnostics engine disabled")
		return
	}

	database, err := cddb.Open(dbPath)
	if err != nil {
		logger.Log(logger.LevelError, map[string]string{"path": dbPath}, err,
			"cluster-doctor: opening database, diagnostics engine disabled")

		return
	}

	// Load previously-imported custom rules and merge them into the built-in
	// set so they're active from startup.
	if customRules, err := loadCustomRules(database); err != nil {
		logger.Log(logger.LevelError, nil, err, "cluster-doctor: loading custom rules")
	} else {
		rules = append(rules, customRules...)
	}

	getClient := func(req *http.Request, clusterName string) (kubernetes.Interface, error) {
		ctxtProxy, err := config.KubeConfigStore.GetContext(clusterName)
		if err != nil {
			return nil, err
		}

		token := config.requestTokenForContext(req, clusterName, ctxtProxy)

		return ctxtProxy.ClientSetWithToken(token)
	}

	licencePath := filepath.Join(filepath.Dir(dbPath), "licence.k8sense-licence")
	cdServer := cdapi.NewServer(database, rules, getClient, licencePath)

	// Enforce Free-tier retention on startup (keep the newest 10 scans per
	// cluster). Pro/time-based retention is applied via the Settings purge UI.
	if pruned, err := cddb.PruneScans(context.Background(), database, freeRetentionScans); err != nil {
		logger.Log(logger.LevelError, nil, err, "cluster-doctor: pruning old scans")
	} else if pruned > 0 {
		logger.Log(logger.LevelInfo, map[string]string{"pruned": strconv.Itoa(pruned)}, nil,
			"cluster-doctor: pruned old scans on startup")
	}

	r.HandleFunc("/cluster-doctor/scan", cdServer.StartScan).Methods("POST")
	r.HandleFunc("/cluster-doctor/scan/multi", cdServer.StartMultiScan).Methods("POST")
	r.HandleFunc("/cluster-doctor/scan/{id}/status", cdServer.ScanStatus).Methods("GET")
	r.HandleFunc("/cluster-doctor/findings/{scanId}", cdServer.GetFindings).Methods("GET")
	r.HandleFunc("/cluster-doctor/findings/{scanId}/export", cdServer.ExportReport).Methods("GET")
	r.HandleFunc("/cluster-doctor/findings/{scanId}/diff/{prevId}", cdServer.ScanDiff).Methods("GET")
	r.HandleFunc("/cluster-doctor/history", cdServer.ListHistory).Methods("GET")
	r.HandleFunc("/cluster-doctor/rules", cdServer.ListRulesForCluster).Methods("GET")
	r.HandleFunc("/cluster-doctor/rules/validate", cdServer.ValidateRule).Methods("POST")
	r.HandleFunc("/cluster-doctor/rules/import", cdServer.ImportRule).Methods("POST")
	r.HandleFunc("/cluster-doctor/rules/custom", cdServer.ListCustomRules).Methods("GET")
	r.HandleFunc("/cluster-doctor/rules/custom/{id}", cdServer.DeleteCustomRule).Methods("DELETE")
	r.HandleFunc("/cluster-doctor/rules/{id}/toggle", cdServer.ToggleRule).Methods("PUT")
	r.HandleFunc("/cluster-doctor/rules/{id}/severity", cdServer.SetRuleSeverity).Methods("PUT")
	r.HandleFunc("/cluster-doctor/guided-fix", cdServer.GuidedFix).Methods("POST")
	r.HandleFunc("/cluster-doctor/findings/suppress", cdServer.SuppressFinding).Methods("POST")
	r.HandleFunc("/cluster-doctor/findings/unsuppress", cdServer.UnsuppressFinding).Methods("POST")
	r.HandleFunc("/cluster-doctor/findings/comment", cdServer.CommentFinding).Methods("PUT")
	r.HandleFunc("/cluster-doctor/audit-log", cdServer.ListAuditLog).Methods("GET")
	r.HandleFunc("/cluster-doctor/audit-log/export", cdServer.ExportAuditLog).Methods("GET")
	r.HandleFunc("/cluster-doctor/clusters/test", cdServer.TestConnection).Methods("GET")
	r.HandleFunc("/cluster-doctor/storage", cdServer.GetStorageStats).Methods("GET")
	r.HandleFunc("/cluster-doctor/storage/purge", cdServer.PurgeScans).Methods("POST")
	r.HandleFunc("/cluster-doctor/licence", cdServer.GetLicence).Methods("GET")
	r.HandleFunc("/cluster-doctor/licence/activate", cdServer.ActivateLicence).Methods("POST")
	r.HandleFunc("/cluster-doctor/licence/trial", cdServer.StartTrial).Methods("POST")

	logger.Log(logger.LevelInfo, map[string]string{"rulesLoaded": strconv.Itoa(len(rules)), "dbPath": dbPath}, nil,
		"cluster-doctor: diagnostics engine ready")
}

// freeRetentionScans is how many scans per cluster the Free tier keeps.
const freeRetentionScans = 10

// loadCustomRules reads persisted custom rules from the DB and parses each
// back into rule structs to merge into the active set.
func loadCustomRules(database *sql.DB) ([]clusterdoctor.Rule, error) {
	stored, err := cddb.ListCustomRules(context.Background(), database)
	if err != nil {
		return nil, err
	}

	var rules []clusterdoctor.Rule

	for _, cr := range stored {
		parsed, err := clusterdoctor.ParseRules([]byte(cr.YAML))
		if err != nil {
			logger.Log(logger.LevelError, map[string]string{"ruleId": cr.ID}, err,
				"cluster-doctor: skipping invalid custom rule")

			continue
		}

		rules = append(rules, parsed...)
	}

	return rules, nil
}
