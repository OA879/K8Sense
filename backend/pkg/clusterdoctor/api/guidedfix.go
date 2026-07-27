package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"github.com/OA879/K8Sense/backend/pkg/clusterdoctor"
	cddb "github.com/OA879/K8Sense/backend/pkg/clusterdoctor/db"
	"github.com/OA879/K8Sense/backend/pkg/logger"
)

// guidedFixRequest is the body of POST /cluster-doctor/guided-fix. Confirmed
// MUST be true — the frontend only sets it after the user clicks through the
// confirmation modal, so a request without it is rejected. This is the
// "explicit human intent" gate from K8SENSE_CONTEXT.md.
type guidedFixRequest struct {
	Cluster      string `json:"cluster"`
	Action       string `json:"action"`
	Namespace    string `json:"namespace"`
	ResourceName string `json:"resourceName"`
	Confirmed    bool   `json:"confirmed"`
	Force        bool   `json:"force,omitempty"`     // delete_pod: --grace-period=0
	Replicas     *int32 `json:"replicas,omitempty"`  // scale_deployment target
}

type guidedFixResponse struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

// allowedGuidedFixActions is the exhaustive allowlist. Anything not here is
// guide-only and must never be executed automatically (etcd, drain, RBAC,
// cert rotation, NetworkPolicy, PVC/PV deletion, Secret/ConfigMap edits, or
// anything on the control plane).
var allowedGuidedFixActions = map[string]bool{
	"delete_pod":         true,
	"delete_job":         true,
	"uncordon_node":      true,
	"cordon_node":        true, // inverse of uncordon_node, used to revert it
	"scale_deployment":   true,
	"restart_deployment": true,
}

// revertInfo describes how to undo a guided fix: the inverse action and the
// prior state it needs. Empty Action means the fix is not reversible
// (delete_pod/job — the object is gone; restart — the rollout already happened).
type revertInfo struct {
	Action  string
	Payload string
}

// kindForAction maps a guided-fix action to the Kubernetes kind it targets, so
// the audit log records what sort of object was touched even though the request
// body doesn't carry a kind.
func kindForAction(action string) string {
	switch action {
	case "delete_pod":
		return "Pod"
	case "delete_job":
		return "Job"
	case "uncordon_node", "cordon_node":
		return "Node"
	case "scale_deployment", "restart_deployment":
		return "Deployment"
	default:
		return ""
	}
}

// GuidedFix handles POST /cluster-doctor/guided-fix. It executes one safe,
// pre-approved remediation action and writes an audit entry regardless of
// outcome.
func (s *Server) GuidedFix(w http.ResponseWriter, r *http.Request) {
	if !s.requirePaid(w) {
		return
	}

	if !s.requireRole(w, clusterdoctor.RoleOperator) {
		return
	}

	var req guidedFixRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
		return
	}

	if !req.Confirmed {
		http.Error(w, `{"error": "action must be explicitly confirmed"}`, http.StatusBadRequest)
		return
	}

	if !allowedGuidedFixActions[req.Action] {
		http.Error(w, `{"error": "action is not permitted as a guided fix"}`, http.StatusForbidden)
		return
	}

	clientset, err := s.getClient(r, req.Cluster)
	if err != nil {
		http.Error(w, `{"error": "cluster not found"}`, http.StatusNotFound)
		return
	}

	// Derive the real identity from the request rather than labelling every
	// action "operator" — a shared (web) deployment's audit log is worthless
	// if it cannot answer "who did this?".
	actor := actorFromRequest(r)

	message, revert, execErr := s.executeGuidedFix(r.Context(), clientset, req)

	entry := cddb.AuditEntry{
		Actor:        actor,
		Action:       req.Action,
		ClusterID:    req.Cluster,
		Namespace:    req.Namespace,
		ResourceKind: kindForAction(req.Action),
		ResourceName: req.ResourceName,
		Payload:      guidedFixPayloadJSON(req),
		PerformedAt:  time.Now().UTC().Unix(),
	}

	if execErr != nil {
		entry.Result = "failed"
		entry.Error = execErr.Error()
	} else {
		entry.Result = "success"
		entry.RevertAction = revert.Action
		entry.RevertPayload = revert.Payload
	}

	if auditErr := cddb.WriteAudit(context.Background(), s.db, entry); auditErr != nil {
		logger.Log(logger.LevelError, map[string]string{"cluster": req.Cluster}, auditErr,
			"cluster-doctor: writing guided-fix audit entry")
	}

	if execErr != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(guidedFixResponse{Result: "failed", Message: execErr.Error()})

		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(guidedFixResponse{Result: "success", Message: message})
}

// executeGuidedFix dispatches to the concrete Kubernetes API call for the
// requested action. It returns a human message and, for reversible actions,
// the revertInfo needed to undo it.
func (s *Server) executeGuidedFix(
	ctx context.Context,
	clientset kubernetes.Interface,
	req guidedFixRequest,
) (string, revertInfo, error) {
	switch req.Action {
	case "delete_pod":
		opts := metav1.DeleteOptions{}
		if req.Force {
			grace := int64(0)
			opts.GracePeriodSeconds = &grace
		}

		if err := clientset.CoreV1().Pods(req.Namespace).Delete(ctx, req.ResourceName, opts); err != nil {
			return "", revertInfo{}, err
		}

		return fmt.Sprintf("Pod %s/%s deleted", req.Namespace, req.ResourceName), revertInfo{}, nil

	case "delete_job":
		policy := metav1.DeletePropagationBackground
		if err := clientset.BatchV1().Jobs(req.Namespace).Delete(
			ctx, req.ResourceName, metav1.DeleteOptions{PropagationPolicy: &policy},
		); err != nil {
			return "", revertInfo{}, err
		}

		return fmt.Sprintf("Job %s/%s deleted", req.Namespace, req.ResourceName), revertInfo{}, nil

	case "uncordon_node":
		if err := s.patchNodeSchedulable(ctx, clientset, req.ResourceName, false); err != nil {
			return "", revertInfo{}, err
		}
		// Undo = cordon the node again.
		return fmt.Sprintf("Node %s uncordoned", req.ResourceName), revertInfo{Action: "cordon_node"}, nil

	case "cordon_node":
		if err := s.patchNodeSchedulable(ctx, clientset, req.ResourceName, true); err != nil {
			return "", revertInfo{}, err
		}

		return fmt.Sprintf("Node %s cordoned", req.ResourceName), revertInfo{}, nil

	case "scale_deployment":
		if req.Replicas == nil {
			return "", revertInfo{}, fmt.Errorf("replicas is required for scale_deployment")
		}

		// Capture the current replica count first so the scale can be reverted.
		prior := int32(1)
		if cur, err := clientset.AppsV1().Deployments(req.Namespace).Get(ctx, req.ResourceName, metav1.GetOptions{}); err == nil && cur.Spec.Replicas != nil {
			prior = *cur.Spec.Replicas
		}

		patch := []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, *req.Replicas))
		if _, err := clientset.AppsV1().Deployments(req.Namespace).Patch(
			ctx, req.ResourceName, types.StrategicMergePatchType, patch, metav1.PatchOptions{},
		); err != nil {
			return "", revertInfo{}, err
		}

		return fmt.Sprintf("Deployment %s/%s scaled to %d replicas", req.Namespace, req.ResourceName, *req.Replicas),
			revertInfo{Action: "scale_deployment", Payload: fmt.Sprintf(`{"replicas":%d}`, prior)}, nil

	case "restart_deployment":
		// Same mechanism as `kubectl rollout restart`: stamp a template
		// annotation so the Deployment rolls its pods. Not reversible.
		patch := []byte(fmt.Sprintf(
			`{"spec":{"template":{"metadata":{"annotations":{"k8sense.io/restartedAt":%q}}}}}`,
			time.Now().UTC().Format(time.RFC3339),
		))
		if _, err := clientset.AppsV1().Deployments(req.Namespace).Patch(
			ctx, req.ResourceName, types.StrategicMergePatchType, patch, metav1.PatchOptions{},
		); err != nil {
			return "", revertInfo{}, err
		}

		return fmt.Sprintf("Deployment %s/%s rollout restarted", req.Namespace, req.ResourceName), revertInfo{}, nil

	default:
		return "", revertInfo{}, fmt.Errorf("unsupported action %q", req.Action)
	}
}

// patchNodeSchedulable cordons (unschedulable=true) or uncordons a node.
func (s *Server) patchNodeSchedulable(
	ctx context.Context, clientset kubernetes.Interface, node string, unschedulable bool,
) error {
	patch := []byte(fmt.Sprintf(`{"spec":{"unschedulable":%t}}`, unschedulable))
	_, err := clientset.CoreV1().Nodes().Patch(
		ctx, node, types.StrategicMergePatchType, patch, metav1.PatchOptions{},
	)

	return err
}

func guidedFixPayloadJSON(req guidedFixRequest) string {
	data, err := json.Marshal(req)
	if err != nil {
		return ""
	}

	return string(data)
}

// revertRequest is the body of POST /cluster-doctor/guided-fix/revert.
type revertRequest struct {
	AuditID   string `json:"auditId"`
	Confirmed bool   `json:"confirmed"`
}

// RevertGuidedFix undoes a previously-applied, reversible guided fix (scale or
// uncordon) using the prior state captured when the fix ran. It writes its own
// audit entry and stamps the original as reverted so it can't be undone twice.
func (s *Server) RevertGuidedFix(w http.ResponseWriter, r *http.Request) {
	if !s.requirePaid(w) {
		return
	}

	if !s.requireRole(w, clusterdoctor.RoleOperator) {
		return
	}

	var req revertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
		return
	}

	if !req.Confirmed {
		http.Error(w, `{"error": "revert must be explicitly confirmed"}`, http.StatusBadRequest)
		return
	}

	orig, err := cddb.GetAudit(r.Context(), s.db, req.AuditID)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, `{"error": "audit entry not found"}`, http.StatusNotFound)
		return
	}

	if err != nil {
		http.Error(w, `{"error": "could not load audit entry"}`, http.StatusInternalServerError)
		return
	}

	if orig.Result != "success" || orig.RevertAction == "" {
		http.Error(w, `{"error": "this action cannot be reverted"}`, http.StatusUnprocessableEntity)
		return
	}

	if orig.RevertedAt != nil {
		http.Error(w, `{"error": "this action has already been reverted"}`, http.StatusConflict)
		return
	}

	clientset, err := s.getClient(r, orig.ClusterID)
	if err != nil {
		http.Error(w, `{"error": "cluster not found"}`, http.StatusNotFound)
		return
	}

	revReq := guidedFixRequest{
		Cluster:      orig.ClusterID,
		Action:       orig.RevertAction,
		Namespace:    orig.Namespace,
		ResourceName: orig.ResourceName,
		Confirmed:    true,
	}

	// scale_deployment reverts need the prior replica count from revert_payload.
	if orig.RevertAction == "scale_deployment" {
		var p struct {
			Replicas int32 `json:"replicas"`
		}
		if jsonErr := json.Unmarshal([]byte(orig.RevertPayload), &p); jsonErr == nil {
			revReq.Replicas = &p.Replicas
		}
	}

	message, _, execErr := s.executeGuidedFix(r.Context(), clientset, revReq)

	entry := cddb.AuditEntry{
		Actor:        actorFromRequest(r),
		Action:       "revert_" + orig.Action,
		ClusterID:    orig.ClusterID,
		Namespace:    orig.Namespace,
		ResourceKind: orig.ResourceKind,
		ResourceName: orig.ResourceName,
		RevertOf:     orig.ID,
		PerformedAt:  time.Now().UTC().Unix(),
	}

	if execErr != nil {
		entry.Result = "failed"
		entry.Error = execErr.Error()
	} else {
		entry.Result = "success"
	}

	if auditErr := cddb.WriteAudit(context.Background(), s.db, entry); auditErr != nil {
		logger.Log(logger.LevelError, map[string]string{"cluster": orig.ClusterID}, auditErr,
			"cluster-doctor: writing revert audit entry")
	}

	if execErr != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(guidedFixResponse{Result: "failed", Message: execErr.Error()})

		return
	}

	// Only mark the original reverted once the inverse action actually succeeded.
	if markErr := cddb.MarkReverted(context.Background(), s.db, orig.ID, time.Now().UTC().Unix()); markErr != nil {
		logger.Log(logger.LevelError, map[string]string{"cluster": orig.ClusterID}, markErr,
			"cluster-doctor: marking audit entry reverted")
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(guidedFixResponse{Result: "success", Message: message})
}
