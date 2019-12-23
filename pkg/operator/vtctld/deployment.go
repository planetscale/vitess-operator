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

package vtctld

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
	"planetscale.dev/vitess-operator/pkg/operator/update"
	"planetscale.dev/vitess-operator/pkg/operator/vitess"
)

const (
	containerName     = "vtctld"
	priorityClassName = "vitess"

	command    = "/vt/bin/vtctld"
	webDir     = "/vt/src/vitess.io/vitess/web/vtctld"
	webDir2    = "/vt/src/vitess.io/vitess/web/vtctld2/app"
	serviceMap = "grpc-vtctl"
	runAsUser  = 999
)

// DeploymentName returns the name of the vtctld Deployment for a given cell.
func DeploymentName(clusterName, cellName string) string {
	return names.Join(clusterName, cellName, planetscalev2.VtctldComponentName)
}

// Spec specifies all the internal parameters needed to deploy vtctld,
// as opposed to the API type planetscalev2.VitessDashboardSpec, which is the public API.
type Spec struct {
	GlobalLockserver  *planetscalev2.VitessLockserverParams
	Cell              *planetscalev2.VitessCellTemplate
	Image             string
	ImagePullPolicy   corev1.PullPolicy
	Labels            map[string]string
	Replicas          int32
	Resources         corev1.ResourceRequirements
	Affinity          *corev1.Affinity
	ExtraFlags        map[string]string
	ExtraEnv          []corev1.EnvVar
	ExtraVolumes      []corev1.Volume
	ExtraVolumeMounts []corev1.VolumeMount
	InitContainers    []corev1.Container
	Annotations       map[string]string
}

// NewDeployment creates a new Deployment object for vtctld.
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

// UpdateDeploymentImmediate updates the mutable parts of the vtctld Deployment
// that are safe to change immediately.
func UpdateDeploymentImmediate(obj *appsv1.Deployment, spec *Spec) {
	// Set labels on the Deployment object.
	update.Labels(&obj.Labels, spec.Labels)

	// Scaling up or down doesn't require a rolling update.
	obj.Spec.Replicas = pointer.Int32Ptr(spec.Replicas)
}

// UpdateDeployment updates the mutable parts of the vtctld Deployment
// that should be changed as part of a gradual, rolling update.
func UpdateDeployment(obj *appsv1.Deployment, spec *Spec) {
	UpdateDeploymentImmediate(obj, spec)

	// Tell Deployment to set the same labels on the Pods it creates.
	update.Labels(&obj.Spec.Template.Labels, spec.Labels)

	// Tell Deployment to set annotations on Pods that it creates.
	obj.Spec.Template.Annotations = spec.Annotations

	// Deployment options.
	obj.Spec.RevisionHistoryLimit = pointer.Int32Ptr(0)

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
	obj.Spec.Template.Spec.PriorityClassName = priorityClassName
	update.Volumes(&obj.Spec.Template.Spec.Volumes, spec.ExtraVolumes)

	update.Containers(&obj.Spec.Template.Spec.InitContainers, spec.InitContainers)
	update.Containers(&obj.Spec.Template.Spec.Containers, []corev1.Container{
		{
			Name:            containerName,
			Image:           spec.Image,
			ImagePullPolicy: spec.ImagePullPolicy,
			Command:         []string{command},
			Args:            flags.FormatArgs(),
			Ports: []corev1.ContainerPort{
				{
					Name:          planetscalev2.DefaultWebPortName,
					Protocol:      corev1.ProtocolTCP,
					ContainerPort: planetscalev2.DefaultWebPort,
				},
				{
					Name:          planetscalev2.DefaultGrpcPortName,
					Protocol:      corev1.ProtocolTCP,
					ContainerPort: planetscalev2.DefaultGrpcPort,
				},
			},
			Resources: spec.Resources,
			SecurityContext: &corev1.SecurityContext{
				RunAsUser: pointer.Int64Ptr(runAsUser),
			},
			ReadinessProbe: &corev1.Probe{
				Handler: corev1.Handler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/debug/health",
						Port: intstr.FromString(planetscalev2.DefaultWebPortName),
					},
				},
			},
			LivenessProbe: &corev1.Probe{
				Handler: corev1.Handler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/debug/status",
						Port: intstr.FromString(planetscalev2.DefaultWebPortName),
					},
				},
				InitialDelaySeconds: 300,
				FailureThreshold:    30,
			},
			VolumeMounts: spec.ExtraVolumeMounts,
			Env:          spec.ExtraEnv,
		},
	})

	if spec.Affinity != nil {
		obj.Spec.Template.Spec.Affinity = spec.Affinity
	} else if spec.Cell.Zone != "" {
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
									Values:   []string{spec.Cell.Zone},
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
		"cell":     spec.Cell.Name,
		"web_dir":  webDir,
		"web_dir2": webDir2,

		"port":        planetscalev2.DefaultWebPort,
		"grpc_port":   planetscalev2.DefaultGrpcPort,
		"service_map": serviceMap,

		"topo_implementation":        spec.GlobalLockserver.Implementation,
		"topo_global_server_address": spec.GlobalLockserver.Address,
		"topo_global_root":           spec.GlobalLockserver.RootPath,

		"logtostderr":           true,
		"enable_queries":        true,
		"enable_realtime_stats": true,

		"workflow_manager_init":         true,
		"workflow_manager_use_election": true,
	}
}
