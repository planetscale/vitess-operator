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

// Package contenthash hashes the content of various objects.
package contenthash

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TopologySpreadConstraints returns a hex-encoded hash of a list of topologySpreadConstraints.
func TopologySpreadConstraints(in []corev1.TopologySpreadConstraint) string {
	h := md5.New()

	for i := range in {
		tsc := &in[i]

		writeStringHash(h, string(tsc.MaxSkew))
		writeStringHash(h, tsc.TopologyKey)
		writeStringHash(h, string(tsc.WhenUnsatisfiable))
		labelSelectors, err := metav1.LabelSelectorAsMap(tsc.LabelSelector)
		if err != nil {
			// There is no return of error for this function
			// or its callers, which makes this a bit of a hack
			labelSelectors = map[string]string{}
		}
		writeStringHash(h, StringMap(labelSelectors))
		writeStringHash(h, strings.Join(tsc.MatchLabelKeys, ""))
		minDomainsSeconds := "nil"
		if tsc.MinDomains != nil {
			minDomainsSeconds = fmt.Sprint(*tsc.MinDomains)
		}
		writeStringHash(h, minDomainsSeconds)
		if tsc.NodeAffinityPolicy != nil {
			writeStringHash(h, string(*tsc.NodeAffinityPolicy))
		}
		if tsc.NodeTaintsPolicy != nil {
			writeStringHash(h, string(*tsc.NodeTaintsPolicy))
		}
	}

	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}
