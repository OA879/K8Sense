package main

import (
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

	getClient := func(req *http.Request, clusterName string) (kubernetes.Interface, error) {
		ctxtProxy, err := config.KubeConfigStore.GetContext(clusterName)
		if err != nil {
			return nil, err
		}

		token := config.requestTokenForContext(req, clusterName, ctxtProxy)

		return ctxtProxy.ClientSetWithToken(token)
	}

	cdServer := cdapi.NewServer(database, rules, getClient)

	r.HandleFunc("/cluster-doctor/scan", cdServer.StartScan).Methods("POST")
	r.HandleFunc("/cluster-doctor/scan/{id}/status", cdServer.ScanStatus).Methods("GET")
	r.HandleFunc("/cluster-doctor/findings/{scanId}", cdServer.GetFindings).Methods("GET")
	r.HandleFunc("/cluster-doctor/findings/{scanId}/export", cdServer.ExportReport).Methods("GET")
	r.HandleFunc("/cluster-doctor/findings/{scanId}/diff/{prevId}", cdServer.ScanDiff).Methods("GET")
	r.HandleFunc("/cluster-doctor/history", cdServer.ListHistory).Methods("GET")
	r.HandleFunc("/cluster-doctor/rules", cdServer.ListRulesForCluster).Methods("GET")
	r.HandleFunc("/cluster-doctor/rules/{id}/toggle", cdServer.ToggleRule).Methods("PUT")
	r.HandleFunc("/cluster-doctor/guided-fix", cdServer.GuidedFix).Methods("POST")
	r.HandleFunc("/cluster-doctor/audit-log", cdServer.ListAuditLog).Methods("GET")

	logger.Log(logger.LevelInfo, map[string]string{"rulesLoaded": strconv.Itoa(len(rules)), "dbPath": dbPath}, nil,
		"cluster-doctor: diagnostics engine ready")
}
