/*
Copyright 2022 PlanetScale Inc.

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

package vtadmin

import (
	"fmt"
	"planetscale.dev/vitess-operator/pkg/operator/secrets"
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
	apiContainerName = "vtadmin-api"
	webContainerName = "vtadmin-web"

	apiCommand = "/vt/bin/vtadmin"
	webDir     = "/vt/web/vtadmin"

	rbacConfigDirName      = "rbac-config"
	discoveryStatiFilePath = "discovery-config"
)

// DeploymentName returns the name of the vtadmin Deployment for a given cell.
func DeploymentName(clusterName, cellName string) string {
	return names.JoinWithConstraints(names.DefaultConstraints, clusterName, cellName, planetscalev2.VtadminComponentName)
}

// DiscoverySecretName returns the name of the vtadmin discovery sercret's name for a given cell.
func DiscoverySecretName(clusterName, cellName string) string {
	return names.JoinWithConstraints(names.DefaultConstraints, clusterName, cellName, planetscalev2.VtadminComponentName, "discovery")
}

// Spec specifies all the internal parameters needed to deploy vtadmin,
// as opposed to the API type planetscalev2.VtAdminSpec, which is the public API.
type Spec struct {
	Cell *planetscalev2.VitessCellTemplate
	// Discovery holds the secret information for the vtctld and vtgate
	// endpoints to use by vtadmin
	Discovery         *planetscalev2.SecretSource
	Rbac              *planetscalev2.SecretSource
	Image             string
	ClusterName       string
	ImagePullPolicy   corev1.PullPolicy
	ImagePullSecrets  []corev1.LocalObjectReference
	Labels            map[string]string
	Replicas          int32
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

// NewDeployment creates a new Deployment object for vtadmin.
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

// UpdateDeploymentImmediate updates the mutable parts of the vtadmin Deployment
// that are safe to change immediately.
func UpdateDeploymentImmediate(obj *appsv1.Deployment, spec *Spec) {
	// Set labels on the Deployment object.
	update.Labels(&obj.Labels, spec.Labels)

	// Scaling up or down doesn't require a rolling update.
	obj.Spec.Replicas = pointer.Int32Ptr(spec.Replicas)
}

// UpdateDeployment updates the mutable parts of the vtadmin Deployment
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
	apiFlags := spec.apiFlags()
	for key, value := range spec.ExtraFlags {
		// We told users in the CRD API field doc not to put any leading '-',
		// but people may not read that so we are liberal in what we accept.
		key = strings.TrimLeft(key, "-")
		apiFlags[key] = value
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

	// Make a copy of Resources since it contains pointers.
	var apiContainerResources corev1.ResourceRequirements
	vtadminAPIContainer := &corev1.Container{
		Name:            apiContainerName,
		Image:           spec.Image,
		ImagePullPolicy: spec.ImagePullPolicy,
		Command:         []string{apiCommand},
		Ports: []corev1.ContainerPort{
			{
				Name:          planetscalev2.DefaultAPIPortName,
				Protocol:      corev1.ProtocolTCP,
				ContainerPort: planetscalev2.DefaultAPIPort,
			},
		},
		Resources:       apiContainerResources,
		SecurityContext: securityContext,
		ReadinessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					// TODO: find the correct end point for readiness check
					Path: "/debug/env",
					Port: intstr.FromString(planetscalev2.DefaultAPIPortName),
				},
			},
		},
		LivenessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					// TODO: find the correct end point for liveness check
					Path: "/debug/env",
					Port: intstr.FromString(planetscalev2.DefaultAPIPortName),
				},
			},
			InitialDelaySeconds: 300,
			FailureThreshold:    30,
		},
		VolumeMounts: spec.ExtraVolumeMounts,
		Env:          spec.ExtraEnv,
	}
	update.ResourceRequirements(&apiContainerResources, &spec.Resources)
	updateRbac(spec, apiFlags, vtadminAPIContainer, &obj.Spec.Template.Spec)
	updateDiscovery(spec, apiFlags, vtadminAPIContainer, &obj.Spec.Template.Spec)
	vtadminAPIContainer.Args = apiFlags.FormatArgs()

	var webContainerResources corev1.ResourceRequirements
	envWithAPIPort := spec.ExtraEnv
	envWithAPIPort = append(envWithAPIPort, corev1.EnvVar{
		Name:  "REACT_APP_VTADMIN_API_ADDRESS",
		Value: fmt.Sprintf("http://localhost:%d", planetscalev2.DefaultAPIPort),
	})
	vtadminWebContainer := &corev1.Container{
		Name:            webContainerName,
		Image:           spec.Image,
		ImagePullPolicy: spec.ImagePullPolicy,
		Command:         []string{fmt.Sprintf("%s/node_modules/.bin/serve", webDir)},
		Args: []string{"--no-clipboard",
			"-l", fmt.Sprintf("%d", planetscalev2.DefaultWebPort),
			"-s", fmt.Sprintf("%s/build", webDir)},
		Ports: []corev1.ContainerPort{
			{
				Name:          planetscalev2.DefaultWebPortName,
				Protocol:      corev1.ProtocolTCP,
				ContainerPort: planetscalev2.DefaultWebPort,
			},
		},
		Resources:       webContainerResources,
		SecurityContext: securityContext,
		ReadinessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					// TODO: find the correct end point for readiness check
					Path: "/",
					Port: intstr.FromString(planetscalev2.DefaultWebPortName),
				},
			},
		},
		LivenessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					// TODO: find the correct end point for liveness check
					Path: "/",
					Port: intstr.FromString(planetscalev2.DefaultWebPortName),
				},
			},
			InitialDelaySeconds: 300,
			FailureThreshold:    30,
		},
		VolumeMounts: spec.ExtraVolumeMounts,
		Env:          envWithAPIPort,
	}
	// TODO: different resource requirements for web and api
	update.ResourceRequirements(&webContainerResources, &spec.Resources)
	update.PodTemplateContainers(&obj.Spec.Template.Spec.Containers, []corev1.Container{*vtadminAPIContainer, *vtadminWebContainer})

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

func (spec *Spec) apiFlags() vitess.Flags {
	return vitess.Flags{
		"addr":         fmt.Sprintf(":%d", planetscalev2.DefaultAPIPort),
		"http-origin":  fmt.Sprintf("http://localhost:%d", planetscalev2.DefaultWebPort),
		"tracer":       "opentracing-jaeger",
		"grpc-tracing": true,
		"http-tracing": true,

		"logtostderr":     true,
		"alsologtostderr": true,
	}
}

//  --http-tablet-url-tmpl "http://{{ .Tablet.Hostname }}:15{{ .Tablet.Alias.Uid }}" \
//  -- \
//  --cluster "id=local,name=local,discovery=staticfile,discovery-staticfile-path=./vtadmin/discovery.json,tablet-fqdn-tmpl={{ .Tablet.Hostname }}:15{{ .Tablet.Alias.Uid }}" \

func updateRbac(spec *Spec, flags vitess.Flags, container *corev1.Container, podSpec *corev1.PodSpec) {
	if spec.Rbac != nil {
		rbacConfigFile := secrets.Mount(spec.Rbac, rbacConfigDirName)
		flags["rbac"] = true
		flags["rbac-config"] = rbacConfigFile.FilePath()

		// Add the volume to the Pod, if needed.
		update.Volumes(&podSpec.Volumes, rbacConfigFile.PodVolumes())
		// Mount the volume in the Container.
		container.VolumeMounts = append(container.VolumeMounts, rbacConfigFile.ContainerVolumeMount())
	} else {
		flags["no-rbac"] = true
	}
}

func updateDiscovery(spec *Spec, flags vitess.Flags, container *corev1.Container, podSpec *corev1.PodSpec) {
	discoveryFile := secrets.Mount(spec.Discovery, discoveryStatiFilePath)
	// Add the volume to the Pod, if needed.
	update.Volumes(&podSpec.Volumes, discoveryFile.PodVolumes())
	// Mount the volume in the Container.
	container.VolumeMounts = append(container.VolumeMounts, discoveryFile.ContainerVolumeMount())

	clusterFlagVal, clusterFlagExists := flags["cluster"]
	var clusterFlagStringProvided string
	var isString bool
	if clusterFlagExists {
		clusterFlagStringProvided, isString = clusterFlagVal.(string)
		if !isString {
			return
		}
	}

	// We use the cluster name as the identifier
	clusterIdentifier := spec.ClusterName
	// If it is not provided, then we use "cluster"
	if clusterIdentifier == "" {
		clusterIdentifier = "cluster"
	}
	clusterFlagString := fmt.Sprintf("id=%s,name=%s,discovery=staticfile,discovery-staticfile-path=%s", clusterIdentifier, clusterIdentifier, discoveryFile.FilePath())
	if len(clusterFlagStringProvided) != 0 {
		clusterFlagString += "," + clusterFlagStringProvided
	}
	flags["cluster"] = clusterFlagString
}
