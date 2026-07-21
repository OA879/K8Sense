package checks

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor"
)

func init() {
	clusterdoctor.RegisterCheck("check_coredns_not_running", checkSystemComponentNotRunning("k8s-app", "kube-dns", "CoreDNS"))
	clusterdoctor.RegisterCheck("check_kube_proxy_not_running", checkSystemComponentNotRunning("k8s-app", "kube-proxy", "kube-proxy"))
	clusterdoctor.RegisterCheck("check_service_no_endpoints", checkServiceNoEndpoints)
	clusterdoctor.RegisterCheck("check_ingress_no_address", checkIngressNoAddress)
}

// checkSystemComponentNotRunning returns a CheckFunc that flags the case
// where a critical kube-system component (selected by labelKey=labelValue,
// e.g. k8s-app=kube-dns) has zero Running pods. displayName is used only in
// the finding's raw payload.
func checkSystemComponentNotRunning(labelKey, labelValue, displayName string) clusterdoctor.CheckFunc {
	return func(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
		pods, err := clientset.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{
			LabelSelector: labelKey + "=" + labelValue,
		})
		if err != nil {
			return nil, err
		}

		running := 0

		for _, pod := range pods.Items {
			if pod.Status.Phase == corev1.PodRunning && podIsReady(pod) {
				running++
			}
		}

		if running == 0 {
			return []clusterdoctor.RawFinding{{
				Namespace:    "kube-system",
				ResourceKind: "DaemonSet",
				ResourceName: displayName,
				RawObject:    fmt.Sprintf(`{"component": %q, "runningPods": 0}`, displayName),
			}}, nil
		}

		return nil, nil
	}
}

// checkServiceNoEndpoints flags Services (with a selector, excluding headless
// and ExternalName) that have no ready backing endpoints — a classic
// selector-mismatch / all-backends-down symptom.
func checkServiceNoEndpoints(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	services, err := clientset.CoreV1().Services(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, svc := range services.Items {
		if svc.Spec.Type == corev1.ServiceTypeExternalName || len(svc.Spec.Selector) == 0 {
			continue
		}

		endpoints, err := clientset.CoreV1().Endpoints(svc.Namespace).Get(ctx, svc.Name, metav1.GetOptions{})
		if err != nil {
			continue // endpoint object may not exist yet; don't fail the scan
		}

		ready := 0
		for _, subset := range endpoints.Subsets {
			ready += len(subset.Addresses)
		}

		if ready == 0 {
			findings = append(findings, clusterdoctor.RawFinding{
				Namespace:    svc.Namespace,
				ResourceKind: "Service",
				ResourceName: svc.Name,
			})
		}
	}

	return findings, nil
}

func checkIngressNoAddress(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	ingresses, err := clientset.NetworkingV1().Ingresses(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, ing := range ingresses.Items {
		hasAddress := false

		for _, lb := range ing.Status.LoadBalancer.Ingress {
			if lb.IP != "" || lb.Hostname != "" {
				hasAddress = true
				break
			}
		}

		if !hasAddress {
			findings = append(findings, clusterdoctor.RawFinding{
				Namespace:    ing.Namespace,
				ResourceKind: "Ingress",
				ResourceName: ing.Name,
			})
		}
	}

	return findings, nil
}
