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

package contenthash

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"sort"
	"strconv"

	corev1 "k8s.io/api/core/v1"
)

// Tolerations returns a hex-encoded hash of a list of Tolerations.
func Tolerations(in []corev1.Toleration) string {
	h := md5.New()

	sort.Slice(in, func(i, j int) bool {
		return in[i].Key < in[j].Key
	})

	for i := range in {
		tol := &in[i]

		writeStringHash(h, tol.Key)
		writeStringHash(h, string(tol.Operator))
		writeStringHash(h, tol.Value)
		writeStringHash(h, string(tol.Effect))

		tolerationSeconds := "nil"
		if tol.TolerationSeconds != nil {
			tolerationSeconds = strconv.FormatInt(*tol.TolerationSeconds, 10)
		}
		writeStringHash(h, tolerationSeconds)
	}

	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}

func writeStringHash(w io.Writer, value string) {
	hash := md5.Sum([]byte(value))
	w.Write(hash[:])
}
