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

// Secrets provides utility functions for handling secrets.
package secrets

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ContentHash stably hashes secrets by their contents.
func ContentHash(secrets ...*corev1.Secret) string {
	hashes := map[string][]byte{}
	for _, s := range secrets {
		hashes[s.Name] = []byte(contentHash(s.Data))
	}

	// Digest of all the dependent hashes.
	return contentHash(hashes)
}

// GetByNames gets all secretNames in namespace.
func GetByNames(ctx context.Context, cl client.Client, namespace string, secretNames sets.String) ([]*corev1.Secret, error) {
	var secrets []*corev1.Secret
	for _, name := range secretNames.UnsortedList() {
		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}

		rval := &corev1.Secret{}
		err := cl.Get(ctx, key, rval)
		if err != nil {
			return nil, err
		}

		secrets = append(secrets, rval)
	}

	return secrets, nil
}

func contentHash(m map[string][]byte) string {
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
