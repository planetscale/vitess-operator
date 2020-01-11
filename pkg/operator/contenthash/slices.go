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
)

// StringList returns a hex-encoded hash of a list of strings.
//
// This can be used to compare two ordered lists of strings for equality.
// The strings can be arbitrary values.
func StringList(in []string) string {
	h := md5.New()
	for _, k := range in {
		kHash := md5.Sum([]byte(k))
		h.Write(kHash[:])
	}

	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}
