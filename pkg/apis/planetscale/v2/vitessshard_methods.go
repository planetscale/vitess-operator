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

package v2

import corev1 "k8s.io/api/core/v1"

// IsExternalMaster returns a boolean indicating whether the tablet is in a pool of type "externalmaster"
// +k8s:openapi-gen=true
func (t *VitessTabletStatus) IsExternalMaster() bool {
	return t.PoolType == ExternalMasterTabletPoolName
}

// IsExternalMaster returns a boolean indicating whether the tablet is in a pool of type "externalmaster"
// +k8s:openapi-gen=true
func (t *VitessTabletStatus) IsRunning() bool {
	return t.Running == corev1.ConditionTrue
}

// InitTabletType returns a string representing what the initial tablet
// type should be for a tablet in this type of pool.
// +k8s:openapi-gen=true
func (t *VitessTabletPoolType) InitTabletType() string {
	switch *t {
	case ExternalMasterPoolType:
		// All external master tablets start out SPARE, as opposed to regular masters which start out REPLICA.
		// We don't want external masters to ever serve REPLICA queries because it's not actually possible to
		// convert an external tablet between REPLICA and MASTER at the MySQL level, since we don't control
		// replication in the external case.
		return "spare"
	case ExternalReplicaPoolType:
		// We tell Vitess this is a normal REPLICA because the distinction that it's an external replica only
		// matters for configuration (e.g. other flags) that we handle at the operator level.
		return "replica"
	case ExternalRdonlyPoolType:
		// We tell Vitess this is a normal RDONLY because the distinction that it's an external rdonly only
		// matters for configuration (e.g. other flags) that we handle at the operator level.
		return "rdonly"
	default:
		return string(*t)
	}
}

// UsingExternalDatastore returns a boolean indicating whether the VitessShard Spec is using
// externally managed MySQL for any of its tablet pools.
// +k8s:openapi-gen=true
func (v *VitessShard) UsingExternalDatastore() bool {
	for i := range v.Spec.TabletPools {
		p := &v.Spec.TabletPools[i]
		if p.ExternalDatastore != nil {
			return true
		}
	}

	return false
}

// AllPoolsUsingMysqld returns a boolean indicating whether the VitessShard Spec is using
// local MySQL for all of it's pools by checking the Mysqld field of all tablet pools.
// +k8s:openapi-gen=true
func (v *VitessShard) AllPoolsUsingMysqld() bool {
	for i := range v.Spec.TabletPools {
		p := &v.Spec.TabletPools[i]
		if p.Mysqld == nil {
			return false
		}
	}

	return true
}
