/*
Copyright 2019 PlanetScale Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package vitessbackupstorage

import (
	"context"
	"fmt"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/controller/vitessbackupstorage/subcontroller"
	"planetscale.dev/vitess-operator/pkg/operator/fork"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	"planetscale.dev/vitess-operator/pkg/operator/update"
	"planetscale.dev/vitess-operator/pkg/operator/vitessbackup"
)

const (
	// operatorContainerNameSubstring needs to be contained within the name of
	// the main container in deploy/operator.yaml.
	operatorContainerNameSubstring = "-operator"

	vitessHomeDir = "/home/vitess"

	subcontrollerCPUMillis   = 100
	subcontrollerMemoryBytes = 128 * (1 << 20) // 128 MiB
)

func (r *ReconcileVitessBackupStorage) reconcileSubcontroller(ctx context.Context, vbs *planetscalev2.VitessBackupStorage) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	// Most of the logic of VitessBackupStorage objects is implemented in the
	// subcontroller. All we do here is launch a subcontroller Pod for each
	// VitessBackupStorage object, since processing each object requires
	// mounting different Secrets (e.g. for S3/GCS credentials) and Volumes
	// (e.g. for NFS). See the 'subcontroller' package for details.
	clusterName := vbs.Labels[planetscalev2.ClusterLabel]
	key := client.ObjectKey{
		Namespace: vbs.Namespace,
		Name:      vbs.Name + "-vitessbackupstorage-subcontroller",
	}
	labels := map[string]string{
		planetscalev2.ComponentLabel: planetscalev2.VBSSubcontrollerComponentName,
		planetscalev2.ClusterLabel:   clusterName,
		vitessbackup.LocationLabel:   vbs.Spec.Location.Name,
	}

	spec, err := r.newSubcontrollerPodSpec(ctx, vbs)
	if err != nil {
		return resultBuilder.Error(err)
	}

	// Reconcile subcontroller Pod.
	err = r.reconciler.ReconcileObject(ctx, vbs, key, labels, true, reconciler.Strategy{
		Kind: &corev1.Pod{},

		New: func(key client.ObjectKey) runtime.Object {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: key.Namespace,
					Name:      key.Name,
					Labels:    labels,
				},
				Spec: *spec,
			}
			update.Annotations(&pod.Annotations, vbs.Spec.Location.Annotations)
			return pod
		},
		UpdateInPlace: func(key client.ObjectKey, newObj runtime.Object) {
			pod := newObj.(*corev1.Pod)
			update.Labels(&pod.Labels, labels)
			update.Annotations(&pod.Annotations, vbs.Spec.Location.Annotations)
		},
		UpdateRecreate: func(key client.ObjectKey, newObj runtime.Object) {
			pod := newObj.(*corev1.Pod)

			// Remember the NodeName from the live object so we don't try to
			// clear it out.
			nodeName := pod.Spec.NodeName

			curSpec := pod.Spec
			newSpec := spec.DeepCopy()
			for _, volume := range curSpec.Volumes {
				if strings.Index(volume.Name, curSpec.ServiceAccountName+"-token-") == 0 {
					// copy the volume from the current pod to the replacement:
					update.Volumes(&(newSpec.Volumes), []corev1.Volume{volume})
				}
			}
			newSpec.Volumes = sortVolsSame(newSpec.Volumes, curSpec.Volumes)
			for _, curContainer := range curSpec.Containers {
				var newContainer *corev1.Container
				for i, c := range newSpec.Containers {
					if c.Name == curContainer.Name {
						newContainer = &(newSpec.Containers[i])
					}
				}
				if newContainer != nil {
					for _, mount := range curContainer.VolumeMounts {
						if strings.Index(mount.Name, curSpec.ServiceAccountName+"-token-") == 0 {
							// copy the mount from the current container to the replacement:
							update.VolumeMounts(&(newContainer.VolumeMounts), []corev1.VolumeMount{mount})
						}
					}
					newContainer.VolumeMounts = sortMountsSame(newContainer.VolumeMounts, curContainer.VolumeMounts)
				}
			}
			pod.Spec = *newSpec
			pod.Spec.NodeName = nodeName
		},
	})
	if err != nil {
		resultBuilder.Error(err)
	}

	return resultBuilder.Result()
}

func sortVolsSame(vols []corev1.Volume, order []corev1.Volume) []corev1.Volume {
	res := make([]corev1.Volume, 0, len(vols))
	found := make(map[string]struct{}, len(vols))
	for _, ov := range order {
		for _, iv := range vols {
			if iv.Name == ov.Name {
				res = append(res, ov)
				found[iv.Name] = struct{}{}
			}
		}
	}
	for _, iv := range vols {
		if _, ok := found[iv.Name]; !ok {
			res = append(res, iv)
		}
	}
	return res
}

func sortMountsSame(mounts []corev1.VolumeMount, order []corev1.VolumeMount) []corev1.VolumeMount {
	res := make([]corev1.VolumeMount, 0, len(mounts))
	found := make(map[string]struct{}, len(mounts))
	for _, ov := range order {
		for _, iv := range mounts {
			if iv.Name == ov.Name {
				res = append(res, ov)
				found[iv.Name] = struct{}{}
			}
		}
	}
	for _, iv := range mounts {
		if _, ok := found[iv.Name]; !ok {
			res = append(res, iv)
		}
	}
	return res
}

func (r *ReconcileVitessBackupStorage) newSubcontrollerPodSpec(ctx context.Context, vbs *planetscalev2.VitessBackupStorage) (*corev1.PodSpec, error) {
	// Start by forking the operator Pod we're running in.
	spec, err := fork.NewPodSpec(ctx, r.client, subcontroller.ForkPath)
	if err != nil {
		return nil, err
	}

	// Find the main operator container.
	var container *corev1.Container
	for i := range spec.Containers {
		if strings.Contains(spec.Containers[i].Name, operatorContainerNameSubstring) {
			container = &spec.Containers[i]
			break
		}
	}
	if container == nil {
		return nil, fmt.Errorf("can't find operator container (name containing %q) in my own Pod", operatorContainerNameSubstring)
	}

	if vbs.Spec.Subcontroller.ServiceAccountName != "" {
		// allow the pod to be launched with a specific serviceaccount in the target namespace (which will be the same
		// namespace as the VitessCluster itself)
		spec.ServiceAccountName = vbs.Spec.Subcontroller.ServiceAccountName
		spec.DeprecatedServiceAccount = vbs.Spec.Subcontroller.ServiceAccountName
	}

	// filter out the service account tokens (volumes and mounts):
	var newVolumes []corev1.Volume
	for _, volume := range spec.Volumes {
		if strings.Index(volume.Name, spec.ServiceAccountName + "-token-") < 0 {
			// skip volumes from the automounted token
			newVolumes = append(newVolumes, volume)
		}
	}
	spec.Volumes = newVolumes
	for i := range spec.Containers {
		var newMounts []corev1.VolumeMount
		for _, mount := range spec.Containers[i].VolumeMounts {
			if strings.Index(mount.Name, spec.ServiceAccountName + "-token-") < 0 {
				// skip mounts from the automounted token
				newMounts = append(newMounts, mount)
			}
		}
		spec.Containers[i].VolumeMounts = newMounts
	}


	// Tell the subcontroller which VitessBackupStorage object to process.
	update.Env(&container.Env, []corev1.EnvVar{
		{
			Name:  subcontroller.VBSNamespaceEnvVar,
			Value: vbs.Namespace,
		},
		{
			Name:  subcontroller.VBSNameEnvVar,
			Value: vbs.Name,
		},
		{
			Name:  "HOME",
			Value: vitessHomeDir,
		},
		{
			Name: k8sutil.WatchNamespaceEnvVar,
			Value: vbs.Namespace,
		},
	})

	// Set resource requests specific to the subcontroller.
	// It doesn't need as much as the main operator process.
	container.Resources.Requests = corev1.ResourceList{
		corev1.ResourceCPU:    *resource.NewMilliQuantity(subcontrollerCPUMillis, resource.DecimalSI),
		corev1.ResourceMemory: *resource.NewQuantity(subcontrollerMemoryBytes, resource.BinarySI),
	}
	container.Resources.Limits = corev1.ResourceList{
		corev1.ResourceMemory: *resource.NewQuantity(subcontrollerMemoryBytes, resource.BinarySI),
	}

	// Add config for this specific backup storage location.
	clusterName := vbs.Labels[planetscalev2.ClusterLabel]
	backupFlags := vitessbackup.StorageFlags(&vbs.Spec.Location, clusterName)
	container.Args = append(container.Args, backupFlags.FormatArgs()...)
	update.VolumeMounts(&container.VolumeMounts, vitessbackup.StorageVolumeMounts(&vbs.Spec.Location))
	update.Volumes(&spec.Volumes, vitessbackup.StorageVolumes(&vbs.Spec.Location))
	update.Env(&container.Env, vitessbackup.StorageEnvVars(&vbs.Spec.Location))

	return spec, nil
}
