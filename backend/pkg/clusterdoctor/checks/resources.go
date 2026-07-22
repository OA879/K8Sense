package checks

import (
	"context"
	"fmt"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/OA879/K8Sense/backend/pkg/clusterdoctor"
)

const (
	quotaWarnPercent     = 85
	quotaCriticalPercent = 95
)

func init() {
	clusterdoctor.RegisterCheck("check_resourcequota_warn", checkResourceQuota(quotaWarnPercent, quotaCriticalPercent))
	clusterdoctor.RegisterCheck("check_resourcequota_critical", checkResourceQuota(quotaCriticalPercent, 101))
	clusterdoctor.RegisterCheck("check_hpa_cannot_compute", checkHPACannotComputeMetrics)
	clusterdoctor.RegisterCheck("check_hpa_at_max", checkHPAAtMaxReplicas)
}

// checkResourceQuota flags namespaces where any quota'd resource's usage is at
// or above minPercent but below maxPercent of its hard limit. The two-band
// design lets RES-001 (85-95%, WARNING) and RES-002 (>=95%, CRITICAL) share
// one implementation without double-reporting the same resource.
func checkResourceQuota(minPercent, maxPercent int64) clusterdoctor.CheckFunc {
	return func(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
		quotas, err := clientset.CoreV1().ResourceQuotas(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		var findings []clusterdoctor.RawFinding

		for _, quota := range quotas.Items {
			worst := worstQuotaUsagePercent(quota.Status.Hard, quota.Status.Used)
			if worst >= minPercent && worst < maxPercent {
				findings = append(findings, clusterdoctor.RawFinding{
					Namespace:    quota.Namespace,
					ResourceKind: "ResourceQuota",
					ResourceName: quota.Name,
					RawObject:    fmt.Sprintf(`{"usagePercent": %d}`, worst),
				})
			}
		}

		return findings, nil
	}
}

// worstQuotaUsagePercent returns the highest used/hard ratio (as a percent)
// across every resource in a quota, or -1 if the quota tracks nothing yet.
func worstQuotaUsagePercent(hard, used corev1.ResourceList) int64 {
	worst := int64(-1)

	for name, hardQty := range hard {
		if hardQty.IsZero() {
			continue
		}

		usedQty, ok := used[name]
		if !ok {
			continue
		}

		percent := ratioPercent(usedQty, hardQty)
		if percent > worst {
			worst = percent
		}
	}

	return worst
}

func ratioPercent(used, hard resource.Quantity) int64 {
	h := hard.MilliValue()
	if h == 0 {
		return 0
	}

	return used.MilliValue() * 100 / h
}

func checkHPACannotComputeMetrics(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	hpas, err := clientset.AutoscalingV2().HorizontalPodAutoscalers(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, hpa := range hpas.Items {
		for _, cond := range hpa.Status.Conditions {
			if cond.Type == autoscalingv2.ScalingActive && cond.Status == "False" {
				findings = append(findings, clusterdoctor.RawFinding{
					Namespace:    hpa.Namespace,
					ResourceKind: "HorizontalPodAutoscaler",
					ResourceName: hpa.Name,
					RawObject:    fmt.Sprintf(`{"reason": %q}`, cond.Reason),
				})

				break
			}
		}
	}

	return findings, nil
}

func checkHPAAtMaxReplicas(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	hpas, err := clientset.AutoscalingV2().HorizontalPodAutoscalers(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, hpa := range hpas.Items {
		if hpa.Status.CurrentReplicas > 0 && hpa.Status.CurrentReplicas == hpa.Spec.MaxReplicas {
			findings = append(findings, clusterdoctor.RawFinding{
				Namespace:    hpa.Namespace,
				ResourceKind: "HorizontalPodAutoscaler",
				ResourceName: hpa.Name,
				RawObject:    fmt.Sprintf(`{"maxReplicas": %d}`, hpa.Spec.MaxReplicas),
			})
		}
	}

	return findings, nil
}
