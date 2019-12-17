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

package lazy

import (
	"planetscale.dev/vitess-operator/pkg/operator/vitess"
)

type VitessFlags struct {
	providers []func(Spec) vitess.Flags
}

func (f *VitessFlags) Add(p func(Spec) vitess.Flags) {
	f.providers = append(f.providers, p)
}

func (f *VitessFlags) Get(spec Spec) vitess.Flags {
	res := vitess.Flags{}
	for _, p := range f.providers {
		for k, v := range p(spec) {
			res[k] = v
		}
	}
	return res
}
