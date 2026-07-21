package main

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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
func resolveRulesDir() (string, []string) {
	var searched []string

	if dir := os.Getenv("K8SENSE_RULES_DIR"); dir != "" {
		return dir, nil
	}

	searched = append(searched, "rules")
	if _, err := os.Stat("rules"); err == nil {
		return "rules", nil
	}

	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "rules")
		searched = append(searched, candidate)

		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "rules", searched
}

// setupClusterDoctor loads the rule library, opens the local SQLite store,
// and registers the /cluster-doctor/* routes on r. It's K8sense's own
// addition on top of the forked Headlamp base — kept in its own file and
// its own pkg/clusterdoctor tree so it stays easy to tell apart from
// upstream code. A failure here (bad rules, can't open the db) disables the
// diagnostics engine but must never take down the rest of the app.
func setupClusterDoctor(r *mux.Router, config *HeadlampConfig) {
	rulesDir, searched := resolveRulesDir()

	rules, err := clusterdoctor.LoadRules(rulesDir)
	if err != nil {
		// Cluster Doctor is the product's core feature, so a missing rule
		// library is not a quiet degradation — say exactly where we looked.
		// Common cause: a container image that didn't ship the rules/ directory.
		logger.Log(logger.LevelError, map[string]string{
			"rulesDir": rulesDir,
			"searched": strings.Join(searched, ", "),
			"hint":     "set K8SENSE_RULES_DIR or ensure rules/ ships with the build",
		}, err, "cluster-doctor: RULE LIBRARY NOT FOUND — diagnostics engine is DISABLED")

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

	// Scheduled scans have no inbound request to derive a token from, so they
	// resolve clients straight from the kubeconfig store instead of getClient.
	scheduledClient := func(clusterName string) (kubernetes.Interface, error) {
		ctxtProxy, err := config.KubeConfigStore.GetContext(clusterName)
		if err != nil {
			return nil, err
		}

		return ctxtProxy.ClientSetWithToken("")
	}

	cdServer.StartScheduler(context.Background(), scheduledClient)

	// Enforce Free-tier retention on startup (keep the newest 10 scans per
	// cluster). Pro/time-based retention is applied via the Settings purge UI.
	if pruned, err := cddb.PruneScans(context.Background(), database, freeRetentionScans); err != nil {
		logger.Log(logger.LevelError, nil, err, "cluster-doctor: pruning old scans")
	} else if pruned > 0 {
		logger.Log(logger.LevelInfo, map[string]string{"pruned": strconv.Itoa(pruned)}, nil,
			"cluster-doctor: pruned old scans on startup")
	}

	cdServer.RegisterRoutes(r)

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
