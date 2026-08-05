package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/OA879/K8Sense/backend/pkg/logger"
)

// Runbooks: governed Ansible automation executed against the pointed cluster.
// Execution is remote-by-design — each run is a short-lived Kubernetes Job using
// an ansible-runner image, so nothing runs on the operator's laptop and the
// feature works identically on macOS, Linux and Windows. The playbook talks to
// the API via the in-cluster ServiceAccount, so what a runbook may do is capped
// by that SA's RBAC (governance, not a limitation). Every run is audited via the
// /cluster-doctor audit middleware.

const (
	runbookNamespace   = "k8sense-runbooks"
	runnerSAName       = "k8sense-runner"
	runnerRoleName     = "k8sense-runner"
	defaultRunnerImage = "quay.io/ansible/ansible-runner:latest"
	runbookTTLSeconds  = 3600
)

// runbookVar describes one form input for a template.
type runbookVar struct {
	Name     string `json:"name"`
	Label    string `json:"label"`
	Required bool   `json:"required"`
	Default  string `json:"default,omitempty"`
}

// runbookTemplate is a curated playbook plus its input schema. The playbook body
// is server-only; the client sees presentation + the var form.
type runbookTemplate struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Category    string       `json:"category"`
	Icon        string       `json:"icon"`
	Vars        []runbookVar `json:"vars"`

	playbook string
}

// runbookTemplates is the starter library. Every playbook runs locally
// (connection: local) and drives the cluster through kubernetes.core, which
// auto-detects the in-cluster ServiceAccount. `run_stamp` is injected by the
// server on every run.
func runbookTemplates() []runbookTemplate {
	return []runbookTemplate{
		{
			ID: "namespace-onboarding", Name: "Onboard a namespace",
			Description: "Create a namespace with a resource quota and a default-deny NetworkPolicy — a governed, repeatable app onboarding.",
			Category:    "Provisioning", Icon: "mdi:cube-outline",
			Vars: []runbookVar{
				{Name: "target_namespace", Label: "Namespace", Required: true},
				{Name: "cpu_quota", Label: "CPU quota (cores)", Default: "8"},
				{Name: "mem_quota", Label: "Memory quota", Default: "16Gi"},
			},
			playbook: `- hosts: localhost
  connection: local
  gather_facts: false
  tasks:
    - name: Create namespace
      kubernetes.core.k8s:
        definition:
          apiVersion: v1
          kind: Namespace
          metadata:
            name: "{{ target_namespace }}"
    - name: Resource quota
      kubernetes.core.k8s:
        definition:
          apiVersion: v1
          kind: ResourceQuota
          metadata:
            name: quota
            namespace: "{{ target_namespace }}"
          spec:
            hard:
              requests.cpu: "{{ cpu_quota }}"
              requests.memory: "{{ mem_quota }}"
              limits.cpu: "{{ cpu_quota }}"
              limits.memory: "{{ mem_quota }}"
    - name: Default-deny NetworkPolicy
      kubernetes.core.k8s:
        definition:
          apiVersion: networking.k8s.io/v1
          kind: NetworkPolicy
          metadata:
            name: default-deny
            namespace: "{{ target_namespace }}"
          spec:
            podSelector: {}
            policyTypes: [Ingress, Egress]
`,
		},
		{
			ID: "default-deny-netpol", Name: "Apply default-deny NetworkPolicy",
			Description: "Add a default-deny (ingress + egress) NetworkPolicy to a namespace — remediates CIS 5.3.2.",
			Category:    "Security", Icon: "mdi:shield-lock-outline",
			Vars: []runbookVar{
				{Name: "target_namespace", Label: "Namespace", Required: true},
			},
			playbook: `- hosts: localhost
  connection: local
  gather_facts: false
  tasks:
    - name: Default-deny NetworkPolicy
      kubernetes.core.k8s:
        definition:
          apiVersion: networking.k8s.io/v1
          kind: NetworkPolicy
          metadata:
            name: default-deny
            namespace: "{{ target_namespace }}"
          spec:
            podSelector: {}
            policyTypes: [Ingress, Egress]
`,
		},
		{
			ID: "rollout-restart", Name: "Rollout restart a deployment",
			Description: "Trigger a rolling restart of a deployment (like kubectl rollout restart) by stamping its pod template.",
			Category:    "Operations", Icon: "mdi:restart",
			Vars: []runbookVar{
				{Name: "target_namespace", Label: "Namespace", Required: true},
				{Name: "deployment", Label: "Deployment", Required: true},
			},
			playbook: `- hosts: localhost
  connection: local
  gather_facts: false
  tasks:
    - name: Rollout restart
      kubernetes.core.k8s:
        state: patched
        definition:
          apiVersion: apps/v1
          kind: Deployment
          metadata:
            name: "{{ deployment }}"
            namespace: "{{ target_namespace }}"
          spec:
            template:
              metadata:
                annotations:
                  k8sense.io/restartedAt: "{{ run_stamp }}"
`,
		},
		{
			ID: "scale-deployment", Name: "Scale a deployment",
			Description: "Set a deployment's replica count — e.g. scale non-prod down overnight.",
			Category:    "Operations", Icon: "mdi:arrow-expand-vertical",
			Vars: []runbookVar{
				{Name: "target_namespace", Label: "Namespace", Required: true},
				{Name: "deployment", Label: "Deployment", Required: true},
				{Name: "replicas", Label: "Replicas", Required: true, Default: "1"},
			},
			playbook: `- hosts: localhost
  connection: local
  gather_facts: false
  tasks:
    - name: Scale deployment
      kubernetes.core.k8s_scale:
        api_version: apps/v1
        kind: Deployment
        name: "{{ deployment }}"
        namespace: "{{ target_namespace }}"
        replicas: "{{ replicas | int }}"
`,
		},
	}
}

func findRunbook(id string) (runbookTemplate, bool) {
	for _, t := range runbookTemplates() {
		if t.ID == id {
			return t, true
		}
	}

	return runbookTemplate{}, false
}

func airGappedMode() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("K8SENSE_AIRGAPPED"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (s *Server) runbooksConfigPath() string {
	return filepath.Join(s.configDir(), "runbooks-config.json")
}

// runnerImage resolves the ansible-runner image: saved config, then env, then
// the default. Operators point this at an internal mirror for air-gapped sites.
func (s *Server) runnerImage() string {
	if data, err := os.ReadFile(s.runbooksConfigPath()); err == nil {
		var c struct {
			Image string `json:"image"`
		}

		if json.Unmarshal(data, &c) == nil && strings.TrimSpace(c.Image) != "" {
			return strings.TrimSpace(c.Image)
		}
	}

	if v := strings.TrimSpace(os.Getenv("K8SENSE_RUNBOOK_IMAGE")); v != "" {
		return v
	}

	return defaultRunnerImage
}

// ListRunbooks handles GET /cluster-doctor/runbooks .
func (s *Server) ListRunbooks(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]interface{}{"runbooks": runbookTemplates()})
}

type runbooksStatus struct {
	Enabled     bool   `json:"enabled"`
	AirGapped   bool   `json:"airGapped"`
	RunnerImage string `json:"runnerImage"`
	Namespace   string `json:"namespace"`
}

// RunbooksStatus handles GET /cluster-doctor/runbooks/status?cluster= — whether
// the runner is bootstrapped on this cluster, plus the air-gap flag so the UI can
// warn about the runner image before anything is attempted.
func (s *Server) RunbooksStatus(w http.ResponseWriter, r *http.Request) {
	cluster := r.URL.Query().Get("cluster")

	status := runbooksStatus{
		AirGapped:   airGappedMode(),
		RunnerImage: s.runnerImage(),
		Namespace:   runbookNamespace,
	}

	clientset, err := s.getClient(r, cluster)
	if err != nil {
		writeClusterNotFound(w, cluster)
		return
	}

	// Enabled == the runner ServiceAccount exists in the runbooks namespace.
	if _, err := clientset.CoreV1().ServiceAccounts(runbookNamespace).
		Get(r.Context(), runnerSAName, metav1.GetOptions{}); err == nil {
		status.Enabled = true
	}

	writeJSON(w, status)
}

// EnableRunbooks handles POST /cluster-doctor/runbooks/enable — the one-time
// bootstrap: namespace + runner ServiceAccount + scoped RBAC.
func (s *Server) EnableRunbooks(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Cluster string `json:"cluster"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	clientset, err := s.getClient(r, req.Cluster)
	if err != nil {
		writeClusterNotFound(w, req.Cluster)
		return
	}

	if err := bootstrapRunner(r, clientset); err != nil {
		writeRunbookError(w, "enabling Runbooks (does your account have permission to create namespaces and RBAC?)", err)
		return
	}

	logger.Log(logger.LevelInfo, map[string]string{"cluster": req.Cluster}, nil, "runbooks: enabled")
	writeJSON(w, runbooksStatus{Enabled: true, AirGapped: airGappedMode(), RunnerImage: s.runnerImage(), Namespace: runbookNamespace})
}

// bootstrapRunner creates the namespace, ServiceAccount, ClusterRole and binding
// idempotently (AlreadyExists is success).
func bootstrapRunner(r *http.Request, clientset kubernetes.Interface) error {
	ctx := r.Context()
	labels := map[string]string{"app.kubernetes.io/managed-by": "k8sense"}

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: runbookNamespace, Labels: labels}}
	if err := createIgnoreExists(clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})); err != nil {
		return err
	}

	sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: runnerSAName, Namespace: runbookNamespace, Labels: labels}}
	if err := createIgnoreExists(clientset.CoreV1().ServiceAccounts(runbookNamespace).Create(ctx, sa, metav1.CreateOptions{})); err != nil {
		return err
	}

	role := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: runnerRoleName, Labels: labels},
		Rules: []rbacv1.PolicyRule{
			{APIGroups: []string{""}, Resources: []string{"namespaces", "resourcequotas", "limitranges", "configmaps", "serviceaccounts", "services", "pods"}, Verbs: []string{"get", "list", "watch", "create", "update", "patch", "delete"}},
			{APIGroups: []string{"apps"}, Resources: []string{"deployments", "statefulsets", "daemonsets"}, Verbs: []string{"get", "list", "watch", "update", "patch"}},
			{APIGroups: []string{"networking.k8s.io"}, Resources: []string{"networkpolicies"}, Verbs: []string{"get", "list", "create", "update", "patch", "delete"}},
			{APIGroups: []string{"rbac.authorization.k8s.io"}, Resources: []string{"roles", "rolebindings"}, Verbs: []string{"get", "list", "create", "update", "patch", "delete"}},
		},
	}
	if err := createIgnoreExists(clientset.RbacV1().ClusterRoles().Create(ctx, role, metav1.CreateOptions{})); err != nil {
		return err
	}

	binding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: runnerRoleName, Labels: labels},
		RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: runnerRoleName},
		Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: runnerSAName, Namespace: runbookNamespace}},
	}

	return createIgnoreExists(clientset.RbacV1().ClusterRoleBindings().Create(ctx, binding, metav1.CreateOptions{}))
}

func createIgnoreExists(_ interface{}, err error) error {
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

type runRunbookRequest struct {
	Cluster   string            `json:"cluster"`
	RunbookID string            `json:"runbookId"`
	Vars      map[string]string `json:"vars"`
	Check     bool              `json:"check"`
}

// RunRunbook handles POST /cluster-doctor/runbooks/run — validates inputs, then
// creates a ConfigMap (playbook + vars) and a Job that executes it.
func (s *Server) RunRunbook(w http.ResponseWriter, r *http.Request) {
	var req runRunbookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	tmpl, ok := findRunbook(req.RunbookID)
	if !ok {
		http.Error(w, `{"error":"unknown runbook"}`, http.StatusNotFound)
		return
	}

	// Validate required vars up front so the failure is plain, not an obscure
	// Ansible error mid-run.
	if req.Vars == nil {
		req.Vars = map[string]string{}
	}

	for _, v := range tmpl.Vars {
		if v.Required && strings.TrimSpace(req.Vars[v.Name]) == "" {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, v.Label+" is required."), http.StatusBadRequest)
			return
		}

		if strings.TrimSpace(req.Vars[v.Name]) == "" && v.Default != "" {
			req.Vars[v.Name] = v.Default
		}
	}

	clientset, err := s.getClient(r, req.Cluster)
	if err != nil {
		writeClusterNotFound(w, req.Cluster)
		return
	}

	// run_stamp is always available to templates.
	req.Vars["run_stamp"] = time.Now().UTC().Format(time.RFC3339)

	extravars, err := json.Marshal(req.Vars)
	if err != nil {
		writeRunbookError(w, "encoding variables", err)
		return
	}

	jobName := fmt.Sprintf("k8sense-rb-%s-%d", tmpl.ID, time.Now().Unix())
	if len(jobName) > 63 { //nolint:mnd // k8s name length limit
		jobName = jobName[:63]
	}

	labels := map[string]string{"app.kubernetes.io/managed-by": "k8sense", "k8sense.io/runbook": tmpl.ID}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: jobName, Namespace: runbookNamespace, Labels: labels},
		Data:       map[string]string{"playbook.yml": tmpl.playbook, "extravars.json": string(extravars)},
	}
	if _, err := clientset.CoreV1().ConfigMaps(runbookNamespace).Create(r.Context(), cm, metav1.CreateOptions{}); err != nil {
		writeRunbookError(w, "creating the run (is Runbooks enabled on this cluster?)", err)
		return
	}

	job := runnerJob(jobName, s.runnerImage(), req.Check, labels)
	if _, err := clientset.BatchV1().Jobs(runbookNamespace).Create(r.Context(), job, metav1.CreateOptions{}); err != nil {
		writeRunbookError(w, "starting the runner Job", err)
		return
	}

	logger.Log(logger.LevelInfo, map[string]string{"cluster": req.Cluster, "runbook": tmpl.ID, "check": fmt.Sprint(req.Check)}, nil,
		"runbooks: run started")

	writeJSON(w, map[string]string{"runId": jobName, "namespace": runbookNamespace})
}

// runnerJob builds the Job that runs one playbook.
func runnerJob(name, image string, check bool, labels map[string]string) *batchv1.Job {
	cmd := []string{"ansible-playbook", "/work/playbook.yml", "-i", "localhost,", "-e", "@/work/extravars.json"}
	if check {
		cmd = append(cmd, "--check")
	}

	backoff := int32(0)
	ttl := int32(runbookTTLSeconds)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: runbookNamespace, Labels: labels},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoff,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					ServiceAccountName: runnerSAName,
					RestartPolicy:      corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name:         "runner",
						Image:        image,
						Command:      cmd,
						VolumeMounts: []corev1.VolumeMount{{Name: "work", MountPath: "/work"}},
					}},
					Volumes: []corev1.Volume{{
						Name: "work",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: name},
							},
						},
					}},
				},
			},
		},
	}
}

// RunbookLogs handles GET /cluster-doctor/runbooks/run/{runId}/logs?cluster= —
// polled by the UI for live output and completion.
func (s *Server) RunbookLogs(w http.ResponseWriter, r *http.Request) {
	cluster := r.URL.Query().Get("cluster")
	runID := mux.Vars(r)["runId"]

	clientset, err := s.getClient(r, cluster)
	if err != nil {
		writeClusterNotFound(w, cluster)
		return
	}

	phase, finished := jobPhase(r, clientset, runID)
	logs := jobLogs(r, clientset, runID)

	writeJSON(w, map[string]interface{}{"phase": phase, "finished": finished, "logs": logs})
}

func jobPhase(r *http.Request, clientset kubernetes.Interface, jobName string) (string, bool) {
	job, err := clientset.BatchV1().Jobs(runbookNamespace).Get(r.Context(), jobName, metav1.GetOptions{})
	if err != nil {
		return "Unknown", false
	}

	switch {
	case job.Status.Succeeded > 0:
		return "Succeeded", true
	case job.Status.Failed > 0:
		return "Failed", true
	case job.Status.Active > 0:
		return "Running", false
	default:
		return "Pending", false
	}
}

// jobLogs returns the runner pod's logs, or "" if the pod isn't up yet.
func jobLogs(r *http.Request, clientset kubernetes.Interface, jobName string) string {
	pods, err := clientset.CoreV1().Pods(runbookNamespace).List(r.Context(), metav1.ListOptions{
		LabelSelector: "job-name=" + jobName,
	})
	if err != nil || len(pods.Items) == 0 {
		return ""
	}

	data, err := clientset.CoreV1().Pods(runbookNamespace).
		GetLogs(pods.Items[0].Name, &corev1.PodLogOptions{}).DoRaw(r.Context())
	if err != nil {
		return ""
	}

	return string(data)
}

// SetRunbooksConfig handles PUT /cluster-doctor/runbooks/config — set the runner
// image (e.g. an internal mirror for air-gapped sites).
func (s *Server) SetRunbooksConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Image string `json:"image"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	req.Image = strings.TrimSpace(req.Image)
	if req.Image == "" {
		http.Error(w, `{"error":"a runner image is required"}`, http.StatusBadRequest)
		return
	}

	data, _ := json.MarshalIndent(req, "", "  ")
	if err := os.WriteFile(s.runbooksConfigPath(), data, 0o600); err != nil {
		writeRunbookError(w, "saving the runner image", err)
		return
	}

	writeJSON(w, map[string]string{"image": s.runnerImage()})
}

func writeRunbookError(w http.ResponseWriter, context string, err error) {
	logger.Log(logger.LevelError, nil, err, "runbooks: "+context)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadGateway)

	body, _ := json.Marshal(map[string]string{"error": context + ": " + err.Error()})
	_, _ = w.Write(body)
}
