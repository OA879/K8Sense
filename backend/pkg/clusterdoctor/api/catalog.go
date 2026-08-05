package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/OA879/K8Sense/backend/pkg/helm"
	"github.com/OA879/K8Sense/backend/pkg/logger"
)

// The App Catalog turns the (previously empty) plugins page into a curated
// marketplace of production-grade open-source operational tools — Prometheus,
// Grafana, Velero, cert-manager and friends — each installable to the pointed
// cluster in one click via the vendored Helm SDK, and removable in one click
// (Helm uninstall deletes the release, so the spun-up pods go with it). The
// charts come from each project's official repo; for air-gapped sites, point
// the repo URLs and image registry at internal mirrors.

const catalogInstallTimeout = 5 * time.Minute

// catalogApp is one installable tool. The install coordinates (repo/chart/values)
// are server-only; the client sees just presentation + placement.
type catalogApp struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Icon        string `json:"icon"`
	Namespace   string `json:"namespace"`

	repoURL     string
	chart       string
	releaseName string
	version     string // "" = latest
	values      map[string]interface{}
}

// catalogApps is the curated set. Values are chosen so each chart installs and
// runs cleanly on a vanilla cluster with no extra input.
func catalogApps() []catalogApp {
	return []catalogApp{
		{
			ID: "kube-prometheus-stack", Name: "Prometheus & Grafana",
			Description: "Full monitoring stack — Prometheus, Grafana dashboards and Alertmanager (kube-prometheus-stack).",
			Category:    "Observability", Icon: "mdi:chart-areaspline", Namespace: "monitoring",
			repoURL: "https://prometheus-community.github.io/helm-charts", chart: "kube-prometheus-stack",
			releaseName: "kube-prometheus-stack",
		},
		{
			ID: "grafana", Name: "Grafana",
			Description: "Standalone Grafana for dashboards and visualization.",
			Category:    "Observability", Icon: "mdi:chart-box-outline", Namespace: "monitoring",
			repoURL: "https://grafana.github.io/helm-charts", chart: "grafana", releaseName: "grafana",
		},
		{
			ID: "loki", Name: "Loki (logs)",
			Description: "Grafana Loki log aggregation with the Promtail collector.",
			Category:    "Observability", Icon: "mdi:text-box-search-outline", Namespace: "logging",
			repoURL: "https://grafana.github.io/helm-charts", chart: "loki-stack", releaseName: "loki",
			values: map[string]interface{}{"grafana": map[string]interface{}{"enabled": false}},
		},
		{
			ID: "metrics-server", Name: "Metrics Server",
			Description: "Cluster-wide CPU/memory metrics that power kubectl top and the HPA.",
			Category:    "Core", Icon: "mdi:gauge", Namespace: "kube-system",
			repoURL: "https://kubernetes-sigs.github.io/metrics-server/", chart: "metrics-server",
			releaseName: "metrics-server",
			// Tolerate self-signed kubelet certs (kind, kubeadm defaults).
			values: map[string]interface{}{"args": []interface{}{"--kubelet-insecure-tls"}},
		},
		{
			ID: "cert-manager", Name: "cert-manager",
			Description: "Automated TLS certificate issuance and renewal (Let's Encrypt, internal CAs).",
			Category:    "Security", Icon: "mdi:certificate-outline", Namespace: "cert-manager",
			repoURL: "https://charts.jetstack.io", chart: "cert-manager", releaseName: "cert-manager",
			values: map[string]interface{}{"crds": map[string]interface{}{"enabled": true}},
		},
		{
			ID: "ingress-nginx", Name: "Ingress NGINX",
			Description: "The NGINX ingress controller for routing external traffic into the cluster.",
			Category:    "Networking", Icon: "mdi:router-network", Namespace: "ingress-nginx",
			repoURL: "https://kubernetes.github.io/ingress-nginx", chart: "ingress-nginx",
			releaseName: "ingress-nginx",
		},
		{
			ID: "velero", Name: "Velero (backup)",
			Description: "Backup and restore for cluster resources and volumes. Configure a storage target after install.",
			Category:    "Backup & DR", Icon: "mdi:backup-restore", Namespace: "velero",
			repoURL: "https://vmware-tanzu.github.io/helm-charts", chart: "velero", releaseName: "velero",
			// Install the server without a cloud provider so it comes up cleanly;
			// a backup storage location is added later.
			values: map[string]interface{}{
				"credentials":      map[string]interface{}{"useSecret": false},
				"snapshotsEnabled": false,
				"backupsEnabled":   false,
				"deployNodeAgent":  false,
			},
		},
	}
}

func findCatalogApp(id string) (catalogApp, bool) {
	for _, a := range catalogApps() {
		if a.ID == id {
			return a, true
		}
	}

	return catalogApp{}, false
}

// catalogAppView is an app plus its per-cluster install state.
type catalogAppView struct {
	catalogApp
	Installed bool   `json:"installed"`
	Status    string `json:"status,omitempty"`
}

// Catalog handles GET /cluster-doctor/catalog?cluster= . It lists every app with
// whether it is currently installed on the target cluster.
func (s *Server) Catalog(w http.ResponseWriter, r *http.Request) {
	cluster := r.URL.Query().Get("cluster")

	installed := map[string]string{} // releaseName -> status

	if cfg, err := s.clusterClientConfig(r, cluster); err == nil {
		installed = listHelmReleases(cfg)
	}

	apps := catalogApps()
	views := make([]catalogAppView, 0, len(apps))

	for _, a := range apps {
		status, ok := installed[a.releaseName]
		views = append(views, catalogAppView{catalogApp: a, Installed: ok, Status: status})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"apps": views})
}

type catalogActionRequest struct {
	Cluster string `json:"cluster"`
	AppID   string `json:"appId"`
}

type catalogActionResponse struct {
	Result  string `json:"result"` // success | failed
	Message string `json:"message"`
}

// InstallApp handles POST /cluster-doctor/catalog/install .
func (s *Server) InstallApp(w http.ResponseWriter, r *http.Request) {
	req, app, cfg, ok := s.catalogAction(w, r)
	if !ok {
		return
	}

	actionConfig, err := helm.NewActionConfig(cfg, app.Namespace)
	if err != nil {
		writeCatalogError(w, "initialising Helm", err)
		return
	}

	install := action.NewInstall(actionConfig)
	install.ReleaseName = app.releaseName
	install.Namespace = app.Namespace
	install.CreateNamespace = true
	install.Timeout = catalogInstallTimeout
	install.ChartPathOptions.RepoURL = app.repoURL
	install.ChartPathOptions.Version = app.version

	chartPath, err := install.ChartPathOptions.LocateChart(app.chart, cli.New())
	if err != nil {
		writeCatalogError(w, "locating chart (is the cluster/registry reachable?)", err)
		return
	}

	chart, err := loader.Load(chartPath)
	if err != nil {
		writeCatalogError(w, "loading chart", err)
		return
	}

	if _, err := install.Run(chart, app.values); err != nil {
		writeCatalogError(w, "installing "+app.Name, err)
		return
	}

	logger.Log(logger.LevelInfo, map[string]string{"cluster": req.Cluster, "app": app.ID}, nil,
		"app-catalog: installed")

	writeJSON(w, catalogActionResponse{Result: "success",
		Message: fmt.Sprintf("%s is installing into namespace %q.", app.Name, app.Namespace)})
}

// UninstallApp handles POST /cluster-doctor/catalog/uninstall . Helm uninstall
// deletes the release and every resource it created, so the pods are torn down.
func (s *Server) UninstallApp(w http.ResponseWriter, r *http.Request) {
	req, app, cfg, ok := s.catalogAction(w, r)
	if !ok {
		return
	}

	actionConfig, err := helm.NewActionConfig(cfg, app.Namespace)
	if err != nil {
		writeCatalogError(w, "initialising Helm", err)
		return
	}

	uninstall := action.NewUninstall(actionConfig)
	uninstall.Timeout = catalogInstallTimeout

	if _, err := uninstall.Run(app.releaseName); err != nil {
		writeCatalogError(w, "uninstalling "+app.Name, err)
		return
	}

	logger.Log(logger.LevelInfo, map[string]string{"cluster": req.Cluster, "app": app.ID}, nil,
		"app-catalog: uninstalled")

	writeJSON(w, catalogActionResponse{Result: "success",
		Message: fmt.Sprintf("%s and its pods have been removed.", app.Name)})
}

// catalogAction decodes and validates a catalog request and resolves the cluster
// config, writing the appropriate error response and returning ok=false on any
// failure so the handlers stay linear.
func (s *Server) catalogAction(w http.ResponseWriter, r *http.Request) (catalogActionRequest, catalogApp, clientcmd.ClientConfig, bool) {
	var req catalogActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return req, catalogApp{}, nil, false
	}

	app, found := findCatalogApp(req.AppID)
	if !found {
		http.Error(w, `{"error":"unknown app"}`, http.StatusNotFound)
		return req, catalogApp{}, nil, false
	}

	cfg, err := s.clusterClientConfig(r, req.Cluster)
	if err != nil {
		writeClusterNotFound(w, req.Cluster)
		return req, catalogApp{}, nil, false
	}

	return req, app, cfg, true
}

// clusterClientConfig resolves the Helm client config for a cluster, or an error
// if the catalog isn't wired or the cluster can't be resolved.
func (s *Server) clusterClientConfig(r *http.Request, cluster string) (clientcmd.ClientConfig, error) {
	if s.getClientConfig == nil {
		return nil, fmt.Errorf("app catalog is not enabled on this server")
	}

	if cluster == "" {
		return nil, fmt.Errorf("no cluster specified")
	}

	return s.getClientConfig(r, cluster)
}

// listHelmReleases returns installed release names (across all namespaces) to a
// status string. Best-effort: on any error it returns an empty map, so the
// catalog still renders (everything shows as not-installed).
func listHelmReleases(cfg clientcmd.ClientConfig) map[string]string {
	out := map[string]string{}

	actionConfig, err := helm.NewActionConfig(cfg, "")
	if err != nil {
		return out
	}

	list := action.NewList(actionConfig)
	list.All = true
	list.AllNamespaces = true
	list.SetStateMask()

	releases, err := list.Run()
	if err != nil {
		return out
	}

	for _, rel := range releases {
		out[rel.Name] = rel.Info.Status.String()
	}

	return out
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// writeClusterNotFound emits a plain, actionable 404 — the user should know
// exactly what to do, not decode HTTP jargon. Shared so every cluster-scoped
// handler speaks the same way.
func writeClusterNotFound(w http.ResponseWriter, cluster string) {
	msg := fmt.Sprintf(
		"Cluster %q isn't connected to this backend. Select or re-add it in the cluster list, then try again.",
		cluster)

	body, _ := json.Marshal(map[string]string{"error": msg})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write(body)
}

func writeCatalogError(w http.ResponseWriter, context string, err error) {
	logger.Log(logger.LevelError, nil, err, "app-catalog: "+context)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadGateway)
	_ = json.NewEncoder(w).Encode(catalogActionResponse{
		Result: "failed", Message: context + ": " + err.Error(),
	})
}
