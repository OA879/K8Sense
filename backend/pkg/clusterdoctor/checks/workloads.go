package checks

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/OA879/K8Sense/backend/pkg/clusterdoctor"
)

func init() {
	clusterdoctor.RegisterCheck("check_deployment_zero_available", checkDeploymentZeroAvailable)
	clusterdoctor.RegisterCheck("check_deployment_below_desired", checkDeploymentBelowDesired)
	clusterdoctor.RegisterCheck("check_daemonset_not_fully_scheduled", checkDaemonSetNotFullyScheduled)
	clusterdoctor.RegisterCheck("check_statefulset_not_all_running", checkStatefulSetNotAllRunning)
	clusterdoctor.RegisterCheck("check_job_failed", checkJobFailed)
	clusterdoctor.RegisterCheck("check_single_replica_deployment", checkSingleReplicaDeployment)
}

func desiredReplicas(spec *int32) int32 {
	if spec == nil {
		return 1
	}

	return *spec
}

func checkDeploymentZeroAvailable(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	deployments, err := clientset.AppsV1().Deployments(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, dep := range deployments.Items {
		desired := desiredReplicas(dep.Spec.Replicas)
		if desired > 0 && dep.Status.AvailableReplicas == 0 {
			findings = append(findings, clusterdoctor.RawFinding{
				Namespace:    dep.Namespace,
				ResourceKind: "Deployment",
				ResourceName: dep.Name,
				RawObject:    fmt.Sprintf(`{"desired": %d, "available": 0}`, desired),
			})
		}
	}

	return findings, nil
}

func checkDeploymentBelowDesired(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	deployments, err := clientset.AppsV1().Deployments(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, dep := range deployments.Items {
		desired := desiredReplicas(dep.Spec.Replicas)
		// Exclude the zero-available case — that's the more severe WL-001.
		if desired > 0 && dep.Status.AvailableReplicas > 0 && dep.Status.AvailableReplicas < desired {
			findings = append(findings, clusterdoctor.RawFinding{
				Namespace:    dep.Namespace,
				ResourceKind: "Deployment",
				ResourceName: dep.Name,
				RawObject: fmt.Sprintf(
					`{"desired": %d, "available": %d}`, desired, dep.Status.AvailableReplicas,
				),
			})
		}
	}

	return findings, nil
}

func checkDaemonSetNotFullyScheduled(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	daemonsets, err := clientset.AppsV1().DaemonSets(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, ds := range daemonsets.Items {
		if ds.Status.NumberReady < ds.Status.DesiredNumberScheduled {
			findings = append(findings, clusterdoctor.RawFinding{
				Namespace:    ds.Namespace,
				ResourceKind: "DaemonSet",
				ResourceName: ds.Name,
				RawObject: fmt.Sprintf(
					`{"desired": %d, "ready": %d}`,
					ds.Status.DesiredNumberScheduled, ds.Status.NumberReady,
				),
			})
		}
	}

	return findings, nil
}

func checkStatefulSetNotAllRunning(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	statefulsets, err := clientset.AppsV1().StatefulSets(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, ss := range statefulsets.Items {
		desired := desiredReplicas(ss.Spec.Replicas)
		if desired > 0 && ss.Status.ReadyReplicas < desired {
			findings = append(findings, clusterdoctor.RawFinding{
				Namespace:    ss.Namespace,
				ResourceKind: "StatefulSet",
				ResourceName: ss.Name,
				RawObject: fmt.Sprintf(
					`{"desired": %d, "ready": %d}`, desired, ss.Status.ReadyReplicas,
				),
			})
		}
	}

	return findings, nil
}

func checkJobFailed(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	jobs, err := clientset.BatchV1().Jobs(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, job := range jobs.Items {
		if job.Status.Failed > 0 {
			findings = append(findings, clusterdoctor.RawFinding{
				Namespace:    job.Namespace,
				ResourceKind: "Job",
				ResourceName: job.Name,
				RawObject:    fmt.Sprintf(`{"failed": %d}`, job.Status.Failed),
			})
		}
	}

	return findings, nil
}

// checkSingleReplicaDeployment is an INFO nudge: a Deployment with exactly one
// replica has no high-availability. Excludes 0-replica (paused/disabled)
// deployments, which aren't an HA concern.
func checkSingleReplicaDeployment(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	deployments, err := clientset.AppsV1().Deployments(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, dep := range deployments.Items {
		if desiredReplicas(dep.Spec.Replicas) == 1 {
			findings = append(findings, clusterdoctor.RawFinding{
				Namespace:    dep.Namespace,
				ResourceKind: "Deployment",
				ResourceName: dep.Name,
			})
		}
	}

	return findings, nil
}
