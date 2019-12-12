// Package stringmaps contains helpers for working with string maps.
package stringmaps

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"sort"
)

// HashKeys returns a hex-encoded hash of all the keys in the map.
//
// This can be used to compare two unordered sets of keys for equality.
// The keys are assumed to not contain newlines.
// The values in the map are ignored.
func HashKeys(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	h := md5.New()
	for _, k := range keys {
		fmt.Fprintln(h, k)
	}

	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}
