/*
Copyright 2019 PlanetScale.

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

package etcd

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"planetscale.dev/vitess-operator/pkg/operator/vitess"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"planetscale.dev/vitess-operator/pkg/operator/k8s"
	"planetscale.dev/vitess-operator/pkg/operator/update"
)

const (
	// LockserverLabel is the label that should be added to Pods to identify
	// which lockserver cluster they belong to.
	LockserverLabel = "etcd.planetscale.com/lockserver"
	// IndexLabel is the label used to identify the index of a member.
	IndexLabel = "etcd.planetscale.com/index"

	// NumReplicas is the number of members per etcd cluster.
	//
	// This is currently hard-coded because it doesn't really make sense to
	// allow it to be customized. Anything less than 3 cannot maintain quorum
	// if a single member becomes unavailable. Anything more than 3 adds latency
	// without providing significant benefit to Vitess.
	//
	// WARNING: DO NOT change this value. That would break all existing EtcdLockservers.
	//          The only way to change this is to implement a new feature to support
	//          having different sizes for different EtcdLockserver objects.
	NumReplicas = 3

	etcdContainerName     = "etcd"
	etcdCommand           = "/usr/local/bin/etcd"
	etcdPriorityClassName = "vitess"

	dataVolumeName      = "data"
	dataVolumeMountPath = "/var/etcd"
	dataVolumeSubPath   = "etcd"
)

// PodName returns the name of the Pod for a given etcd member.
func PodName(lockserverName string, index int) string {
	return fmt.Sprintf("%s-%d", lockserverName, index)
}

// Spec specifies all the internal parameters needed to deploy an etcd instance.
type Spec struct {
	LockserverName    string
	Image             string
	Resources         corev1.ResourceRequirements
	Labels            map[string]string
	Zone              string
	Index             int
	DataVolumePVCName string
	DataVolumePVCSpec *corev1.PersistentVolumeClaimSpec
	ExtraFlags        map[string]string
	ExtraEnv          []corev1.EnvVar
	ExtraVolumes      []corev1.Volume
	ExtraVolumeMounts []corev1.VolumeMount
	Affinity          *corev1.Affinity
	Annotations       map[string]string
}

// NewPod creates a new etcd Pod.
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

// UpdatePodInPlace updates only the parts of an etcd Pod that can be changed
// immediately by an in-place update.
func UpdatePodInPlace(obj *corev1.Pod, spec *Spec) {
	// Update labels and annotations, but ignore existing ones we don't set.
	update.Labels(&obj.Labels, spec.Labels)
}

// UpdatePod updates all parts of an etcd Pod to match the desired state,
// including parts that are immutable.
// If anything actually changes, the Pod must be deleted and recreated as
// part of a rolling update in order to converge to the desired state.
func UpdatePod(obj *corev1.Pod, spec *Spec) {
	// Update labels, but ignore existing ones we don't set.
	update.Labels(&obj.Labels, spec.Labels)

	// Update annotations and recreate pod if annotations are updated or removed.
	annotations := spec.Annotations
	annotationsHash := map[string]string{
		"annotationsDeltaHash": contentHash(annotations),
	}
	update.Annotations(&annotations, annotationsHash)
	update.Annotations(&obj.Annotations, annotations)

	// Compute default environment variables first.
	env := []corev1.EnvVar{
		// Reference Values: https://github.com/etcd-io/etcd/blob/master/Documentation/op-guide/maintenance.md#auto-compaction
		{
			Name:  "ETCD_AUTO_COMPACTION_MODE",
			Value: "revision",
		},
		{
			Name:  "ETCD_AUTO_COMPACTION_RETENTION",
			Value: "1000",
		},
		{
			Name:  "ETCD_QUOTA_BACKEND_BYTES",
			Value: "8589934592", // 8 * 1024 * 1024 * 1024 = 8GiB
		},
		{
			Name:  "ETCD_MAX_REQUEST_BYTES",
			Value: "8388608", // 8 * 1024 * 1024 = 8MiB
		},
		{
			Name:  "ETCDCTL_API",
			Value: "3",
		},
	}
	// Apply user-provided environment variable overrides.
	update.Env(&env, spec.ExtraEnv)

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      dataVolumeName,
			MountPath: dataVolumeMountPath,
			SubPath:   dataVolumeSubPath,
		},
	}
	update.VolumeMounts(&volumeMounts, spec.ExtraVolumeMounts)

	etcdContainer := &corev1.Container{
		Name:    etcdContainerName,
		Image:   spec.Image,
		Command: []string{etcdCommand},
		Args:    spec.Args(),
		Ports: []corev1.ContainerPort{
			{
				Name:          clientPortName,
				Protocol:      corev1.ProtocolTCP,
				ContainerPort: clientPortNumber,
			},
			{
				Name:          peerPortName,
				Protocol:      corev1.ProtocolTCP,
				ContainerPort: peerPortNumber,
			},
		},
		Resources: spec.Resources,
		ReadinessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				Exec: &corev1.ExecAction{
					Command: []string{"etcdctl", "endpoint", "health"},
				},
			},
			FailureThreshold:    3,
			InitialDelaySeconds: 1,
			PeriodSeconds:       5,
			SuccessThreshold:    1,
			TimeoutSeconds:      5,
		},
		LivenessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				Exec: &corev1.ExecAction{
					Command: []string{"etcdctl", "endpoint", "status"},
				},
			},
			FailureThreshold:    30,
			InitialDelaySeconds: 300,
			PeriodSeconds:       5,
			SuccessThreshold:    1,
			TimeoutSeconds:      5,
		},
		Env:          env,
		VolumeMounts: volumeMounts,
	}

	update.Volumes(&obj.Spec.Volumes, []corev1.Volume{
		{
			Name: dataVolumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: spec.DataVolumePVCName,
				},
			},
		},
	})
	update.Volumes(&obj.Spec.Volumes, spec.ExtraVolumes)

	obj.Spec.Hostname = PodName(spec.LockserverName, spec.Index)
	obj.Spec.Subdomain = PeerServiceName(spec.LockserverName)

	// In both the case of the user injecting their own affinity and the default, we
	// simply override the pod's existing affinity configuration.
	if spec.Affinity != nil {
		obj.Spec.Affinity = spec.Affinity
	} else {
		obj.Spec.Affinity = &corev1.Affinity{
			// Try to spread the replicas across Nodes if possible.
			PodAntiAffinity: &corev1.PodAntiAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
					{
						Weight: 2,
						PodAffinityTerm: corev1.PodAffinityTerm{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									LockserverLabel: spec.LockserverName,
								},
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
		} else {
			// If we're not limited to one zone, try to spread across zones.
			paa := &obj.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution
			*paa = append(*paa, corev1.WeightedPodAffinityTerm{
				// Weight zone spreading as less important than node spreading.
				Weight: 1,
				PodAffinityTerm: corev1.PodAffinityTerm{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							LockserverLabel: spec.LockserverName,
						},
					},
					TopologyKey: k8s.ZoneFailureDomainLabel,
				},
			})
		}
	}

	// Use the PriorityClass we defined for etcd in deploy/priority.yaml.
	obj.Spec.PriorityClassName = etcdPriorityClassName

	// Update the containers we care about in the Pod template,
	// ignoring other containers that may have been injected.
	update.Containers(&obj.Spec.Containers, []corev1.Container{
		*etcdContainer,
	})
}

// Args returns the etcd args.
func (spec *Spec) Args() []string {
	hostname := PodName(spec.LockserverName, spec.Index)
	subdomain := PeerServiceName(spec.LockserverName)

	listenPeerURLs := fmt.Sprintf("http://0.0.0.0:%d", peerPortNumber)
	listenClientURLs := fmt.Sprintf("http://0.0.0.0:%d", clientPortNumber)
	advertiseClientURLs := fmt.Sprintf("http://%s.%s:%d", hostname, subdomain, clientPortNumber)

	// Use static bootstrapping.
	initialClusterToken := spec.LockserverName
	initialAdvertisePeerURLs := fmt.Sprintf("http://%s.%s:%d", hostname, subdomain, peerPortNumber)
	initialCluster := make([]string, 0, NumReplicas)
	for i := 1; i <= NumReplicas; i++ {
		name := PodName(spec.LockserverName, i)
		initialCluster = append(initialCluster, fmt.Sprintf("%s=http://%s.%s:%d", name, name, subdomain, peerPortNumber))
	}

	flags := vitess.Flags{
		"data-dir":              dataVolumeMountPath,
		"name":                  hostname,
		"listen-peer-urls":      listenPeerURLs,
		"listen-client-urls":    listenClientURLs,
		"advertise-client-urls": advertiseClientURLs,

		// All "initial-*" flags are ignored after bootstrapping.
		"initial-cluster-state":       "new",
		"initial-cluster-token":       initialClusterToken,
		"initial-advertise-peer-urls": initialAdvertisePeerURLs,
		"initial-cluster":             strings.Join(initialCluster, ","),
	}

	// Apply user-supplied extra flags last so they take precedence.
	for key, value := range spec.ExtraFlags {
		// We told users in the CRD API field doc not to put any leading '-',
		// but we are liberal in what we accept.
		key = strings.TrimLeft(key, "-")
		flags[key] = value
	}

	return flags.FormatArgs()
}

func contentHash(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	h := md5.New()
	for _, k := range keys {
		v := m[k]
		kHash := md5.Sum([]byte(k))
		h.Write(kHash[:])
		vHash := md5.Sum([]byte(v))
		h.Write(vHash[:])
	}

	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}
