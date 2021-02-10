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

package vtgate

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
	containerName = "vtgate"

	command    = "/vt/bin/vtgate"
	serviceMap = "grpc-vtgateservice"

	tabletTypesToWait = "MASTER,REPLICA"

	bufferMasterTrafficDuringFailover = true
	bufferMinTimeBetweenFailovers     = "20s"
	bufferMaxFailoverDuration         = "10s"

	grpcMaxMessageSize = 64 * 1024 * 1024

	staticAuthDirName      = "vtgate-static-auth"
	tlsCertDirName         = "vtgate-tls-cert"
	tlsKeyDirName          = "vtgate-tls-key"
	tlsClientCACertDirName = "vtgate-tls-ca-cert"
)

// DeploymentName returns the name of the vtgate Deployment for a given cell.
func DeploymentName(clusterName, cellName string) string {
	return names.JoinWithConstraints(names.DefaultConstraints, clusterName, cellName, planetscalev2.VtgateComponentName)
}

// Spec specifies all the internal parameters needed to deploy vtgate,
// as opposed to the API type planetscalev2.VitessCellGatewaySpec, which is the public API.
type Spec struct {
	Cell              *planetscalev2.VitessCellSpec
	CellsToWatch      []string
	Labels            map[string]string
	Replicas          int32
	Resources         corev1.ResourceRequirements
	Authentication    *planetscalev2.VitessGatewayAuthentication
	SecureTransport   *planetscalev2.VitessGatewaySecureTransport
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

// NewDeployment creates a new Deployment object for vtgate.
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

// UpdateDeployment updates the mutable parts of the vtgate Deployment.
func UpdateDeployment(obj *appsv1.Deployment, spec *Spec) {
	// Set labels on the Deployment object.
	update.Labels(&obj.Labels, spec.Labels)

	// Reset Pod template labels so we remove old ones.
	obj.Spec.Template.Labels = nil
	// Tell Deployment to set the same labels on the Pods it creates.
	update.Labels(&obj.Spec.Template.Labels, spec.Labels)

	// Tell Deployment to set user labels on the Pods it creates.
	update.Labels(&obj.Spec.Template.Labels, spec.ExtraLabels)
	// Tell Deployment to set annotations on Pods it creates.
	obj.Spec.Template.Annotations = spec.Annotations

	// Deployment options.
	obj.Spec.Replicas = pointer.Int32Ptr(spec.Replicas)
	obj.Spec.RevisionHistoryLimit = pointer.Int32Ptr(0)

	// Reset the list of volumes in the template so we remove old ones.
	obj.Spec.Template.Spec.Volumes = nil

	// Pod template options.
	obj.Spec.Template.Spec.ImagePullSecrets = spec.Cell.ImagePullSecrets
	obj.Spec.Template.Spec.PriorityClassName = planetscalev2.DefaultVitessPriorityClass
	obj.Spec.Template.Spec.ServiceAccountName = planetscalev2.DefaultVitessServiceAccount
	obj.Spec.Template.Spec.Tolerations = spec.Tolerations

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

	env := []corev1.EnvVar{}
	update.GOMAXPROCS(&env, spec.Resources)
	update.Env(&env, spec.ExtraEnv)

	securityContext := &corev1.SecurityContext{}
	if planetscalev2.DefaultVitessRunAsUser >= 0 {
		securityContext.RunAsUser = pointer.Int64Ptr(planetscalev2.DefaultVitessRunAsUser)
	}

	// Start building the main Container to put in the Pod template.
	vtgateContainer := &corev1.Container{
		Name:            containerName,
		Image:           spec.Cell.Images.Vtgate,
		ImagePullPolicy: spec.Cell.ImagePullPolicies.Vtgate,
		Command:         []string{command},
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
			{
				Name:          planetscalev2.DefaultMysqlPortName,
				Protocol:      corev1.ProtocolTCP,
				ContainerPort: planetscalev2.DefaultMysqlPort,
			},
		},
		SecurityContext: securityContext,
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
		Env:          env,
	}
	// Make a copy of Resources since it contains pointers.
	update.ResourceRequirements(&vtgateContainer.Resources, &spec.Resources)

	// Get all the flags that don't need any logic.
	flags := spec.baseFlags()

	// Update the Pod template, container, and flags for various optional things.
	updateAuth(spec, flags, vtgateContainer, &obj.Spec.Template.Spec)
	updateTransport(spec, flags, vtgateContainer, &obj.Spec.Template.Spec)
	update.Volumes(&obj.Spec.Template.Spec.Volumes, spec.ExtraVolumes)

	// Apply user-provided overrides last so they take precedence.
	for key, value := range spec.ExtraFlags {
		// We told users in the CRD API field doc not to put any leading '-',
		// but people may not read that so we are liberal in what we accept.
		key = strings.TrimLeft(key, "-")
		flags[key] = value
	}
	// Write out the final flags list.
	vtgateContainer.Args = flags.FormatArgs()

	update.PodTemplateContainers(&obj.Spec.Template.Spec.InitContainers, spec.InitContainers)
	update.PodTemplateContainers(&obj.Spec.Template.Spec.Containers, spec.SidecarContainers)

	// Update the container we care about in the Pod template,
	// ignoring other containers that may have been injected.
	update.PodTemplateContainers(&obj.Spec.Template.Spec.Containers, []corev1.Container{*vtgateContainer})
}

func (spec *Spec) baseFlags() vitess.Flags {
	cellsToWatch := spec.CellsToWatch
	if len(cellsToWatch) == 0 {
		// For now, we have all cells watch all cells.
		// When there are many cells, it might make sense for each cell to only watch
		// itself plus whatever cells might contain MySQL masters, assuming only certain
		// cells are "master-eligible".
		cellsToWatch = spec.Cell.AllCells
	}

	return vitess.Flags{
		"cell":                 spec.Cell.Name,
		"cells_to_watch":       strings.Join(cellsToWatch, ","),
		"tablet_types_to_wait": tabletTypesToWait,

		"enable_buffer":                     bufferMasterTrafficDuringFailover,
		"buffer_min_time_between_failovers": bufferMinTimeBetweenFailovers,
		"buffer_max_failover_duration":      bufferMaxFailoverDuration,

		"grpc_max_message_size": grpcMaxMessageSize,

		"mysql_server_port": planetscalev2.DefaultMysqlPort,

		"logtostderr":                true,
		"topo_implementation":        spec.Cell.GlobalLockserver.Implementation,
		"topo_global_server_address": spec.Cell.GlobalLockserver.Address,
		"topo_global_root":           spec.Cell.GlobalLockserver.RootPath,

		"service_map": serviceMap,
		"port":        planetscalev2.DefaultWebPort,
		"grpc_port":   planetscalev2.DefaultGrpcPort,
	}
}

func updateAuth(spec *Spec, flags vitess.Flags, container *corev1.Container, podSpec *corev1.PodSpec) {
	if spec.Authentication.Static != nil && spec.Authentication.Static.Secret != nil {
		staticAuthFile := secrets.Mount(spec.Authentication.Static.Secret, staticAuthDirName)

		// Get usernames and passwords from a static file, mounted from a Secret.
		flags["mysql_auth_server_impl"] = "static"
		flags["mysql_auth_server_static_file"] = staticAuthFile.FilePath()
		flags["mysql_auth_static_reload_interval"] = "30s"

		// Add the volume to the Pod, if needed.
		update.Volumes(&podSpec.Volumes, staticAuthFile.PodVolumes())

		// Mount the volume in the Container.
		container.VolumeMounts = append(container.VolumeMounts, staticAuthFile.ContainerVolumeMount())
	}
}

func updateTransport(spec *Spec, flags vitess.Flags, container *corev1.Container, podSpec *corev1.PodSpec) {
	if spec.SecureTransport != nil && spec.SecureTransport.TLS != nil {
		tls := spec.SecureTransport.TLS

		// Required, but may change in future, so leaving option open to allow optional.
		if tls.CertSecret == nil || tls.KeySecret == nil {
			return
		}

		tlsCertFile := secrets.Mount(tls.CertSecret, tlsCertDirName)
		tlsKeyFile := secrets.Mount(tls.KeySecret, tlsKeyDirName)

		// GRPC does not have an equivalent flag,
		// and all GRPC transport is required to be encrypted when certs are set.
		flags["mysql_server_require_secure_transport"] = spec.SecureTransport.Required

		flags["mysql_server_ssl_cert"] = tlsCertFile.FilePath()
		flags["mysql_server_ssl_key"] = tlsKeyFile.FilePath()
		flags["grpc_cert"] = tlsCertFile.FilePath()
		flags["grpc_key"] = tlsKeyFile.FilePath()

		// Add the volumes to the Pod, if needed.
		update.Volumes(&podSpec.Volumes, tlsCertFile.PodVolumes())
		update.Volumes(&podSpec.Volumes, tlsKeyFile.PodVolumes())

		// Mount the volumes in the Container.
		container.VolumeMounts = append(container.VolumeMounts,
			tlsCertFile.ContainerVolumeMount(),
			tlsKeyFile.ContainerVolumeMount(),
		)

		if tls.ClientCACertSecret != nil {
			clientCACertFile := secrets.Mount(tls.ClientCACertSecret, tlsClientCACertDirName)

			flags["mysql_server_ssl_ca"] = clientCACertFile.FilePath()
			flags["grpc_ca"] = clientCACertFile.FilePath()

			update.Volumes(&podSpec.Volumes, clientCACertFile.PodVolumes())

			container.VolumeMounts = append(container.VolumeMounts, clientCACertFile.ContainerVolumeMount())
		}
	}
}
