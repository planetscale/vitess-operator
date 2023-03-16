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

package etcd

import (
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"planetscale.dev/vitess-operator/pkg/operator/update"
)

const (
	// QuorumSize is the number of replicas that must be available for the etcd
	// lockserver to respond to queries.
	QuorumSize = NumReplicas/2 + 1
)

// PDBName returns the name of the PDB for an EtcdLockserver.
func PDBName(lockserverName string) string {
	return "etcd-lockserver-" + lockserverName
}

// NewPDB creates a new PDB.
func NewPDB(key client.ObjectKey, labels map[string]string) *policyv1beta1.PodDisruptionBudget {
	// This tells `kubectl drain` not to delete one of the members unless the
	// number of remaining members will still be at least QuorumSize.
	minAvailable := intstr.FromInt(QuorumSize)

	return &policyv1beta1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      key.Name,
			Labels:    labels,
		},
		Spec: policyv1beta1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			MinAvailable: &minAvailable,
		},
	}
}

// UpdatePDBInPlace updates an existing PDB in-place.
func UpdatePDBInPlace(obj *policyv1beta1.PodDisruptionBudget, labels map[string]string) {
	// Update labels, but ignore existing ones we don't set.
	update.Labels(&obj.Labels, labels)
}
