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

package vttablet

import (
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"planetscale.dev/vitess-operator/pkg/operator/mysql"
	"sigs.k8s.io/controller-runtime/pkg/client"
	topodatapb "vitess.io/vitess/go/vt/proto/topodata"
	"vitess.io/vitess/go/vt/topo/topoproto"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/desiredstatehash"
	"planetscale.dev/vitess-operator/pkg/operator/k8s"
	"planetscale.dev/vitess-operator/pkg/operator/names"
	"planetscale.dev/vitess-operator/pkg/operator/update"
)

// PodName returns the name of the Pod for a given vttablet.
func PodName(clusterName string, tabletAlias topodatapb.TabletAlias) string {
	return names.JoinWithConstraints(names.DefaultConstraints, clusterName, planetscalev2.VttabletComponentName, topoproto.TabletAliasString(&tabletAlias))
}

// NewPod creates a new vttablet Pod from a Spec.
func NewPod(key client.ObjectKey, spec *Spec) *corev1.Pod {
	obj := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      key.Name,
		},
	}

	UpdatePod(obj, spec)
	return obj
}

// UpdatePodInPlace updates only the parts of a vttablet Pod that can be changed
// immediately by an in-place update.
func UpdatePodInPlace(obj *corev1.Pod, spec *Spec) {
	// Update labels and annotations, but ignore existing ones we don't set.
	update.Labels(&obj.Labels, spec.Labels)
}

// UpdatePod updates all parts of a vttablet Pod to match the desired state,
// including parts that are immutable.
// If anything actually changes, the Pod must be deleted and recreated as
// part of a rolling update in order to converge to the desired state.
func UpdatePod(obj *corev1.Pod, spec *Spec) {
	// Update our own labels, but ignore existing ones we don't set.
	update.Labels(&obj.Labels, spec.Labels)

	// Update desired user labels.
	update.Labels(&obj.Labels, spec.ExtraLabels)
	// Update desired annotations.
	update.Annotations(&obj.Annotations, spec.Annotations)

	// Collect some common values that will be shared across containers.
	volumeMounts := tabletVolumeMounts.Get(spec)

	// Compute all operator-generated vttablet flags first.
	// Then apply user-provided overrides last so they take precedence.
	vttabletAllFlags := vttabletFlags.Get(spec)
	for key, value := range spec.Vttablet.ExtraFlags {
		// We told users in the CRD API field doc not to put any leading '-',
		// but people may not read that so we are liberal in what we accept.
		key = strings.TrimLeft(key, "-")
		vttabletAllFlags[key] = value
	}
	mysql.UpdateMySQLServerVersion(vttabletAllFlags, spec.Images.Mysqld.Image())

	// Compute all operator-generated env vars first.
	env := tabletEnvVars.Get(spec)
	vttabletEnv := append(vttabletEnvVars.Get(spec), env...)
	update.GOMAXPROCS(&vttabletEnv, spec.Vttablet.Resources)
	mysqldExporterEnv := mysqldExporterEnvVars.Get(spec)
	// Then apply user-provided overrides last so they take precedence.
	update.Env(&env, spec.ExtraEnv)
	update.Env(&vttabletEnv, spec.ExtraEnv)
	update.Env(&mysqldExporterEnv, spec.ExtraEnv)

	// Compute all operator-generated volume mounts first.
	mysqldMounts := append(mysqldVolumeMounts.Get(spec), volumeMounts...)
	vttabletMounts := append(vttabletVolumeMounts.Get(spec), volumeMounts...)
	// Then apply user-provided overrides last so they take precedence.
	update.VolumeMounts(&mysqldMounts, spec.ExtraVolumeMounts)
	update.VolumeMounts(&vttabletMounts, spec.ExtraVolumeMounts)

	securityContext := &corev1.SecurityContext{}
	if planetscalev2.DefaultVitessRunAsUser >= 0 {
		securityContext.RunAsUser = ptr.To(planetscalev2.DefaultVitessRunAsUser)
	}

	vttabletLifecycle := &spec.Vttablet.Lifecycle

	// Build the containers.
	vttabletContainer := &corev1.Container{
		Name:            vttabletContainerName,
		Image:           spec.Images.Vttablet,
		ImagePullPolicy: spec.ImagePullPolicies.Vttablet,
		Command:         []string{vttabletCommand},
		Args:            vttabletAllFlags.FormatArgs(),
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
		SecurityContext: securityContext,
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					// We can't use /debug/health for vttablet as we do for
					// other Vitess servers. On vttablet, that handler has been
					// corrupted into a useless hybrid of readiness and liveness
					// that can't be fixed because it would break legacy users.
					// Instead, vttablet (and only vttablet) has /healthz for
					// actual readiness.
					Path: "/healthz",
					Port: intstr.FromString(planetscalev2.DefaultWebPortName),
				},
			},
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/debug/status",
					Port: intstr.FromString(planetscalev2.DefaultWebPortName),
				},
			},
			InitialDelaySeconds: 300,
			FailureThreshold:    30,
		},
		Lifecycle:    vttabletLifecycle,
		Env:          vttabletEnv,
		VolumeMounts: vttabletMounts,
	}
	// Make a copy of Resources since it contains pointers.
	update.ResourceRequirements(&vttabletContainer.Resources, &spec.Vttablet.Resources)

	var mysqldContainer *corev1.Container
	var mysqldExporterContainer *corev1.Container

	mysqlctldAllFlags := mysqlctldFlags.Get(spec)
	mysql.UpdateMySQLServerVersion(mysqlctldAllFlags, spec.Images.Mysqld.Image())

	if spec.Mysqld != nil {
		mysqldContainer = &corev1.Container{
			Name:            MysqldContainerName,
			Image:           spec.Images.Mysqld.Image(),
			ImagePullPolicy: spec.ImagePullPolicies.Mysqld,
			Command:         []string{mysqldCommand},
			Args:            mysqlctldAllFlags.FormatArgs(),
			Ports: []corev1.ContainerPort{
				{
					Name:          planetscalev2.DefaultMysqlPortName,
					Protocol:      corev1.ProtocolTCP,
					ContainerPort: planetscalev2.DefaultMysqlPort,
				},
			},
			SecurityContext: securityContext,
			ReadinessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt(planetscalev2.DefaultMysqlPort),
					},
				},
				PeriodSeconds: 2,
			},
			// TODO(enisoc): Add liveness probes that make sense for mysqld.
			Env:          env,
			VolumeMounts: mysqldMounts,
		}

		update.ResourceRequirements(&mysqldContainer.Resources, &spec.Mysqld.Resources)

		// Compute all operator-generated mysqld_exporter flags first.
		// Then apply user-provided overrides last so they take precedence.
		mysqldExporterAllFlags := mysqldExporterFlags.Get(spec)
		if spec.MysqldExporter != nil {
			for key, value := range spec.MysqldExporter.ExtraFlags {
				// We told users in the CRD API field doc not to put any leading '-',
				// but people may not read that so we are liberal in what we accept.
				key = strings.TrimLeft(key, "-")
				mysqldExporterAllFlags[key] = value
			}
		}

		// TODO: Can/should we still run mysqld_exporter pointing at external mysql?
		mysqldExporterContainer = &corev1.Container{
			Name:            mysqldExporterContainerName,
			Image:           spec.Images.MysqldExporter,
			ImagePullPolicy: spec.ImagePullPolicies.MysqldExporter,
			Command:         []string{mysqldExporterCommand},
			Args:            mysqldExporterAllFlags.FormatArgsConvertBoolean(),
			Env:             mysqldExporterEnv,
			Ports: []corev1.ContainerPort{
				{
					Name:          mysqldExporterPortName,
					ContainerPort: mysqldExporterPort,
				},
			},
			SecurityContext: securityContext,
			VolumeMounts:    mysqldMounts,
			// TODO(enisoc): Add readiness and liveness probes that make sense for mysqld-exporter.
			//   This depends on the exact semantics of each of mysqld-exporter's HTTP handlers,
			//   so we need to do more investigation. For now it's better to leave them empty.
		}

		if spec.MysqldExporter != nil && (len(spec.MysqldExporter.Resources.Limits) > 0 || len(spec.MysqldExporter.Resources.Requests) > 0) {
			update.ResourceRequirements(&mysqldExporterContainer.Resources, &spec.MysqldExporter.Resources)
		} else {
			mysqldExporterContainer.Resources = corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    *resource.NewMilliQuantity(mysqldExporterCPURequestMillis, resource.DecimalSI),
					corev1.ResourceMemory: *resource.NewQuantity(mysqldExporterMemoryRequestBytes, resource.BinarySI),
				},
				// Set resource limits on mysqld-exporter because we've observed
				// occasional runaway growth in usage that requires a restart.
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    *resource.NewMilliQuantity(mysqldExporterCPULimitMillis, resource.DecimalSI),
					corev1.ResourceMemory: *resource.NewQuantity(mysqldExporterMemoryLimitBytes, resource.BinarySI),
				},
			}
			// TODO(enisoc): Add readiness and liveness probes that make sense for mysqld-exporter.
			//   This depends on the exact semantics of each of mysqld-exporter's HTTP handlers,
			//   so we need to do more investigation. For now it's better to leave them empty.
		}

		if spec.MysqldExporter != nil && (len(spec.MysqldExporter.Resources.Limits) > 0 || len(spec.MysqldExporter.Resources.Requests) > 0) {
			update.ResourceRequirements(&mysqldExporterContainer.Resources, &spec.MysqldExporter.Resources)
		}
	}

	// Set the resource requirements on each of the default vttablet init
	// containers to the same values as the vttablet container itself in
	// case the cluster requires them.
	defaultTabletInitContainers := tabletInitContainers.Get(spec)
	for i := range defaultTabletInitContainers {
		c := &defaultTabletInitContainers[i]
		update.ResourceRequirements(&c.Resources, &spec.Vttablet.Resources)
	}

	// Make the final list of desired containers and init containers.
	initContainers := []corev1.Container{}
	initContainers = append(initContainers, defaultTabletInitContainers...)
	initContainers = append(initContainers, spec.InitContainers...)

	sidecarContainers := []corev1.Container{}
	sidecarContainers = append(sidecarContainers, spec.SidecarContainers...)

	containers := []corev1.Container{
		*vttabletContainer,
	}

	if spec.Mysqld != nil {
		containers = append(containers, *mysqldContainer)

		// Only deploy mysqld-exporter if the image is set.
		if mysqldExporterContainer.Image != "" {
			containers = append(containers, *mysqldExporterContainer)
		}
	}

	// Record hashes of desired label and annotation keys to force the Pod
	// to be recreated if a key disappears from the desired list.
	desiredStateHash := desiredstatehash.NewBuilder()
	desiredStateHash.AddStringMapKeys("labels-keys", spec.ExtraLabels)
	desiredStateHash.AddStringMapKeys("annotations-keys", spec.Annotations)

	// Record a hash of desired containers to force the Pod to be recreated if
	// something is removed from our desired state that we otherwise might
	// mistake for an item added by the API server and leave behind.
	desiredStateHash.AddContainersUpdates("init-containers", initContainers)
	desiredStateHash.AddContainersUpdates("containers", containers)

	// Record a hash of desired tolerations and topologySpreadConstraints
	// to force the Pod to be recreated if one disappears from the desired list.
	desiredStateHash.AddTolerations("tolerations", spec.Tolerations)
	desiredStateHash.AddTopologySpreadConstraints("topologySpreadConstraints", spec.TopologySpreadConstraints)

	// Add the final desired state hash annotation.
	update.Annotations(&obj.Annotations, map[string]string{
		desiredstatehash.Annotation: desiredStateHash.String(),
	})

	// Update the containers we care about in the Pod template,
	// ignoring other containers that may have been injected.
	update.PodContainers(&obj.Spec.InitContainers, initContainers)
	update.PodContainers(&obj.Spec.Containers, sidecarContainers)
	update.PodContainers(&obj.Spec.Containers, containers)

	// Update other parts of the Pod.
	obj.Spec.ImagePullSecrets = spec.ImagePullSecrets
	update.Annotations(&obj.Annotations, tabletAnnotations.Get(spec))
	update.Volumes(&obj.Spec.Volumes, tabletVolumes.Get(spec))
	update.Volumes(&obj.Spec.Volumes, spec.ExtraVolumes)
	update.Tolerations(&obj.Spec.Tolerations, spec.Tolerations)
	update.TopologySpreadConstraints(&obj.Spec.TopologySpreadConstraints, spec.TopologySpreadConstraints)

	if obj.Spec.SecurityContext == nil {
		obj.Spec.SecurityContext = &corev1.PodSecurityContext{}
	}
	if planetscalev2.DefaultVitessFSGroup >= 0 {
		obj.Spec.SecurityContext.FSGroup = ptr.To(planetscalev2.DefaultVitessFSGroup)
	}

	if spec.Vttablet.TerminationGracePeriodSeconds != nil {
		obj.Spec.TerminationGracePeriodSeconds = spec.Vttablet.TerminationGracePeriodSeconds
	} else {
		obj.Spec.TerminationGracePeriodSeconds = ptr.To(int64(defaultTerminationGracePeriodSeconds))
	}

	// In both the case of the user injecting their own affinity and the default, we
	// simply override the pod's existing affinity configuration.
	if spec.Affinity != nil {
		obj.Spec.Affinity = spec.Affinity
	} else {
		obj.Spec.Affinity = &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
					{
						// A Node with no members of the same shard would be ideal.
						Weight: 2,
						PodAffinityTerm: corev1.PodAffinityTerm{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: spec.shardLabels(),
							},
							TopologyKey: k8s.HostnameLabel,
						},
					},
					{
						// If that's not possible, a Node that at least has no
						// members of the exact same pool would be nice.
						Weight: 1,
						PodAffinityTerm: corev1.PodAffinityTerm{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: spec.poolLabels(),
							},
							TopologyKey: k8s.HostnameLabel,
						},
					},
				},
			},
		}
		if spec.Zone != "" {
			// Limit to a specific zone.
			obj.Spec.Affinity.NodeAffinity = &corev1.NodeAffinity{
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
			}
		}
	}

	// Use the PriorityClass we defined for vttablets in deploy/priority.yaml,
	// or a custom value if overridden on the operator command line.
	if planetscalev2.DefaultVitessPriorityClass != "" {
		obj.Spec.PriorityClassName = planetscalev2.DefaultVitessPriorityClass
	}

	if planetscalev2.DefaultVitessServiceAccount != "" {
		obj.Spec.ServiceAccountName = planetscalev2.DefaultVitessServiceAccount
	}
}

// AliasFromPod returns a TabletAlias corresponding to a vttablet Pod.
func AliasFromPod(pod *corev1.Pod) topodatapb.TabletAlias {
	uid, _ := strconv.ParseUint(pod.Labels[planetscalev2.TabletUidLabel], 10, 32)
	return topodatapb.TabletAlias{
		Cell: pod.Labels[planetscalev2.CellLabel],
		Uid:  uint32(uid),
	}
}
