/*
Copyright 2026 PlanetScale Inc.

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

package controllermanager

import (
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

func TestInitFlagsExposesTLSFlags(t *testing.T) {
	InitFlags()

	tlsFlags := []string{
		"tablet-manager-grpc-ca",
		"tablet-manager-grpc-cert",
		"tablet-manager-grpc-key",
		"tablet-manager-grpc-crl",
		"tablet-manager-grpc-server-name",
		"topo-global-server-address",
		"topo-etcd-tls-ca",
		"topo-etcd-tls-cert",
		"topo-etcd-tls-key",
	}
	for _, name := range tlsFlags {
		for _, variant := range []string{name, strings.ReplaceAll(name, "-", "_")} {
			if pflag.CommandLine.Lookup(variant) == nil {
				t.Errorf("flag --%s is not registered on the operator command line", variant)
			}
		}
	}
}
