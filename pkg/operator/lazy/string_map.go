/*
Copyright 2019 PlanetScale.

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

type StringMap struct {
	providers []func(Spec) map[string]string
}

func (s *StringMap) Add(p func(Spec) map[string]string) {
	s.providers = append(s.providers, p)
}

func (s *StringMap) Get(spec Spec) map[string]string {
	res := map[string]string{}
	for _, p := range s.providers {
		for k, v := range p(spec) {
			res[k] = v
		}
	}
	return res
}
