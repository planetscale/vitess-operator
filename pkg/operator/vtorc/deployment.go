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

package vtorc

import (
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/k8s"
	"planetscale.dev/vitess-operator/pkg/operator/names"
	"planetscale.dev/vitess-operator/pkg/operator/secrets"
	"planetscale.dev/vitess-operator/pkg/operator/update"
	"planetscale.dev/vitess-operator/pkg/operator/vitess"
)

const (
	containerName = "vtorc"

	command = "/vt/bin/vtorc"
	webDir  = "/vt/web/orchestrator"

	configDirName = "vtorc-config"
)

func deploymentName(clusterName, keyspace, shardSafeName, cellName string) string {
	return names.JoinWithConstraints(names.DefaultConstraints, clusterName, keyspace, shardSafeName, cellName, planetscalev2.VtorcComponentName)
}

// DeploymentName returns the name of the orchestrator Deployment for a given tablet pool.
func DeploymentName(clusterName, keyspace string, shardKeyRange planetscalev2.VitessKeyRange, cellName string) string {
	return deploymentName(clusterName, keyspace, shardKeyRange.SafeName(), cellName)
}

// Spec specifies all the internal parameters needed to deploy orchestrator,
// as opposed to the API type planetscalev2.VitessDashboardSpec, which is the public API.
type Spec struct {
	GlobalLockserver  planetscalev2.VitessLockserverParams
	ConfigSecret      planetscalev2.SecretSource
	Keyspace          string
	Shard             string
	Cell              string
	Zone              string
	Image             string
	ImagePullPolicy   corev1.PullPolicy
	ImagePullSecrets  []corev1.LocalObjectReference
	Labels            map[string]string
	Resources         corev1.ResourceRequirements
	Affinity          *corev1.Affinity
	ExtraFlags        map[string]string
	ExtraEnv          []corev1.EnvVar
	ExtraVolumes      []corev1.Volume
	ExtraVolumeMounts []corev1.VolumeMount
	InitContainers    []corev1.Container
	SidecarContainers []corev1.Container
	Annotations       map[string]string
	ExtraLabels       map[string]string
	Tolerations       []corev1.Toleration
}

// NewDeployment creates a new Deployment object for vtorc.
func NewDeployment(key client.ObjectKey, spec *Spec) *appsv1.Deployment {
	// Fill in the immutable parts.
	obj := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      key.Name,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: spec.Labels,
			},
		},
	}
	// Set everything else.
	UpdateDeployment(obj, spec)
	return obj
}

// UpdateDeploymentImmediate updates the mutable parts of the orchestrator Deployment
// that are safe to change immediately.
func UpdateDeploymentImmediate(obj *appsv1.Deployment, spec *Spec) {
	// Set labels on the Deployment object.
	update.Labels(&obj.Labels, spec.Labels)
}

// UpdateDeployment updates the mutable parts of the vtorc Deployment
// that should be changed as part of a gradual, rolling update.
func UpdateDeployment(obj *appsv1.Deployment, spec *Spec) {
	UpdateDeploymentImmediate(obj, spec)

	// Reset Pod template labels so we remove old ones.
	obj.Spec.Template.Labels = nil
	// Tell Deployment to set the same labels on the Pods it creates.
	update.Labels(&obj.Spec.Template.Labels, spec.Labels)
	// Tell Deployment to set user labels on the Pods it creates.
	update.Labels(&obj.Spec.Template.Labels, spec.ExtraLabels)

	// Tell Deployment to set annotations on Pods that it creates.
	obj.Spec.Template.Annotations = spec.Annotations

	// Deployment options.
	obj.Spec.RevisionHistoryLimit = pointer.Int32Ptr(0)

	// Reset the list of volumes in the template so we remove old ones.
	obj.Spec.Template.Spec.Volumes = nil

	// Apply user-provided flag overrides after generating base flags.
	flags := spec.flags()
	for key, value := range spec.ExtraFlags {
		// We told users in the CRD API field doc not to put any leading '-',
		// but people may not read that so we are liberal in what we accept.
		key = strings.TrimLeft(key, "-")
		flags[key] = value
	}

	// Set only the Pod template fields we care about.
	// Use functions from the `operator/update` package for lists
	// that should actually be treated like maps (update items by the .Name field).
	obj.Spec.Template.Spec.ImagePullSecrets = spec.ImagePullSecrets
	obj.Spec.Template.Spec.PriorityClassName = planetscalev2.DefaultVitessPriorityClass
	obj.Spec.Template.Spec.ServiceAccountName = planetscalev2.DefaultVitessServiceAccount
	obj.Spec.Template.Spec.Tolerations = spec.Tolerations
	update.Volumes(&obj.Spec.Template.Spec.Volumes, spec.ExtraVolumes)

	securityContext := &corev1.SecurityContext{}
	if planetscalev2.DefaultVitessRunAsUser >= 0 {
		securityContext.RunAsUser = pointer.Int64Ptr(planetscalev2.DefaultVitessRunAsUser)
	}

	update.PodTemplateContainers(&obj.Spec.Template.Spec.InitContainers, spec.InitContainers)
	update.PodTemplateContainers(&obj.Spec.Template.Spec.Containers, spec.SidecarContainers)
	vtorcContainer := &corev1.Container{
		Name:            containerName,
		Image:           spec.Image,
		ImagePullPolicy: spec.ImagePullPolicy,
		Command:         []string{command},
		Ports: []corev1.ContainerPort{
			{
				Name:          planetscalev2.DefaultWebPortName,
				Protocol:      corev1.ProtocolTCP,
				ContainerPort: planetscalev2.DefaultVtorcWebPort,
			},
		},
		Resources:       spec.Resources,
		SecurityContext: securityContext,
		ReadinessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					// TODO(sougou): fix orch to export better end points
					Path: "/web/clusters",
					Port: intstr.FromString(planetscalev2.DefaultWebPortName),
				},
			},
		},
		LivenessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					// TODO(sougou): fix orch to export better end points
					Path: "/web/clusters",
					Port: intstr.FromString(planetscalev2.DefaultWebPortName),
				},
			},
			InitialDelaySeconds: 300,
			FailureThreshold:    30,
		},
		VolumeMounts: spec.ExtraVolumeMounts,
		Env:          spec.ExtraEnv,
	}
	updateConfig(spec, flags, vtorcContainer, &obj.Spec.Template.Spec)
	vtorcContainer.Args = flags.FormatArgs()
	update.PodTemplateContainers(&obj.Spec.Template.Spec.Containers, []corev1.Container{*vtorcContainer})

	if spec.Affinity != nil {
		obj.Spec.Template.Spec.Affinity = spec.Affinity
	} else if spec.Zone != "" {
		// Limit to a specific zone.
		obj.Spec.Template.Spec.Affinity = &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      k8s.ZoneFailureDomainLabel,
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{spec.Zone},
								},
							},
						},
					},
				},
			},
		}
	} else {
		obj.Spec.Template.Spec.Affinity = nil
	}
}

func (spec *Spec) flags() vitess.Flags {
	return vitess.Flags{
		"topo_implementation":        spec.GlobalLockserver.Implementation,
		"topo_global_server_address": spec.GlobalLockserver.Address,
		"topo_global_root":           spec.GlobalLockserver.RootPath,

		"clusters_to_watch": spec.Keyspace + "/" + spec.Shard,

		"orc_web_dir": webDir,

		"logtostderr": true,
	}
}

func updateConfig(spec *Spec, flags vitess.Flags, container *corev1.Container, podSpec *corev1.PodSpec) {
	configFile := secrets.Mount(&spec.ConfigSecret, configDirName)

	flags["config"] = configFile.FilePath()

	// Add the volume to the Pod, if needed.
	update.Volumes(&podSpec.Volumes, configFile.PodVolumes())

	// Mount the volume in the Container.
	container.VolumeMounts = append(container.VolumeMounts, configFile.ContainerVolumeMount())
}
