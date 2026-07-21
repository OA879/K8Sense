package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor"
	cddb "github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor/db"
)

// suppressRequest is the body of the suppress/unsuppress/comment endpoints. A
// suppression is keyed by resource identity rather than the per-scan finding
// UUID, because that UUID isn't stable across scans.
type suppressRequest struct {
	Cluster      string `json:"cluster"`
	RuleID       string `json:"ruleId"`
	Namespace    string `json:"namespace"`
	ResourceKind string `json:"resourceKind"`
	ResourceName string `json:"resourceName"`
	Reason       string `json:"reason,omitempty"`
	Comment      string `json:"comment,omitempty"`
	By           string `json:"by,omitempty"`
}

func writeOK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"result": "ok"})
}

// SuppressFinding handles POST /cluster-doctor/findings/suppress. It mutes a
// finding for a resource across scans; reason is required.
func (s *Server) SuppressFinding(w http.ResponseWriter, r *http.Request) {
	if !s.requireRole(w, clusterdoctor.RoleOperator) {
		return
	}

	var req suppressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Cluster == "" || req.RuleID == "" || req.ResourceName == "" {
		http.Error(w, `{"error": "cluster, ruleId and resourceName are required"}`, http.StatusBadRequest)
		return
	}

	if req.Reason == "" {
		http.Error(w, `{"error": "reason is required"}`, http.StatusBadRequest)
		return
	}

	by := req.By
	if by == "" {
		by = r.Header.Get("X-K8sense-Actor")
	}

	err := cddb.AddSuppression(r.Context(), s.db, cddb.Suppression{
		ClusterID:    req.Cluster,
		RuleID:       req.RuleID,
		Namespace:    req.Namespace,
		ResourceKind: req.ResourceKind,
		ResourceName: req.ResourceName,
		Reason:       req.Reason,
		SuppressedBy: by,
		SuppressedAt: time.Now().UTC().Unix(),
		Comment:      req.Comment,
	})
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	writeOK(w)
}

// UnsuppressFinding handles POST /cluster-doctor/findings/unsuppress. It
// removes a resource's suppression by primary key.
func (s *Server) UnsuppressFinding(w http.ResponseWriter, r *http.Request) {
	if !s.requireRole(w, clusterdoctor.RoleOperator) {
		return
	}

	var req suppressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Cluster == "" || req.RuleID == "" || req.ResourceName == "" {
		http.Error(w, `{"error": "cluster, ruleId and resourceName are required"}`, http.StatusBadRequest)
		return
	}

	err := cddb.RemoveSuppression(
		r.Context(), s.db,
		req.Cluster, req.RuleID, req.Namespace, req.ResourceKind, req.ResourceName,
	)
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	writeOK(w)
}

// CommentFinding handles PUT /cluster-doctor/findings/comment. It attaches a
// note to a resource without necessarily muting it.
func (s *Server) CommentFinding(w http.ResponseWriter, r *http.Request) {
	if !s.requireRole(w, clusterdoctor.RoleOperator) {
		return
	}

	var req suppressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Cluster == "" || req.RuleID == "" || req.ResourceName == "" {
		http.Error(w, `{"error": "cluster, ruleId and resourceName are required"}`, http.StatusBadRequest)
		return
	}

	err := cddb.SetComment(
		r.Context(), s.db,
		req.Cluster, req.RuleID, req.Namespace, req.ResourceKind, req.ResourceName, req.Comment,
	)
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	writeOK(w)
}
