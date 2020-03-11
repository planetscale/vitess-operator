/*
Copyright 2020 PlanetScale Inc.

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

package lockserver

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/names"
	"planetscale.dev/vitess-operator/pkg/operator/update"
)

// LocalEtcdName returns the name of the EtcdLockserver object used for a cell-local lockserver.
func LocalEtcdName(clusterName, cellName string) string {
	// It's important that "etcd" is in the name, even though we already know it's an EtcdCluster object,
	// because etcd-operator uses that same name to create things like Services that might collide.
	return names.Join(clusterName, cellName, planetscalev2.EtcdComponentName)
}

// GlobalEtcdName returns the name of the EtcdLockserver object used for a global lockserver.
func GlobalEtcdName(clusterName string) string {
	// It's important that "etcd" is in the name, even though we already know it's an EtcdCluster object,
	// because etcd-operator uses that same name to create things like Services that might collide.
	return names.Join(clusterName, planetscalev2.EtcdComponentName)
}

// NewEtcdLockserver generates an EtcdLockserver object for the given EtcdLockserverTemplate.
// The EtcdLockserverTemplate must have already had defaults filled in.
func NewEtcdLockserver(key client.ObjectKey, tpl *planetscalev2.EtcdLockserverTemplate, labels map[string]string, zone string) *planetscalev2.EtcdLockserver {
	ls := &planetscalev2.EtcdLockserver{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
	}
	UpdateEtcdLockserver(ls, tpl, labels, zone)
	return ls
}

// UpdateEtcdLockserver updates parts of an existing EtcdLockserver that are allowed to change in-place.
// The EtcdLockserverTemplate must have already had defaults filled in.
func UpdateEtcdLockserver(obj *planetscalev2.EtcdLockserver, tpl *planetscalev2.EtcdLockserverTemplate, labels map[string]string, zone string) {
	update.Labels(&obj.Labels, labels)
	obj.Spec.Zone = zone
	obj.Spec.EtcdLockserverTemplate = *tpl
}
