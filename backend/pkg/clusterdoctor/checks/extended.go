package checks

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"

	"github.com/OA879/K8Sense/backend/pkg/clusterdoctor"
)

// This file holds the "second wave" of checks that fill out the planned rule
// catalogue. They all work with the standard client-go clientset — no metrics
// server, Prometheus scrape, or dynamic client — so they behave the same on
// kind, self-managed and managed clusters (finding nothing where the relevant
// object doesn't exist rather than erroring).

// recentEventWindow bounds how far back an event counts as "current". The API
// server garbage-collects events after ~1h by default, so anything within this
// window is effectively live.
const recentEventWindow = time.Hour

// livenessFailThreshold is how many times a liveness probe must have failed
// before POD-007 reports it — one transient failure isn't worth paging on.
const livenessFailThreshold = 3

// leaseStaleAfter is how old a control-plane leader Lease's renewTime may be
// before we treat leadership as lost. Kube components renew every ~2-10s, so a
// minute of silence is a strong signal.
const leaseStaleAfter = time.Minute

// cniDaemonSets are the well-known CNI DaemonSet name prefixes across common
// providers. A CNI whose pods aren't all ready breaks pod networking cluster-wide.
var cniDaemonSets = []string{
	"kindnet", "calico-node", "cilium", "kube-flannel", "aws-node", "weave-net", "canal",
}

// systemNamespaces are skipped by namespace-hygiene checks (RES-003): the
// control plane manages its own namespaces and doesn't need a LimitRange.
var systemNamespaces = map[string]bool{
	"kube-system": true, "kube-public": true, "kube-node-lease": true,
	"local-path-storage": true,
}

func init() {
	clusterdoctor.RegisterCheck("check_liveness_probe_failing", checkLivenessProbeFailing)
	clusterdoctor.RegisterCheck("check_scheduler_not_leader", checkSchedulerNotLeader)
	clusterdoctor.RegisterCheck("check_controller_manager_not_leader", checkControllerManagerNotLeader)
	clusterdoctor.RegisterCheck("check_volume_mount_errors", checkVolumeMountErrors)
	clusterdoctor.RegisterCheck("check_cni_daemonset_not_available", checkCNIDaemonSetNotAvailable)
	clusterdoctor.RegisterCheck("check_nodeport_conflicts", checkNodePortConflicts)
	clusterdoctor.RegisterCheck("check_limitrange_missing", checkLimitRangeMissing)
	clusterdoctor.RegisterCheck("check_ingress_cert_expiring", checkIngressCert(false))
	clusterdoctor.RegisterCheck("check_ingress_cert_expired", checkIngressCert(true))
	clusterdoctor.RegisterCheck("check_deployment_stalled_rollout", checkDeploymentStalledRollout)
	clusterdoctor.RegisterCheck("check_cronjob_overdue", checkCronJobOverdue)
	clusterdoctor.RegisterCheck("check_deployment_no_pdb", checkDeploymentNoPDB)
}

// recentWarningEvents lists Warning events seen within recentEventWindow whose
// Reason is in reasons. Shared by the event-driven checks.
func recentWarningEvents(
	ctx context.Context, clientset kubernetes.Interface, reasons map[string]bool,
) ([]corev1.Event, error) {
	events, err := clientset.CoreV1().Events(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var out []corev1.Event

	for _, ev := range events.Items {
		if ev.Type != corev1.EventTypeWarning || !reasons[ev.Reason] {
			continue
		}

		last := ev.LastTimestamp.Time
		if last.IsZero() {
			last = ev.EventTime.Time
		}

		if !last.IsZero() && time.Since(last) > recentEventWindow {
			continue
		}

		out = append(out, ev)
	}

	return out, nil
}

// POD-007 — a liveness probe failing repeatedly. Shows up as repeated
// "Unhealthy" Warning events; we require a few failures to avoid flapping.
func checkLivenessProbeFailing(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	events, err := recentWarningEvents(ctx, clientset, map[string]bool{"Unhealthy": true})
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}

	var findings []clusterdoctor.RawFinding

	for _, ev := range events {
		if !strings.Contains(ev.Message, "Liveness probe failed") || ev.Count < livenessFailThreshold {
			continue
		}

		key := ev.InvolvedObject.Namespace + "/" + ev.InvolvedObject.Name
		if seen[key] {
			continue
		}

		seen[key] = true

		findings = append(findings, clusterdoctor.RawFinding{
			Namespace:    ev.InvolvedObject.Namespace,
			ResourceKind: "Pod",
			ResourceName: ev.InvolvedObject.Name,
			RawObject:    fmt.Sprintf(`{"failures": %d}`, ev.Count),
		})
	}

	return findings, nil
}

// STOR-004 — a pod can't mount its volume (FailedMount / FailedAttachVolume).
func checkVolumeMountErrors(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	events, err := recentWarningEvents(ctx, clientset,
		map[string]bool{"FailedMount": true, "FailedAttachVolume": true})
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}

	var findings []clusterdoctor.RawFinding

	for _, ev := range events {
		key := ev.InvolvedObject.Namespace + "/" + ev.InvolvedObject.Name
		if seen[key] {
			continue
		}

		seen[key] = true

		findings = append(findings, clusterdoctor.RawFinding{
			Namespace:    ev.InvolvedObject.Namespace,
			ResourceKind: ev.InvolvedObject.Kind,
			ResourceName: ev.InvolvedObject.Name,
			RawObject:    fmt.Sprintf(`{"reason": %q}`, ev.Reason),
		})
	}

	return findings, nil
}

// leaderNotHeld reports a finding if the named control-plane Lease exists but
// has no fresh holder. A missing Lease means the component isn't managed as a
// leader-elected pod here (e.g. a managed control plane) — that's not a
// finding, so it returns (nil, nil).
func leaderNotHeld(ctx context.Context, clientset kubernetes.Interface, leaseName, component string) ([]clusterdoctor.RawFinding, error) {
	lease, err := clientset.CoordinationV1().Leases("kube-system").Get(ctx, leaseName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	holder := ""
	if lease.Spec.HolderIdentity != nil {
		holder = *lease.Spec.HolderIdentity
	}

	stale := lease.Spec.RenewTime == nil || time.Since(lease.Spec.RenewTime.Time) > leaseStaleAfter

	if holder == "" || stale {
		return []clusterdoctor.RawFinding{{
			Namespace:    "kube-system",
			ResourceKind: "Lease",
			ResourceName: leaseName,
			RawObject:    fmt.Sprintf(`{"component": %q, "holder": %q, "stale": %t}`, component, holder, stale),
		}}, nil
	}

	return nil, nil
}

// CP-003 — the scheduler holds no fresh leader lease (no active scheduler).
func checkSchedulerNotLeader(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	return leaderNotHeld(ctx, clientset, "kube-scheduler", "kube-scheduler")
}

// CP-004 — the controller-manager holds no fresh leader lease.
func checkControllerManagerNotLeader(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	return leaderNotHeld(ctx, clientset, "kube-controller-manager", "kube-controller-manager")
}

// NET-003 — a CNI DaemonSet has pods that aren't all ready, so pod networking
// is degraded or down on some nodes.
func checkCNIDaemonSetNotAvailable(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	daemonsets, err := clientset.AppsV1().DaemonSets("kube-system").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, ds := range daemonsets.Items {
		isCNI := false

		for _, prefix := range cniDaemonSets {
			if strings.HasPrefix(ds.Name, prefix) {
				isCNI = true
				break
			}
		}

		if isCNI && ds.Status.NumberReady < ds.Status.DesiredNumberScheduled {
			findings = append(findings, clusterdoctor.RawFinding{
				Namespace:    ds.Namespace,
				ResourceKind: "DaemonSet",
				ResourceName: ds.Name,
				RawObject: fmt.Sprintf(`{"desired": %d, "ready": %d}`,
					ds.Status.DesiredNumberScheduled, ds.Status.NumberReady),
			})
		}
	}

	return findings, nil
}

// NET-006 — two Services claim the same NodePort. The API server normally
// rejects this at creation, but a restored/edited object can still collide.
func checkNodePortConflicts(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	services, err := clientset.CoreV1().Services(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	type svcRef struct{ ns, name string }

	byPort := map[int32][]svcRef{}

	for _, svc := range services.Items {
		for _, port := range svc.Spec.Ports {
			if port.NodePort != 0 {
				byPort[port.NodePort] = append(byPort[port.NodePort], svcRef{svc.Namespace, svc.Name})
			}
		}
	}

	var findings []clusterdoctor.RawFinding

	for nodePort, refs := range byPort {
		if len(refs) < 2 {
			continue
		}

		for _, ref := range refs {
			findings = append(findings, clusterdoctor.RawFinding{
				Namespace:    ref.ns,
				ResourceKind: "Service",
				ResourceName: ref.name,
				RawObject:    fmt.Sprintf(`{"nodePort": %d, "conflicts": %d}`, nodePort, len(refs)),
			})
		}
	}

	return findings, nil
}

// RES-003 — a non-system namespace has no LimitRange, so pods there can run
// with no default/ceiling on resources.
func checkLimitRangeMissing(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	namespaces, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	ranges, err := clientset.CoreV1().LimitRanges(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	hasRange := map[string]bool{}
	for _, lr := range ranges.Items {
		hasRange[lr.Namespace] = true
	}

	var findings []clusterdoctor.RawFinding

	for _, ns := range namespaces.Items {
		if systemNamespaces[ns.Name] || ns.Status.Phase == corev1.NamespaceTerminating {
			continue
		}

		if !hasRange[ns.Name] {
			findings = append(findings, clusterdoctor.RawFinding{
				ResourceKind: "Namespace",
				ResourceName: ns.Name,
			})
		}
	}

	return findings, nil
}

// checkIngressCert flags Ingress TLS whose backing certificate is expiring
// (CERT-003) or already expired (CERT-004), reusing the same public-cert-only
// parsing as the Secret checks.
func checkIngressCert(expiredOnly bool) clusterdoctor.CheckFunc {
	return func(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
		ingresses, err := clientset.NetworkingV1().Ingresses(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		now := time.Now()

		var findings []clusterdoctor.RawFinding

		for _, ing := range ingresses.Items {
			for _, tls := range ing.Spec.TLS {
				if tls.SecretName == "" {
					continue
				}

				secret, err := clientset.CoreV1().Secrets(ing.Namespace).Get(ctx, tls.SecretName, metav1.GetOptions{})
				if err != nil {
					continue // secret missing/unreadable — not this rule's concern
				}

				cert := parseLeafCert(secret.Data["tls.crt"])
				if cert == nil {
					continue
				}

				expired := now.After(cert.NotAfter)
				expiringSoon := !expired && cert.NotAfter.Sub(now) < certExpiryWarnWindow

				if (expiredOnly && expired) || (!expiredOnly && expiringSoon) {
					findings = append(findings, clusterdoctor.RawFinding{
						Namespace:    ing.Namespace,
						ResourceKind: "Ingress",
						ResourceName: ing.Name,
						RawObject: fmt.Sprintf(`{"secret": %q, "notAfter": %q}`,
							tls.SecretName, cert.NotAfter.Format(time.RFC3339)),
					})

					break // one finding per Ingress is enough
				}
			}
		}

		return findings, nil
	}
}

// WL-002 — a Deployment's rollout has stalled (ProgressDeadlineExceeded).
func checkDeploymentStalledRollout(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	deployments, err := clientset.AppsV1().Deployments(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, dep := range deployments.Items {
		for _, cond := range dep.Status.Conditions {
			if cond.Type == "Progressing" &&
				cond.Status == corev1.ConditionFalse &&
				cond.Reason == "ProgressDeadlineExceeded" {
				findings = append(findings, clusterdoctor.RawFinding{
					Namespace:    dep.Namespace,
					ResourceKind: "Deployment",
					ResourceName: dep.Name,
					RawObject:    fmt.Sprintf(`{"reason": %q}`, cond.Reason),
				})

				break
			}
		}
	}

	return findings, nil
}

// cronIntervalMinutes gives a best-effort period, in minutes, for a standard
// 5-field cron schedule. It's intentionally conservative: anything it can't
// confidently narrow defaults to daily, so CronJobOverdue never false-positives
// on an unusual schedule.
func cronIntervalMinutes(schedule string) int {
	const dailyMinutes = 1440

	fields := strings.Fields(schedule)
	if len(fields) != 5 { //nolint:mnd // a standard cron has 5 fields
		return dailyMinutes
	}

	minute, hour := fields[0], fields[1]

	if strings.HasPrefix(minute, "*/") {
		if n, err := strconv.Atoi(strings.TrimPrefix(minute, "*/")); err == nil && n > 0 {
			return n
		}
	}

	if minute == "*" {
		return 1
	}

	// Fixed minute — cadence is governed by the hour field.
	if hour == "*" {
		return 60 //nolint:mnd // once per hour
	}

	if strings.HasPrefix(hour, "*/") {
		if n, err := strconv.Atoi(strings.TrimPrefix(hour, "*/")); err == nil && n > 0 {
			return n * 60 //nolint:mnd // every n hours
		}
	}

	return dailyMinutes
}

// WL-007 — a CronJob hasn't run in more than twice its schedule interval,
// suggesting it's stuck or its controller isn't scheduling it.
func checkCronJobOverdue(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	cronjobs, err := clientset.BatchV1().CronJobs(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, cj := range cronjobs.Items {
		if cj.Spec.Suspend != nil && *cj.Spec.Suspend {
			continue
		}

		if cj.Status.LastScheduleTime == nil {
			continue // never ran yet — not overdue in the sense this rule means
		}

		interval := time.Duration(cronIntervalMinutes(cj.Spec.Schedule)) * time.Minute
		if time.Since(cj.Status.LastScheduleTime.Time) > 2*interval {
			findings = append(findings, clusterdoctor.RawFinding{
				Namespace:    cj.Namespace,
				ResourceKind: "CronJob",
				ResourceName: cj.Name,
				RawObject: fmt.Sprintf(`{"schedule": %q, "lastRun": %q}`,
					cj.Spec.Schedule, cj.Status.LastScheduleTime.Format(time.RFC3339)),
			})
		}
	}

	return findings, nil
}

// WL-008 — a multi-replica Deployment has no PodDisruptionBudget, so a node
// drain / voluntary disruption can take all its pods down at once.
func checkDeploymentNoPDB(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	deployments, err := clientset.AppsV1().Deployments(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	pdbs, err := clientset.PolicyV1().PodDisruptionBudgets(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, dep := range deployments.Items {
		if desiredReplicas(dep.Spec.Replicas) < 2 { //nolint:mnd // single-replica has no HA to protect (WL-009)
			continue
		}

		podLabels := labels.Set(dep.Spec.Template.Labels)
		covered := false

		for _, pdb := range pdbs.Items {
			if pdb.Namespace != dep.Namespace || pdb.Spec.Selector == nil {
				continue
			}

			sel, err := metav1.LabelSelectorAsSelector(pdb.Spec.Selector)
			if err != nil {
				continue
			}

			if !sel.Empty() && sel.Matches(podLabels) {
				covered = true
				break
			}
		}

		if !covered {
			findings = append(findings, clusterdoctor.RawFinding{
				Namespace:    dep.Namespace,
				ResourceKind: "Deployment",
				ResourceName: dep.Name,
			})
		}
	}

	return findings, nil
}
