package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor"
	cddb "github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor/db"
	"github.com/kubernetes-sigs/headlamp/backend/pkg/logger"
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
	"scale_deployment":   true,
	"restart_deployment": true,
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

	actor := r.Header.Get("X-K8sense-Actor")
	if actor == "" {
		actor = "operator"
	}

	message, execErr := s.executeGuidedFix(r.Context(), clientset, req)

	entry := cddb.AuditEntry{
		Actor:        actor,
		Action:       req.Action,
		ClusterID:    req.Cluster,
		Namespace:    req.Namespace,
		ResourceName: req.ResourceName,
		Payload:      guidedFixPayloadJSON(req),
		PerformedAt:  time.Now().UTC().Unix(),
	}

	if execErr != nil {
		entry.Result = "failed"
		entry.Error = execErr.Error()
	} else {
		entry.Result = "success"
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
// requested action. Each branch is one narrow, reversible operation.
func (s *Server) executeGuidedFix(
	ctx context.Context,
	clientset kubernetes.Interface,
	req guidedFixRequest,
) (string, error) {
	switch req.Action {
	case "delete_pod":
		opts := metav1.DeleteOptions{}
		if req.Force {
			grace := int64(0)
			opts.GracePeriodSeconds = &grace
		}

		if err := clientset.CoreV1().Pods(req.Namespace).Delete(ctx, req.ResourceName, opts); err != nil {
			return "", err
		}

		return fmt.Sprintf("Pod %s/%s deleted", req.Namespace, req.ResourceName), nil

	case "delete_job":
		policy := metav1.DeletePropagationBackground
		if err := clientset.BatchV1().Jobs(req.Namespace).Delete(
			ctx, req.ResourceName, metav1.DeleteOptions{PropagationPolicy: &policy},
		); err != nil {
			return "", err
		}

		return fmt.Sprintf("Job %s/%s deleted", req.Namespace, req.ResourceName), nil

	case "uncordon_node":
		patch := []byte(`{"spec":{"unschedulable":false}}`)
		if _, err := clientset.CoreV1().Nodes().Patch(
			ctx, req.ResourceName, types.StrategicMergePatchType, patch, metav1.PatchOptions{},
		); err != nil {
			return "", err
		}

		return fmt.Sprintf("Node %s uncordoned", req.ResourceName), nil

	case "scale_deployment":
		if req.Replicas == nil {
			return "", fmt.Errorf("replicas is required for scale_deployment")
		}

		patch := []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, *req.Replicas))
		if _, err := clientset.AppsV1().Deployments(req.Namespace).Patch(
			ctx, req.ResourceName, types.StrategicMergePatchType, patch, metav1.PatchOptions{},
		); err != nil {
			return "", err
		}

		return fmt.Sprintf("Deployment %s/%s scaled to %d replicas", req.Namespace, req.ResourceName, *req.Replicas), nil

	case "restart_deployment":
		// Same mechanism as `kubectl rollout restart`: stamp a template
		// annotation so the Deployment rolls its pods.
		patch := []byte(fmt.Sprintf(
			`{"spec":{"template":{"metadata":{"annotations":{"k8sense.io/restartedAt":%q}}}}}`,
			time.Now().UTC().Format(time.RFC3339),
		))
		if _, err := clientset.AppsV1().Deployments(req.Namespace).Patch(
			ctx, req.ResourceName, types.StrategicMergePatchType, patch, metav1.PatchOptions{},
		); err != nil {
			return "", err
		}

		return fmt.Sprintf("Deployment %s/%s rollout restarted", req.Namespace, req.ResourceName), nil

	default:
		return "", fmt.Errorf("unsupported action %q", req.Action)
	}
}

func guidedFixPayloadJSON(req guidedFixRequest) string {
	data, err := json.Marshal(req)
	if err != nil {
		return ""
	}

	return string(data)
}
