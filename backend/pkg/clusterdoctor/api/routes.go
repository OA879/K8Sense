package api

import "github.com/gorilla/mux"

// RegisterRoutes wires every /cluster-doctor/* endpoint onto r. It lives here
// rather than in cmd so that handler tests exercise the exact same routing
// table the binary serves — no risk of the test router drifting from the real
// one as endpoints are added.
func (s *Server) RegisterRoutes(r *mux.Router) {
	// Record every cluster-doctor mutation/export in the audit log. The
	// middleware self-scopes to /cluster-doctor paths, so sharing Headlamp's
	// router is safe (see auditMiddleware).
	r.Use(s.auditMiddleware)

	r.HandleFunc("/cluster-doctor/scan", s.StartScan).Methods("POST")
	r.HandleFunc("/cluster-doctor/scan/multi", s.StartMultiScan).Methods("POST")
	r.HandleFunc("/cluster-doctor/scan/{id}/status", s.ScanStatus).Methods("GET")
	r.HandleFunc("/cluster-doctor/findings/{scanId}", s.GetFindings).Methods("GET")
	r.HandleFunc("/cluster-doctor/findings/{scanId}/export", s.ExportReport).Methods("GET")
	r.HandleFunc("/cluster-doctor/findings/{scanId}/diff/{prevId}", s.ScanDiff).Methods("GET")
	r.HandleFunc("/cluster-doctor/history", s.ListHistory).Methods("GET")
	r.HandleFunc("/cluster-doctor/rules", s.ListRulesForCluster).Methods("GET")
	r.HandleFunc("/cluster-doctor/rules/validate", s.ValidateRule).Methods("POST")
	r.HandleFunc("/cluster-doctor/rules/import", s.ImportRule).Methods("POST")
	r.HandleFunc("/cluster-doctor/rules/custom", s.ListCustomRules).Methods("GET")
	r.HandleFunc("/cluster-doctor/rules/custom/{id}", s.DeleteCustomRule).Methods("DELETE")
	r.HandleFunc("/cluster-doctor/rules/{id}/toggle", s.ToggleRule).Methods("PUT")
	r.HandleFunc("/cluster-doctor/rules/{id}/severity", s.SetRuleSeverity).Methods("PUT")
	r.HandleFunc("/cluster-doctor/network-map", s.NetworkMap).Methods("GET")
	r.HandleFunc("/cluster-doctor/timeline", s.Timeline).Methods("GET")
	r.HandleFunc("/cluster-doctor/cost", s.Cost).Methods("GET")
	r.HandleFunc("/cluster-doctor/upgrade", s.UpgradeReadiness).Methods("GET")
	r.HandleFunc("/cluster-doctor/compliance", s.Compliance).Methods("GET")
	r.HandleFunc("/cluster-doctor/ai/status", s.AIStatus).Methods("GET")
	r.HandleFunc("/cluster-doctor/ai/chat", s.AIChat).Methods("POST")
	r.HandleFunc("/cluster-doctor/ai/config", s.GetAIConfig).Methods("GET")
	r.HandleFunc("/cluster-doctor/ai/config", s.SetAIConfig).Methods("PUT")
	r.HandleFunc("/cluster-doctor/catalog", s.Catalog).Methods("GET")
	r.HandleFunc("/cluster-doctor/catalog/install", s.InstallApp).Methods("POST")
	r.HandleFunc("/cluster-doctor/catalog/uninstall", s.UninstallApp).Methods("POST")
	r.HandleFunc("/cluster-doctor/runbooks", s.ListRunbooks).Methods("GET")
	r.HandleFunc("/cluster-doctor/runbooks/status", s.RunbooksStatus).Methods("GET")
	r.HandleFunc("/cluster-doctor/runbooks/enable", s.EnableRunbooks).Methods("POST")
	r.HandleFunc("/cluster-doctor/runbooks/run", s.RunRunbook).Methods("POST")
	r.HandleFunc("/cluster-doctor/runbooks/run/{runId}/logs", s.RunbookLogs).Methods("GET")
	r.HandleFunc("/cluster-doctor/runbooks/config", s.SetRunbooksConfig).Methods("PUT")
	r.HandleFunc("/cluster-doctor/vulnscan", s.RunVulnScan).Methods("POST")
	r.HandleFunc("/cluster-doctor/vulnscan/config", s.VulnScanConfig).Methods("GET")
	r.HandleFunc("/cluster-doctor/vulnscan/config", s.SetVulnScanConfig).Methods("PUT")
	r.HandleFunc("/cluster-doctor/vulnscan/{runId}", s.VulnScanStatus).Methods("GET")
	r.HandleFunc("/cluster-doctor/guided-fix", s.GuidedFix).Methods("POST")
	r.HandleFunc("/cluster-doctor/guided-fix/revert", s.RevertGuidedFix).Methods("POST")
	r.HandleFunc("/cluster-doctor/findings/suppress", s.SuppressFinding).Methods("POST")
	r.HandleFunc("/cluster-doctor/findings/unsuppress", s.UnsuppressFinding).Methods("POST")
	r.HandleFunc("/cluster-doctor/findings/comment", s.CommentFinding).Methods("PUT")
	r.HandleFunc("/cluster-doctor/audit-log", s.ListAuditLog).Methods("GET")
	r.HandleFunc("/cluster-doctor/notifications", s.GetNotifyConfig).Methods("GET")
	r.HandleFunc("/cluster-doctor/notifications", s.SetNotifyConfig).Methods("PUT")
	r.HandleFunc("/cluster-doctor/notifications/test", s.TestNotification).Methods("POST")
	r.HandleFunc("/cluster-doctor/branding", s.GetBranding).Methods("GET")
	r.HandleFunc("/cluster-doctor/branding", s.SetBranding).Methods("PUT")
	r.HandleFunc("/cluster-doctor/role", s.GetRole).Methods("GET")
	r.HandleFunc("/cluster-doctor/role", s.SetRole).Methods("PUT")
	r.HandleFunc("/cluster-doctor/audit-log/export", s.ExportAuditLog).Methods("GET")
	r.HandleFunc("/cluster-doctor/clusters/test", s.TestConnection).Methods("GET")
	r.HandleFunc("/cluster-doctor/storage", s.GetStorageStats).Methods("GET")
	r.HandleFunc("/cluster-doctor/storage/purge", s.PurgeScans).Methods("POST")
	r.HandleFunc("/cluster-doctor/licence", s.GetLicence).Methods("GET")
	r.HandleFunc("/cluster-doctor/licence/activate", s.ActivateLicence).Methods("POST")
	r.HandleFunc("/cluster-doctor/licence/trial", s.StartTrial).Methods("POST")
}
