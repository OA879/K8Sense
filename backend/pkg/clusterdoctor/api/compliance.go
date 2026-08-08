package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CIS Kubernetes Benchmark compliance mode. It evaluates the cluster against a
// curated set of CIS controls that are derivable from the plain Kubernetes API
// (no node access) — mostly Section 5 (RBAC, Pod Security Standards, Network
// Policies, general policies) — and produces an audit-ready report: pass/fail
// per control and an overall score. Runs fully offline, like everything else.

// complianceViolation is one resource failing a control.
type complianceViolation struct {
	Namespace string `json:"namespace,omitempty"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Detail    string `json:"detail,omitempty"`
}

// controlResult is a control's outcome.
type controlResult struct {
	ID          string                `json:"id"`
	Title       string                `json:"title"`
	Section     string                `json:"section"`
	Status      string                `json:"status"` // pass | fail
	Remediation string                `json:"remediation"`
	Violations  []complianceViolation `json:"violations"`
}

// controlRemediations maps each control to plain, actionable guidance shown with
// its findings — what to change to make it pass. Kept beside the controls so the
// two stay in sync.
var controlRemediations = map[string]string{
	"CIS-5.1.1": "Remove the cluster-admin ClusterRoleBinding and grant a ClusterRole/Role scoped to only what the subject actually needs.",
	"CIS-5.1.3": "Replace wildcard (*) verbs, resources or apiGroups with the explicit, minimal set the workload requires.",
	"CIS-5.2.1": "Set securityContext.privileged: false; grant only the specific Linux capabilities the container needs.",
	"CIS-5.2.2": "Remove hostPID: true from the pod spec — containers should not share the host process namespace.",
	"CIS-5.2.3": "Remove hostIPC: true from the pod spec.",
	"CIS-5.2.4": "Remove hostNetwork: true; expose the workload through a Service instead of host networking.",
	"CIS-5.2.5": "Set securityContext.allowPrivilegeEscalation: false on each container.",
	"CIS-5.2.6": "Set securityContext.runAsNonRoot: true (and a non-zero runAsUser) so the container can't run as root.",
	"CIS-5.2.8": "Drop the added Linux capabilities; add back only those strictly required.",
	"CIS-5.2.9": "Add securityContext.capabilities.drop: [\"ALL\"], then add back only the capabilities the container needs.",
	"CIS-5.3.2": "Create a default-deny NetworkPolicy (ingress + egress) in the namespace, then allow only required flows. Runbooks → “Apply default-deny NetworkPolicy” does this in one click.",
	"CIS-5.7.4": "Move workloads out of the default namespace into a dedicated, labelled namespace. Runbooks → “Onboard a namespace” sets one up with a quota and default-deny policy.",
}

// complianceReport is the whole benchmark run.
type complianceReport struct {
	Framework string          `json:"framework"`
	Score     int             `json:"score"` // percent of evaluated controls passing
	Passed    int             `json:"passed"`
	Failed    int             `json:"failed"`
	Total     int             `json:"total"`
	Controls  []controlResult `json:"controls"`
	Note      string          `json:"note"`
}

// complianceData is gathered once and shared across all control checks.
type complianceData struct {
	pods    []corev1.Pod
	crbs    []rbacv1.ClusterRoleBinding
	croles  []rbacv1.ClusterRole
	roles   []rbacv1.Role
	netpols []networkingv1.NetworkPolicy
}

// cisControl pairs a control's metadata with its check.
type cisControl struct {
	id      string
	title   string
	section string
	check   func(complianceData) []complianceViolation
}

// controls are evaluated against workload namespaces to keep the score meaningful.
//
//nolint:gochecknoglobals // system namespaces are platform-managed; CIS pod-security
var complianceSystemNS = map[string]bool{
	"kube-system": true, "kube-public": true, "kube-node-lease": true,
	"local-path-storage": true,
}

// Compliance handles GET /cluster-doctor/compliance?cluster=&namespace= .
func (s *Server) Compliance(w http.ResponseWriter, r *http.Request) {
	cluster := r.URL.Query().Get("cluster")

	clientset, err := s.getClient(r, cluster)
	if err != nil {
		writeClusterNotFound(w, cluster)
		return
	}

	report, err := buildComplianceReport(r.Context(), clientset, r.URL.Query().Get("namespace"))
	if err != nil {
		http.Error(w, `{"error": "could not build compliance report"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(report)
}

func buildComplianceReport(ctx context.Context, clientset kubernetes.Interface, namespace string) (complianceReport, error) {
	data, err := gatherComplianceData(ctx, clientset, namespace)
	if err != nil {
		return complianceReport{}, err
	}

	report := complianceReport{
		Framework: "CIS Kubernetes Benchmark (API-derivable subset)",
		Note: "System namespaces (kube-system, etc.) are excluded from Pod Security " +
			"controls, as they are platform-managed. Node/control-plane file checks require " +
			"host access and are out of scope here.",
	}

	for _, c := range cisControls() {
		violations := c.check(data)

		status := "pass"
		if len(violations) > 0 {
			status = "fail"
			report.Failed++
		} else {
			report.Passed++
		}

		if violations == nil {
			violations = []complianceViolation{}
		}

		report.Controls = append(report.Controls, controlResult{
			ID: c.id, Title: c.title, Section: c.section, Status: status,
			Remediation: controlRemediations[c.id], Violations: violations,
		})
	}

	report.Total = report.Passed + report.Failed
	if report.Total > 0 {
		report.Score = report.Passed * 100 / report.Total
	}

	sort.Slice(report.Controls, func(i, j int) bool { return report.Controls[i].ID < report.Controls[j].ID })

	return report, nil
}

func gatherComplianceData(ctx context.Context, clientset kubernetes.Interface, namespace string) (complianceData, error) {
	ns := namespace
	if ns == "" {
		ns = metav1.NamespaceAll
	}

	pods, err := clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return complianceData{}, err
	}

	data := complianceData{pods: pods.Items}

	if crbs, err := clientset.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{}); err == nil {
		data.crbs = crbs.Items
	}

	if croles, err := clientset.RbacV1().ClusterRoles().List(ctx, metav1.ListOptions{}); err == nil {
		data.croles = croles.Items
	}

	if roles, err := clientset.RbacV1().Roles(ns).List(ctx, metav1.ListOptions{}); err == nil {
		data.roles = roles.Items
	}

	if nps, err := clientset.NetworkingV1().NetworkPolicies(ns).List(ctx, metav1.ListOptions{}); err == nil {
		data.netpols = nps.Items
	}

	return data, nil
}

// workloadPods returns pods outside the platform-managed system namespaces.
func (d complianceData) workloadPods() []corev1.Pod {
	var out []corev1.Pod

	for _, p := range d.pods {
		if !complianceSystemNS[p.Namespace] {
			out = append(out, p)
		}
	}

	return out
}

func podViolation(p corev1.Pod, detail string) complianceViolation {
	return complianceViolation{Namespace: p.Namespace, Kind: "Pod", Name: p.Name, Detail: detail}
}

// podContainers returns init + regular containers.
func podContainers(p corev1.Pod) []corev1.Container {
	return append(append([]corev1.Container{}, p.Spec.InitContainers...), p.Spec.Containers...)
}

// cisControls is the curated, API-derivable control set.
func cisControls() []cisControl {
	return []cisControl{
		{
			"CIS-5.1.1", "Ensure cluster-admin role is only used where required",
			"5.1 RBAC and Service Accounts",
			func(d complianceData) []complianceViolation {
				var v []complianceViolation
				for _, b := range d.crbs {
					if b.RoleRef.Name == "cluster-admin" && b.Name != "cluster-admin" {
						v = append(v, complianceViolation{
							Kind: "ClusterRoleBinding", Name: b.Name,
							Detail: "binds cluster-admin — review whether this is required",
						})
					}
				}
				return v
			},
		},
		{
			"CIS-5.1.3", "Minimize wildcard use in Roles and ClusterRoles",
			"5.1 RBAC and Service Accounts",
			func(d complianceData) []complianceViolation {
				var v []complianceViolation
				check := func(kind, ns, name string, rules []rbacv1.PolicyRule) {
					if strings.HasPrefix(name, "system:") || name == "cluster-admin" {
						return
					}
					for _, r := range rules {
						if contains(r.Verbs, "*") || contains(r.Resources, "*") || contains(r.APIGroups, "*") {
							v = append(v, complianceViolation{Namespace: ns, Kind: kind, Name: name,
								Detail: "uses a wildcard (*) verb/resource/apiGroup"})
							return
						}
					}
				}
				for _, c := range d.croles {
					check("ClusterRole", "", c.Name, c.Rules)
				}
				for _, r := range d.roles {
					check("Role", r.Namespace, r.Name, r.Rules)
				}
				return v
			},
		},
		podControl("CIS-5.2.1", "Minimize the admission of privileged containers", func(c corev1.Container) bool {
			return c.SecurityContext != nil && c.SecurityContext.Privileged != nil && *c.SecurityContext.Privileged
		}, "container runs privileged"),
		podSpecControl("CIS-5.2.2", "Minimize the admission of containers wishing to share the host process ID namespace",
			func(p corev1.Pod) bool { return p.Spec.HostPID }, "pod uses hostPID"),
		podSpecControl("CIS-5.2.3", "Minimize the admission of containers wishing to share the host IPC namespace",
			func(p corev1.Pod) bool { return p.Spec.HostIPC }, "pod uses hostIPC"),
		podSpecControl("CIS-5.2.4", "Minimize the admission of containers wishing to share the host network namespace",
			func(p corev1.Pod) bool { return p.Spec.HostNetwork }, "pod uses hostNetwork"),
		podControl("CIS-5.2.5", "Minimize the admission of containers with allowPrivilegeEscalation", func(c corev1.Container) bool {
			return c.SecurityContext == nil || c.SecurityContext.AllowPrivilegeEscalation == nil ||
				*c.SecurityContext.AllowPrivilegeEscalation
		}, "allowPrivilegeEscalation is not set to false"),
		podControl("CIS-5.2.6", "Minimize the admission of root containers", func(c corev1.Container) bool {
			sc := c.SecurityContext
			nonRoot := sc != nil && sc.RunAsNonRoot != nil && *sc.RunAsNonRoot
			rootUser := sc != nil && sc.RunAsUser != nil && *sc.RunAsUser == 0
			return !nonRoot || rootUser
		}, "does not enforce runAsNonRoot (may run as root)"),
		podControl("CIS-5.2.8", "Minimize the admission of containers with added capabilities", func(c corev1.Container) bool {
			return c.SecurityContext != nil && c.SecurityContext.Capabilities != nil &&
				len(c.SecurityContext.Capabilities.Add) > 0
		}, "adds Linux capabilities"),
		podControl("CIS-5.2.9", "Minimize the admission of containers with capabilities assigned (drop ALL)", func(c corev1.Container) bool {
			sc := c.SecurityContext
			if sc == nil || sc.Capabilities == nil {
				return true
			}
			for _, d := range sc.Capabilities.Drop {
				if d == "ALL" || d == "all" {
					return false
				}
			}
			return true
		}, "does not drop ALL capabilities"),
		{
			"CIS-5.3.2", "Ensure that all namespaces have Network Policies defined",
			"5.3 Network Policies and CNI",
			func(d complianceData) []complianceViolation {
				withNP := map[string]bool{}
				for _, np := range d.netpols {
					withNP[np.Namespace] = true
				}
				seen := map[string]bool{}
				var v []complianceViolation
				for _, p := range d.workloadPods() {
					if seen[p.Namespace] || withNP[p.Namespace] {
						continue
					}
					seen[p.Namespace] = true
					v = append(v, complianceViolation{Namespace: p.Namespace, Kind: "Namespace",
						Name: p.Namespace, Detail: "runs workloads but has no NetworkPolicy"})
				}
				return v
			},
		},
		{
			"CIS-5.7.4", "The default namespace should not be used",
			"5.7 General Policies",
			func(d complianceData) []complianceViolation {
				var v []complianceViolation
				for _, p := range d.pods {
					if p.Namespace == "default" {
						v = append(v, podViolation(p, "workload runs in the default namespace"))
					}
				}
				return v
			},
		},
	}
}

// podControl builds a Pod Security control that fails a workload pod when any of
// its containers matches bad().
func podControl(id, title string, bad func(corev1.Container) bool, detail string) cisControl {
	return cisControl{id, title, "5.2 Pod Security Standards", func(d complianceData) []complianceViolation {
		var v []complianceViolation
		for _, p := range d.workloadPods() {
			for _, c := range podContainers(p) {
				if bad(c) {
					v = append(v, podViolation(p, c.Name+": "+detail))
					break
				}
			}
		}
		return v
	}}
}

// podSpecControl builds a Pod Security control that fails on a pod-level setting.
func podSpecControl(id, title string, bad func(corev1.Pod) bool, detail string) cisControl {
	return cisControl{id, title, "5.2 Pod Security Standards", func(d complianceData) []complianceViolation {
		var v []complianceViolation
		for _, p := range d.workloadPods() {
			if bad(p) {
				v = append(v, podViolation(p, detail))
			}
		}
		return v
	}}
}

func contains(s []string, target string) bool {
	for _, x := range s {
		if x == target {
			return true
		}
	}

	return false
}
