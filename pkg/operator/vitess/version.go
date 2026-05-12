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

package vitess

import (
	"strings"

	"github.com/blang/semver/v4"
)

// MajorVersionFromImage parses the Vitess major version from a Docker image
// reference like "vitess/lite:v24.0.0-mysql80". Returns (major, true) when
// the image carries a SemVer-compatible tag and (0, false) otherwise
// (rolling tags such as "mysql80" or "latest", digests, or empty input).
func MajorVersionFromImage(image string) (int, bool) {
	if image == "" {
		return 0, false
	}
	// Drop digest portion if present (e.g. "repo:tag@sha256:...").
	if at := strings.IndexByte(image, '@'); at >= 0 {
		image = image[:at]
	}
	colon := strings.LastIndexByte(image, ':')
	if colon < 0 {
		return 0, false
	}
	v, err := semver.ParseTolerant(image[colon+1:])
	if err != nil {
		return 0, false
	}
	return int(v.Major), true
}
