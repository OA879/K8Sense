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
	clusterdoctor.RegisterCheck("check_pvc_pending", checkPVCPending)
	clusterdoctor.RegisterCheck("check_pv_released_or_failed", checkPVReleasedOrFailed)
	clusterdoctor.RegisterCheck("check_storageclass_no_provisioner", checkStorageClassNoProvisioner)
	clusterdoctor.RegisterCheck("check_pvc_using_default_storageclass", checkPVCUsingDefaultStorageClass)
}

func checkPVCPending(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	pvcs, err := clientset.CoreV1().PersistentVolumeClaims(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, pvc := range pvcs.Items {
		if pvc.Status.Phase == corev1.ClaimPending {
			sc := ""
			if pvc.Spec.StorageClassName != nil {
				sc = *pvc.Spec.StorageClassName
			}

			findings = append(findings, clusterdoctor.RawFinding{
				Namespace:    pvc.Namespace,
				ResourceKind: "PersistentVolumeClaim",
				ResourceName: pvc.Name,
				RawObject:    fmt.Sprintf(`{"storageClass": %q}`, sc),
			})
		}
	}

	return findings, nil
}

func checkPVReleasedOrFailed(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	pvs, err := clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, pv := range pvs.Items {
		if pv.Status.Phase == corev1.VolumeReleased || pv.Status.Phase == corev1.VolumeFailed {
			findings = append(findings, clusterdoctor.RawFinding{
				ResourceKind: "PersistentVolume",
				ResourceName: pv.Name,
				RawObject:    fmt.Sprintf(`{"phase": %q}`, pv.Status.Phase),
			})
		}
	}

	return findings, nil
}

func checkStorageClassNoProvisioner(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	classes, err := clientset.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, sc := range classes.Items {
		// "kubernetes.io/no-provisioner" is the sentinel used by static/local
		// volumes — a StorageClass with an empty provisioner, by contrast, can
		// never dynamically provision and almost always signals a misconfig.
		if sc.Provisioner == "" {
			findings = append(findings, clusterdoctor.RawFinding{
				ResourceKind: "StorageClass",
				ResourceName: sc.Name,
			})
		}
	}

	return findings, nil
}

// checkPVCUsingDefaultStorageClass flags PVCs that rely on the cluster's
// default StorageClass implicitly (no storageClassName set). This is an INFO
// nudge toward being explicit, not an error.
func checkPVCUsingDefaultStorageClass(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
	pvcs, err := clientset.CoreV1().PersistentVolumeClaims(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var findings []clusterdoctor.RawFinding

	for _, pvc := range pvcs.Items {
		if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName == "" {
			findings = append(findings, clusterdoctor.RawFinding{
				Namespace:    pvc.Namespace,
				ResourceKind: "PersistentVolumeClaim",
				ResourceName: pvc.Name,
			})
		}
	}

	return findings, nil
}
