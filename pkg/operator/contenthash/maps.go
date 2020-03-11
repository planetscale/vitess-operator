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

// Package contenthash hashes the content of various objects.
package contenthash

import (
	"crypto/md5"
	"encoding/hex"
	"sort"
)

// StringMapKeys returns a hex-encoded hash of only the keys in the map.
//
// This can be used to compare two unordered sets of keys for equality.
// The keys can be arbitrary strings.
// The values in the map are ignored.
func StringMapKeys(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return StringList(keys)
}

// BytesMap returns a hex-encoded hash of all the content in the map.
//
// This can be used to compare two unordered sets of key-value pairs for equality.
// The keys can be arbitrary strings.
// The values can be arbitrary byte slices.
func BytesMap(m map[string][]byte) string {
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
		vHash := md5.Sum(v)
		h.Write(vHash[:])
	}

	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}

// StringMap returns a hex-encoded hash of all the content in the map.
//
// This can be used to compare two unordered sets of key-value pairs for equality.
// The keys and values can both be arbitrary strings.
func StringMap(m map[string]string) string {
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
