package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/mux"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/OA879/K8Sense/backend/pkg/logger"
)

// Vulnerability scanning: scan the container images actually running in the
// pointed cluster for CVEs, using Trivy. Like Runbooks, the scan runs as an
// in-cluster Job (no local dependency, OS-independent) and is air-gap-capable —
// Trivy reads its vulnerability DB from an internal mirror when TRIVY_DB_REPOSITORY
// is set. The backend gathers the image list from the API, the Job scans each
// image and prints JSON, and the backend parses it into a severity-ranked report.

const (
	vulnNamespace     = "k8sense-vulnscan"
	defaultTrivyImage = "aquasec/trivy:latest"
	maxScanImages     = 60
	vulnTTLSeconds    = 3600
	imageMarker       = "@@IMAGE "
	endMarker         = "@@END"
)

var severityOrder = map[string]int{"CRITICAL": 0, "HIGH": 1, "MEDIUM": 2, "LOW": 3, "UNKNOWN": 4}

// imageVuln is one CVE in one image.
type imageVuln struct {
	VulnID           string `json:"vulnId"`
	Severity         string `json:"severity"`
	PkgName          string `json:"pkgName"`
	InstalledVersion string `json:"installedVersion"`
	FixedVersion     string `json:"fixedVersion,omitempty"`
	Title            string `json:"title,omitempty"`
}

// imageResult is one image's scan outcome.
type imageResult struct {
	Image      string         `json:"image"`
	Namespaces []string       `json:"namespaces"`
	Counts     map[string]int `json:"counts"`
	Vulns      []imageVuln    `json:"vulns"`
	Error      string         `json:"error,omitempty"`
}

// vulnReport is the whole scan.
type vulnReport struct {
	Totals map[string]int `json:"totals"`
	Images []imageResult  `json:"images"`
}

func (s *Server) vulnConfigPath() string {
	return filepath.Join(s.configDir(), "vulnscan-config.json")
}

type vulnConfig struct {
	Image        string `json:"image"`
	DBRepository string `json:"dbRepository,omitempty"`
}

// trivyConfig resolves the Trivy image and optional mirrored DB repository.
func (s *Server) trivyConfig() vulnConfig {
	cfg := vulnConfig{Image: defaultTrivyImage}

	if data, err := os.ReadFile(s.vulnConfigPath()); err == nil {
		var stored vulnConfig
		if json.Unmarshal(data, &stored) == nil {
			if strings.TrimSpace(stored.Image) != "" {
				cfg.Image = strings.TrimSpace(stored.Image)
			}

			cfg.DBRepository = strings.TrimSpace(stored.DBRepository)
		}
	}

	if v := strings.TrimSpace(os.Getenv("K8SENSE_TRIVY_IMAGE")); v != "" {
		cfg.Image = v
	}

	if v := strings.TrimSpace(os.Getenv("TRIVY_DB_REPOSITORY")); v != "" && cfg.DBRepository == "" {
		cfg.DBRepository = v
	}

	return cfg
}

// VulnScanConfig handles GET /cluster-doctor/vulnscan/config .
func (s *Server) VulnScanConfig(w http.ResponseWriter, r *http.Request) {
	cfg := s.trivyConfig()
	writeJSON(w, map[string]interface{}{"image": cfg.Image, "dbRepository": cfg.DBRepository, "airGapped": airGappedMode()})
}

// SetVulnScanConfig handles PUT /cluster-doctor/vulnscan/config .
func (s *Server) SetVulnScanConfig(w http.ResponseWriter, r *http.Request) {
	var req vulnConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	req.Image = strings.TrimSpace(req.Image)
	if req.Image == "" {
		http.Error(w, `{"error":"a Trivy image is required"}`, http.StatusBadRequest)
		return
	}

	req.DBRepository = strings.TrimSpace(req.DBRepository)

	data, _ := json.MarshalIndent(req, "", "  ")
	if err := os.WriteFile(s.vulnConfigPath(), data, 0o600); err != nil {
		writeRunbookError(w, "saving the scanner config", err)
		return
	}

	cfg := s.trivyConfig()
	writeJSON(w, map[string]interface{}{"image": cfg.Image, "dbRepository": cfg.DBRepository, "airGapped": airGappedMode()})
}

// gatherImages maps each unique running image to the namespaces it runs in.
func gatherImages(ctx context.Context, clientset kubernetes.Interface) (map[string][]string, error) {
	pods, err := clientset.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	set := map[string]map[string]bool{}

	add := func(image, ns string) {
		if image == "" {
			return
		}

		if set[image] == nil {
			set[image] = map[string]bool{}
		}

		set[image][ns] = true
	}

	for i := range pods.Items {
		p := &pods.Items[i]
		for _, c := range p.Spec.InitContainers {
			add(c.Image, p.Namespace)
		}

		for _, c := range p.Spec.Containers {
			add(c.Image, p.Namespace)
		}
	}

	out := map[string][]string{}

	for image, nsSet := range set {
		var namespaces []string
		for ns := range nsSet {
			namespaces = append(namespaces, ns)
		}

		sort.Strings(namespaces)
		out[image] = namespaces
	}

	return out, nil
}

// RunVulnScan handles POST /cluster-doctor/vulnscan — gather images and start the
// Trivy Job.
func (s *Server) RunVulnScan(w http.ResponseWriter, r *http.Request) {
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

	images, err := gatherImages(r.Context(), clientset)
	if err != nil {
		writeRunbookError(w, "listing running images", err)
		return
	}

	if len(images) == 0 {
		http.Error(w, `{"error":"No running images found to scan."}`, http.StatusBadRequest)
		return
	}

	// Bound the work: scan the first maxScanImages unique images (sorted for a
	// stable selection).
	names := make([]string, 0, len(images))
	for image := range images {
		names = append(names, image)
	}

	sort.Strings(names)

	if len(names) > maxScanImages {
		names = names[:maxScanImages]
	}

	meta, _ := json.Marshal(images)

	if err := ensureNamespace(r.Context(), clientset, vulnNamespace); err != nil {
		writeRunbookError(w, "creating the scan namespace (do you have permission?)", err)
		return
	}

	runID := fmt.Sprintf("k8sense-vuln-%d", time.Now().Unix())
	labels := map[string]string{"app.kubernetes.io/managed-by": "k8sense", "k8sense.io/vulnscan": "true"}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: runID, Namespace: vulnNamespace, Labels: labels},
		Data:       map[string]string{"images.txt": strings.Join(names, "\n"), "meta.json": string(meta)},
	}
	if _, err := clientset.CoreV1().ConfigMaps(vulnNamespace).Create(r.Context(), cm, metav1.CreateOptions{}); err != nil {
		writeRunbookError(w, "creating the scan", err)
		return
	}

	job := trivyJob(runID, s.trivyConfig(), labels)
	if _, err := clientset.BatchV1().Jobs(vulnNamespace).Create(r.Context(), job, metav1.CreateOptions{}); err != nil {
		writeRunbookError(w, "starting the Trivy Job", err)
		return
	}

	logger.Log(logger.LevelInfo, map[string]string{"cluster": req.Cluster, "images": fmt.Sprint(len(names))}, nil,
		"vulnscan: started")

	writeJSON(w, map[string]interface{}{"runId": runID, "imageCount": len(names)})
}

// trivyJob builds the Job that scans each image and prints delimited JSON.
func trivyJob(name string, cfg vulnConfig, labels map[string]string) *batchv1.Job {
	script := "while IFS= read -r img; do " +
		"[ -z \"$img\" ] && continue; " +
		"echo \"" + imageMarker + "$img\"; " +
		"trivy image --quiet --scanners vuln --format json \"$img\" 2>/dev/null || echo '{\"__scan_error__\":true}'; " +
		"echo \"" + endMarker + "\"; " +
		"done < /work/images.txt"

	env := []corev1.EnvVar{}
	if cfg.DBRepository != "" {
		env = append(env, corev1.EnvVar{Name: "TRIVY_DB_REPOSITORY", Value: cfg.DBRepository})
	}

	backoff := int32(0)
	ttl := int32(vulnTTLSeconds)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: vulnNamespace, Labels: labels},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoff,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name:         "trivy",
						Image:        cfg.Image,
						Command:      []string{"sh", "-c", script},
						Env:          env,
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

// VulnScanStatus handles GET /cluster-doctor/vulnscan/{runId}?cluster= — phase
// while running, and the parsed report once the Job finishes.
func (s *Server) VulnScanStatus(w http.ResponseWriter, r *http.Request) {
	cluster := r.URL.Query().Get("cluster")
	runID := mux.Vars(r)["runId"]

	clientset, err := s.getClient(r, cluster)
	if err != nil {
		writeClusterNotFound(w, cluster)
		return
	}

	phase, finished := jobPhaseIn(r.Context(), clientset, vulnNamespace, runID)

	resp := map[string]interface{}{"phase": phase, "finished": finished}

	if finished {
		logs := jobLogsIn(r.Context(), clientset, vulnNamespace, runID)
		namespaces := scanMeta(r.Context(), clientset, runID)
		resp["report"] = buildVulnReport(logs, namespaces)
	}

	writeJSON(w, resp)
}

// scanMeta reads the image→namespaces map saved at scan time.
func scanMeta(ctx context.Context, clientset kubernetes.Interface, runID string) map[string][]string {
	cm, err := clientset.CoreV1().ConfigMaps(vulnNamespace).Get(ctx, runID, metav1.GetOptions{})
	if err != nil {
		return map[string][]string{}
	}

	var meta map[string][]string
	if json.Unmarshal([]byte(cm.Data["meta.json"]), &meta) != nil {
		return map[string][]string{}
	}

	return meta
}

// buildVulnReport parses the delimited Trivy output into a ranked report. Kept
// pure so it's unit-testable without a cluster.
func buildVulnReport(logs string, namespaces map[string][]string) vulnReport {
	report := vulnReport{Totals: map[string]int{}}

	for _, block := range splitImageBlocks(logs) {
		res := imageResult{Image: block.image, Namespaces: namespaces[block.image], Counts: map[string]int{}}

		var parsed struct {
			ScanError bool `json:"__scan_error__"`
			Results   []struct {
				Vulnerabilities []struct {
					VulnerabilityID  string `json:"VulnerabilityID"`
					PkgName          string `json:"PkgName"`
					InstalledVersion string `json:"InstalledVersion"`
					FixedVersion     string `json:"FixedVersion"`
					Severity         string `json:"Severity"`
					Title            string `json:"Title"`
				} `json:"Vulnerabilities"`
			} `json:"Results"`
		}

		if err := json.Unmarshal([]byte(block.body), &parsed); err != nil || parsed.ScanError {
			res.Error = "could not scan this image (unreachable registry, or Trivy DB missing?)"
			report.Images = append(report.Images, res)

			continue
		}

		for _, rs := range parsed.Results {
			for _, v := range rs.Vulnerabilities {
				sev := strings.ToUpper(v.Severity)
				res.Counts[sev]++
				report.Totals[sev]++
				res.Vulns = append(res.Vulns, imageVuln{
					VulnID: v.VulnerabilityID, Severity: sev, PkgName: v.PkgName,
					InstalledVersion: v.InstalledVersion, FixedVersion: v.FixedVersion, Title: v.Title,
				})
			}
		}

		sort.SliceStable(res.Vulns, func(i, j int) bool {
			return severityOrder[res.Vulns[i].Severity] < severityOrder[res.Vulns[j].Severity]
		})

		report.Images = append(report.Images, res)
	}

	// Most-vulnerable images first (by critical, then high).
	sort.SliceStable(report.Images, func(i, j int) bool {
		if report.Images[i].Counts["CRITICAL"] != report.Images[j].Counts["CRITICAL"] {
			return report.Images[i].Counts["CRITICAL"] > report.Images[j].Counts["CRITICAL"]
		}

		return report.Images[i].Counts["HIGH"] > report.Images[j].Counts["HIGH"]
	})

	return report
}

type imageBlock struct {
	image string
	body  string
}

// splitImageBlocks extracts each "@@IMAGE <ref> ... @@END" section.
func splitImageBlocks(logs string) []imageBlock {
	var blocks []imageBlock

	lines := strings.Split(logs, "\n")

	var current *imageBlock

	var body strings.Builder

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, imageMarker):
			current = &imageBlock{image: strings.TrimSpace(strings.TrimPrefix(line, imageMarker))}
			body.Reset()
		case strings.TrimSpace(line) == endMarker:
			if current != nil {
				current.body = body.String()
				blocks = append(blocks, *current)
				current = nil
			}
		default:
			if current != nil {
				body.WriteString(line)
				body.WriteByte('\n')
			}
		}
	}

	return blocks
}

func ensureNamespace(ctx context.Context, clientset kubernetes.Interface, name string) error {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{"app.kubernetes.io/managed-by": "k8sense"}}}

	_, err := clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

// jobPhaseIn / jobLogsIn are namespace-parameterised versions of the runbook
// helpers, reused for the vuln-scan Job.
func jobPhaseIn(ctx context.Context, clientset kubernetes.Interface, namespace, jobName string) (string, bool) {
	job, err := clientset.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
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

func jobLogsIn(ctx context.Context, clientset kubernetes.Interface, namespace, jobName string) string {
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: "job-name=" + jobName})
	if err != nil || len(pods.Items) == 0 {
		return ""
	}

	data, err := clientset.CoreV1().Pods(namespace).GetLogs(pods.Items[0].Name, &corev1.PodLogOptions{}).DoRaw(ctx)
	if err != nil {
		return ""
	}

	return string(data)
}
