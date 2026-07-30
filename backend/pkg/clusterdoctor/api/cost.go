package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Cost & waste finder. Everything here is derived from the plain Kubernetes API
// — no cloud billing integration — so it works on any cluster, in desktop or
// hosted mode. It surfaces spend that is provably wasted (a cloud load balancer
// with nothing behind it, storage nobody mounts) plus how much of the cluster
// you are reserving, and puts a rough dollar figure on it using conservative,
// clearly-labelled unit estimates.

const (
	// Conservative monthly unit-cost estimates (USD). Rough on-demand cloud
	// averages — deliberately labelled as estimates in the UI; a later revision
	// can make these configurable / provider-specific.
	lbMonthlyUSD        = 18.0 // a managed load balancer's base charge
	storageGBMonthlyUSD = 0.10 // per provisioned GiB
)

// costItem is one wasteful thing with its estimated monthly cost.
type costItem struct {
	Category      string  `json:"category"` // idle-loadbalancer | unused-pvc | orphaned-pv
	Namespace     string  `json:"namespace,omitempty"`
	ResourceKind  string  `json:"resourceKind"`
	ResourceName  string  `json:"resourceName"`
	Detail        string  `json:"detail"`
	EstMonthlyUSD float64 `json:"estMonthlyUsd"`
}

// costReport is the whole picture: cluster reservation + waste line items.
type costReport struct {
	// Cluster reservation (requests vs allocatable), so over-provisioning is visible.
	CPURequestedMilli   int64 `json:"cpuRequestedMilli"`
	CPUAllocatableMilli int64 `json:"cpuAllocatableMilli"`
	MemRequestedBytes   int64 `json:"memRequestedBytes"`
	MemAllocatableBytes int64 `json:"memAllocatableBytes"`

	EstMonthlyWasteUSD float64    `json:"estMonthlyWasteUsd"`
	Items              []costItem `json:"items"`
}

// Cost handles GET /cluster-doctor/cost?cluster=&namespace= .
func (s *Server) Cost(w http.ResponseWriter, r *http.Request) {
	cluster := r.URL.Query().Get("cluster")

	clientset, err := s.getClient(r, cluster)
	if err != nil {
		http.Error(w, `{"error": "cluster not found"}`, http.StatusNotFound)
		return
	}

	report, err := buildCostReport(r.Context(), clientset, r.URL.Query().Get("namespace"))
	if err != nil {
		http.Error(w, `{"error": "could not build cost report"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(report)
}

// buildCostReport assembles cluster reservation and the waste line items.
func buildCostReport(ctx context.Context, clientset kubernetes.Interface, namespace string) (costReport, error) {
	ns := namespace
	if ns == "" {
		ns = metav1.NamespaceAll
	}

	report := costReport{Items: []costItem{}}

	// Cluster reservation is cluster-wide regardless of a namespace filter.
	report.CPUAllocatableMilli, report.MemAllocatableBytes = nodeAllocatable(ctx, clientset)
	report.CPURequestedMilli, report.MemRequestedBytes = podRequests(ctx, clientset, ns)

	idle, err := idleLoadBalancers(ctx, clientset, ns)
	if err != nil {
		return costReport{}, err
	}

	report.Items = append(report.Items, idle...)

	pvc, err := unusedPVCs(ctx, clientset, ns)
	if err != nil {
		return costReport{}, err
	}

	report.Items = append(report.Items, pvc...)

	// Orphaned PVs are cluster-scoped (not namespaced).
	if namespace == "" {
		orphans, err := orphanedPVs(ctx, clientset)
		if err != nil {
			return costReport{}, err
		}

		report.Items = append(report.Items, orphans...)
	}

	for _, it := range report.Items {
		report.EstMonthlyWasteUSD += it.EstMonthlyUSD
	}

	sort.Slice(report.Items, func(i, j int) bool {
		return report.Items[i].EstMonthlyUSD > report.Items[j].EstMonthlyUSD
	})

	return report, nil
}

// nodeAllocatable sums allocatable CPU (milli) and memory (bytes) across nodes.
func nodeAllocatable(ctx context.Context, clientset kubernetes.Interface) (cpuMilli, memBytes int64) {
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, 0
	}

	for _, n := range nodes.Items {
		cpuMilli += n.Status.Allocatable.Cpu().MilliValue()
		memBytes += n.Status.Allocatable.Memory().Value()
	}

	return cpuMilli, memBytes
}

// podRequests sums CPU (milli) and memory (bytes) requests across non-terminal pods.
func podRequests(ctx context.Context, clientset kubernetes.Interface, ns string) (cpuMilli, memBytes int64) {
	pods, err := clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, 0
	}

	for _, p := range pods.Items {
		if p.Status.Phase == corev1.PodSucceeded || p.Status.Phase == corev1.PodFailed {
			continue
		}

		for _, c := range p.Spec.Containers {
			cpuMilli += c.Resources.Requests.Cpu().MilliValue()
			memBytes += c.Resources.Requests.Memory().Value()
		}
	}

	return cpuMilli, memBytes
}

// idleLoadBalancers finds LoadBalancer Services with no ready endpoints — a
// cloud LB is billed whether or not anything is behind it.
func idleLoadBalancers(ctx context.Context, clientset kubernetes.Interface, ns string) ([]costItem, error) {
	svcs, err := clientset.CoreV1().Services(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var items []costItem

	for _, svc := range svcs.Items {
		if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
			continue
		}

		if endpointsReady(ctx, clientset, svc.Namespace, svc.Name) {
			continue
		}

		items = append(items, costItem{
			Category: "idle-loadbalancer", Namespace: svc.Namespace,
			ResourceKind: "Service", ResourceName: svc.Name,
			Detail:        "LoadBalancer with no ready endpoints — billed for nothing",
			EstMonthlyUSD: lbMonthlyUSD,
		})
	}

	return items, nil
}

// endpointsReady reports whether a Service currently has at least one ready
// endpoint address.
func endpointsReady(ctx context.Context, clientset kubernetes.Interface, ns, name string) bool {
	ep, err := clientset.CoreV1().Endpoints(ns).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false
	}

	if err != nil {
		return true // can't tell — don't flag on uncertainty
	}

	for _, sub := range ep.Subsets {
		if len(sub.Addresses) > 0 {
			return true
		}
	}

	return false
}

// unusedPVCs finds Bound PersistentVolumeClaims that no pod mounts — provisioned
// storage that is being paid for but not used.
func unusedPVCs(ctx context.Context, clientset kubernetes.Interface, ns string) ([]costItem, error) {
	pvcs, err := clientset.CoreV1().PersistentVolumeClaims(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	pods, err := clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	mounted := mountedPVCs(pods.Items)

	var items []costItem

	for _, pvc := range pvcs.Items {
		if pvc.Status.Phase != corev1.ClaimBound {
			continue
		}

		if mounted[pvc.Namespace+"/"+pvc.Name] {
			continue
		}

		gib, capStr := storageGiB(pvcCapacity(pvc))
		items = append(items, costItem{
			Category: "unused-pvc", Namespace: pvc.Namespace,
			ResourceKind: "PersistentVolumeClaim", ResourceName: pvc.Name,
			Detail:        fmt.Sprintf("Bound but not mounted by any pod (%s)", capStr),
			EstMonthlyUSD: gib * storageGBMonthlyUSD,
		})
	}

	return items, nil
}

// mountedPVCs returns the set of "namespace/claim" mounted by any pod.
func mountedPVCs(pods []corev1.Pod) map[string]bool {
	mounted := map[string]bool{}

	for _, p := range pods {
		for _, v := range p.Spec.Volumes {
			if v.PersistentVolumeClaim != nil {
				mounted[p.Namespace+"/"+v.PersistentVolumeClaim.ClaimName] = true
			}
		}
	}

	return mounted
}

// orphanedPVs finds PersistentVolumes stuck Released or Available — storage
// that outlived its claim and is still provisioned.
func orphanedPVs(ctx context.Context, clientset kubernetes.Interface) ([]costItem, error) {
	pvs, err := clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var items []costItem

	for _, pv := range pvs.Items {
		if pv.Status.Phase != corev1.VolumeReleased && pv.Status.Phase != corev1.VolumeAvailable {
			continue
		}

		gib, capStr := storageGiB(pv.Spec.Capacity[corev1.ResourceStorage])
		items = append(items, costItem{
			Category: "orphaned-pv", ResourceKind: "PersistentVolume", ResourceName: pv.Name,
			Detail:        fmt.Sprintf("%s and still provisioned (%s)", pv.Status.Phase, capStr),
			EstMonthlyUSD: gib * storageGBMonthlyUSD,
		})
	}

	return items, nil
}

func pvcCapacity(pvc corev1.PersistentVolumeClaim) resource.Quantity {
	if q, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
		return q
	}

	return pvc.Spec.Resources.Requests[corev1.ResourceStorage]
}

// storageGiB converts a storage quantity to GiB and a human string.
func storageGiB(q resource.Quantity) (float64, string) {
	bytes := q.Value()
	gib := float64(bytes) / (1024 * 1024 * 1024)

	return gib, fmt.Sprintf("%.1f GiB", gib)
}
