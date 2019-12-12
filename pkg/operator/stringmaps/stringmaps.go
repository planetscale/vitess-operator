package stringmaps

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"sort"
)

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
