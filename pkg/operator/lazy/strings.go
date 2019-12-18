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

type Strings struct {
	providers []func(Spec) []string
}

func (s *Strings) Add(p func(Spec) []string) {
	s.providers = append(s.providers, p)
}

func (s *Strings) Get(spec Spec) []string {
	res := []string{}
	for _, p := range s.providers {
		res = append(res, p(spec)...)
	}
	return res
}
