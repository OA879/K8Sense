package checks

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/OA879/K8Sense/backend/pkg/clusterdoctor"
)

const (
	pendingTooLongThreshold  = 10 * time.Minute
	restartCountThreshold    = 5
	oomKilledExitCode        = 137
)

func init() {
	clusterdoctor.RegisterCheck("check_crashloopbackoff", checkCrashLoopBackOff)
	clusterdoctor.RegisterCheck("check_oomkilled", checkOOMKilled)
	clusterdoctor.RegisterCheck("check_pending_too_long", checkPendingTooLong)
	clusterdoctor.RegisterCheck("check_evicted_pods", checkEvictedPods)
	clusterdoctor.RegisterCheck("check_image_pull_backoff", checkImagePullBackOff)
	clusterdoctor.RegisterCheck("check_init_container_stuck", checkInitContainerStuck)
	clusterdoctor.RegisterCheck("check_missing_resource_limits", checkMissingResourceLimits)
	clusterdoctor.RegisterCheck("check_missing_resource_requests", checkMissingResourceRequests)
	clusterdoctor.RegisterCheck("check_pod_stuck_terminating", checkPodStuckTerminating)
	clusterdoctor.RegisterCheck("check_pod_running_as_root", checkPodRunningAsRoot)
	clusterdoctor.RegisterCheck("check_pod_no_readiness_probe", checkPodNoReadinessProbe)
	clusterdoctor.RegisterCheck("check_pod_frequent_restarts", checkPodFrequentRestarts)
}

// listNonSucceededPods returns every pod that isn't in the terminal
// Succeeded phase, since completed batch work is never a finding (see
// K8SENSE_CONTEXT.md "Always skip automatically").
func listNonSucceededPods(ctx context.Context, clientset kubernetes.Interface) ([]corev1.Pod, error) {
	pods, err := clientset.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var out []corev1.Pod

	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodSucceeded {
			continue
		}

		out = append(out, pod)
	}

	return out, nil
}

func podFinding(pod corev1.Pod, extra string) clusterdoctor.RawFinding {
	return clusterdoctor.RawFinding{
		Namespace:    pod.Namespace,
		ResourceKind: "Pod",
		ResourceName: pod.Name,
		RawObject:    extra,
	}
}

func checkCrashLoopBackOff(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	pods, err := listNonSucceededPods(ctx, clientset)
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, pod := range pods {
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
				findings = append(findings, podFinding(pod, fmt.Sprintf(`{"container": %q}`, cs.Name)))

				break
			}
		}
	}

	return findings, nil
}

func checkOOMKilled(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	pods, err := listNonSucceededPods(ctx, clientset)
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, pod := range pods {
		for _, cs := range pod.Status.ContainerStatuses {
			term := cs.LastTerminationState.Terminated
			if term != nil && term.ExitCode == oomKilledExitCode {
				findings = append(findings, podFinding(pod, fmt.Sprintf(`{"container": %q}`, cs.Name)))

				break
			}
		}
	}

	return findings, nil
}

func checkPendingTooLong(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	pods, err := listNonSucceededPods(ctx, clientset)
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, pod := range pods {
		if pod.Status.Phase != corev1.PodPending {
			continue
		}

		if time.Since(pod.CreationTimestamp.Time) > pendingTooLongThreshold {
			findings = append(findings, podFinding(pod, ""))
		}
	}

	return findings, nil
}

func checkEvictedPods(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	// Evicted pods are Failed phase with a specific status.reason; they are
	// deliberately not filtered out by listNonSucceededPods since Failed !=
	// Succeeded, but we still want every namespace, not just non-terminal.
	pods, err := clientset.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodFailed && pod.Status.Reason == "Evicted" {
			findings = append(findings, podFinding(pod, ""))
		}
	}

	return findings, nil
}

func checkImagePullBackOff(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	pods, err := listNonSucceededPods(ctx, clientset)
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, pod := range pods {
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Waiting == nil {
				continue
			}

			reason := cs.State.Waiting.Reason
			if reason == "ImagePullBackOff" || reason == "ErrImagePull" {
				findings = append(findings, podFinding(pod, fmt.Sprintf(`{"container": %q, "reason": %q}`, cs.Name, reason)))

				break
			}
		}
	}

	return findings, nil
}

const initContainerStuckThreshold = 10 * time.Minute

func checkInitContainerStuck(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	pods, err := listNonSucceededPods(ctx, clientset)
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, pod := range pods {
		if len(pod.Spec.InitContainers) == 0 {
			continue
		}

		if time.Since(pod.CreationTimestamp.Time) < initContainerStuckThreshold {
			continue
		}

		for _, cs := range pod.Status.InitContainerStatuses {
			if cs.State.Waiting != nil || (cs.State.Running != nil && !cs.Ready) {
				findings = append(findings, podFinding(pod, fmt.Sprintf(`{"initContainer": %q}`, cs.Name)))

				break
			}
		}
	}

	return findings, nil
}

func checkMissingResourceLimits(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	pods, err := listNonSucceededPods(ctx, clientset)
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, pod := range pods {
		for _, c := range pod.Spec.Containers {
			if len(c.Resources.Limits) == 0 {
				findings = append(findings, podFinding(pod, fmt.Sprintf(`{"container": %q}`, c.Name)))

				break
			}
		}
	}

	return findings, nil
}

func checkMissingResourceRequests(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	pods, err := listNonSucceededPods(ctx, clientset)
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, pod := range pods {
		for _, c := range pod.Spec.Containers {
			if len(c.Resources.Requests) == 0 {
				findings = append(findings, podFinding(pod, fmt.Sprintf(`{"container": %q}`, c.Name)))

				break
			}
		}
	}

	return findings, nil
}

func checkPodStuckTerminating(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	pods, err := clientset.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, pod := range pods.Items {
		if pod.DeletionTimestamp == nil {
			continue
		}

		if time.Since(pod.DeletionTimestamp.Time) > initContainerStuckThreshold {
			findings = append(findings, podFinding(pod, ""))
		}
	}

	return findings, nil
}

func checkPodRunningAsRoot(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	pods, err := listNonSucceededPods(ctx, clientset)
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, pod := range pods {
		if runsAsRoot(pod) {
			findings = append(findings, podFinding(pod, ""))
		}
	}

	return findings, nil
}

// runsAsRoot reports whether any container in pod is explicitly allowed to
// run as UID 0, either via an explicit RunAsUser: 0 or by not setting
// RunAsNonRoot anywhere (pod or container level) with no user set at all
// (the image's own default, which is root more often than not).
func runsAsRoot(pod corev1.Pod) bool {
	podLevelNonRoot := pod.Spec.SecurityContext != nil && pod.Spec.SecurityContext.RunAsNonRoot != nil &&
		*pod.Spec.SecurityContext.RunAsNonRoot

	podLevelUser := pod.Spec.SecurityContext != nil && pod.Spec.SecurityContext.RunAsUser != nil &&
		*pod.Spec.SecurityContext.RunAsUser == 0

	if podLevelUser {
		return true
	}

	for _, c := range pod.Spec.Containers {
		if c.SecurityContext != nil && c.SecurityContext.RunAsUser != nil {
			if *c.SecurityContext.RunAsUser == 0 {
				return true
			}

			continue // explicit non-zero UID set at container level
		}

		if c.SecurityContext != nil && c.SecurityContext.RunAsNonRoot != nil && *c.SecurityContext.RunAsNonRoot {
			continue
		}

		if podLevelNonRoot {
			continue
		}

		// Neither pod nor container opted out of root — the image's default
		// user (frequently root) applies.
		return true
	}

	return false
}

func checkPodNoReadinessProbe(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	pods, err := listNonSucceededPods(ctx, clientset)
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, pod := range pods {
		for _, c := range pod.Spec.Containers {
			if c.ReadinessProbe == nil {
				findings = append(findings, podFinding(pod, fmt.Sprintf(`{"container": %q}`, c.Name)))

				break
			}
		}
	}

	return findings, nil
}

func checkPodFrequentRestarts(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	pods, err := listNonSucceededPods(ctx, clientset)
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, pod := range pods {
		var total int32

		for _, cs := range pod.Status.ContainerStatuses {
			total += cs.RestartCount
		}

		if total > restartCountThreshold {
			findings = append(findings, podFinding(pod, fmt.Sprintf(`{"restartCount": %d}`, total)))
		}
	}

	return findings, nil
}
