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

import (
	"k8s.io/apimachinery/pkg/util/sets"
)

// ReloadSecretNames returns a string set containing the names of gateway secrets that have to be forcefully reloaded by restarting all vtgates
func (s *VitessCellGatewaySpec) ReloadSecretNames() sets.String {
	secretNames := sets.NewString()

	if s.SecureTransport != nil &&
		s.SecureTransport.TLS != nil {
		tls := s.SecureTransport.TLS
		if tls.ClientCACertSecret != nil {
			secretNames.Insert(tls.ClientCACertSecret.Name)
		}
		if tls.CertSecret != nil {
			secretNames.Insert(tls.CertSecret.Name)
		}
		if tls.KeySecret != nil {
			secretNames.Insert(tls.KeySecret.Name)
		}
	}

	for i := range s.ExtraVolumes {
		vol := &s.ExtraVolumes[i]
		if vol.Secret != nil {
			secretNames.Insert(vol.Secret.SecretName)
		}
	}

	return secretNames
}
