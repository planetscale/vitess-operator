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
	"maps"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// droppedFlags are flags the operator used to pass to Vitess binaries and no
// longer does. A live command line may still carry them, so that dropping
// them doesn't restart a Pod by itself.
var droppedFlags = map[string]bool{
	"logtostderr":     true,
	"alsologtostderr": true,
}

// EquivalentArgs reports whether a live Vitess command line sets the same
// flags to the same values as the desired one, both in the form FormatArgs
// produces, apart from the underscore or dash spelling of flag names. A flag in
// droppedFlags that is only in the live command line is ignored; one in the
// desired command line, such as a user's own setting, must match.
//
// A command line that sets one flag under two spellings is only equivalent to
// an identical one, since Vitess then overrides or combines the values
// depending on the flag.
func EquivalentArgs(live, desired []string) bool {
	if slices.Equal(live, desired) {
		return true
	}
	liveFlags, ok := parseFlagArgs(live)
	if !ok {
		return false
	}
	desiredFlags, ok := parseFlagArgs(desired)
	if !ok {
		return false
	}
	for name := range droppedFlags {
		if _, ok := desiredFlags[name]; !ok {
			delete(liveFlags, name)
		}
	}
	return maps.Equal(liveFlags, desiredFlags)
}

// parseFlagArgs maps each normalized flag name in a command line to its
// value. It fails on anything that isn't a --name=value flag, and on a flag
// that is set more than once.
func parseFlagArgs(args []string) (map[string]string, bool) {
	flags := make(map[string]string, len(args))
	for _, arg := range args {
		nameValue, ok := strings.CutPrefix(arg, "--")
		if !ok {
			return nil, false
		}
		name, value, ok := strings.Cut(nameValue, "=")
		if !ok {
			return nil, false
		}
		name = normalizeFlagName(name)
		if _, ok := flags[name]; ok {
			return nil, false
		}
		flags[name] = value
	}
	return flags, true
}

// KeepEquivalentArgs returns the args of the container with the given name in
// live if that container runs the same image and its args are equivalent to
// the desired ones (see EquivalentArgs), and the desired args otherwise.
//
// The same image accepts the command line it is already running, so a change
// that only respells flag names, or drops a flag in droppedFlags, doesn't
// restart a Pod or roll a Deployment by itself. The new command line goes out
// with the next new image or real flag change for that container; a Pod
// recreated for any other reason keeps the old spelling.
func KeepEquivalentArgs(live []corev1.Container, name, image string, desired []string) []string {
	for i := range live {
		if live[i].Name != name {
			continue
		}
		if live[i].Image == image && EquivalentArgs(live[i].Args, desired) {
			return slices.Clone(live[i].Args)
		}
		break
	}
	return desired
}
