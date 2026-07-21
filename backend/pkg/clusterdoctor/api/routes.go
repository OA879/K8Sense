package api

import "github.com/gorilla/mux"

// RegisterRoutes wires every /cluster-doctor/* endpoint onto r. It lives here
// rather than in cmd so that handler tests exercise the exact same routing
// table the binary serves — no risk of the test router drifting from the real
// one as endpoints are added.
func (s *Server) RegisterRoutes(r *mux.Router) {
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
	r.HandleFunc("/cluster-doctor/guided-fix", s.GuidedFix).Methods("POST")
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
