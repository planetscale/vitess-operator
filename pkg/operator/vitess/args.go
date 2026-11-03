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
	"regexp"
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
// live if the Vitess binary it runs still comes from the same pinned image and
// those args are equivalent to the desired ones (see EquivalentArgs), and the
// desired args otherwise. liveBinaryImage is the image the binary came from in
// live, and binaryImage the one it will come from. For most containers that is
// the container's own image; mysqlctld instead runs a binary copied from the
// vttablet image.
//
// The same build accepts the command line it is already running, so a change
// that only respells flag names, or drops a flag in droppedFlags, doesn't
// restart a Pod or roll a Deployment by itself. The new command line goes out
// with the next new image or real flag change for that container, and with
// any Pod recreated for another reason, since a new Pod is built from scratch;
// a Deployment rolled for another reason keeps the old spelling. An image that isn't
// pinned (see pinnedImage) can start a newer binary without its reference
// changing, which may no longer accept the old spelling, so it always gets the
// desired args.
func KeepEquivalentArgs(live []corev1.Container, name, liveBinaryImage, binaryImage string, desired []string) []string {
	if liveBinaryImage != binaryImage || !pinnedImage(binaryImage) {
		return desired
	}
	for i := range live {
		if live[i].Name != name {
			continue
		}
		if EquivalentArgs(live[i].Args, desired) {
			return slices.Clone(live[i].Args)
		}
		break
	}
	return desired
}

// ContainerImage returns the image of the container with the given name, or ""
// if there is none.
func ContainerImage(containers []corev1.Container, name string) string {
	for i := range containers {
		if containers[i].Name == name {
			return containers[i].Image
		}
	}
	return ""
}

// releaseTag matches the tags Vitess publishes for a release: v24.0.5,
// optionally with a release candidate and a MySQL flavor, as in
// v24.0.0-rc1-mysql84 or v24.0.5-percona80.
var releaseTag = regexp.MustCompile(`^v\d+\.\d+\.\d+(-rc\d+)?(-[a-z]+\d+)?$`)

// pinnedImage reports whether an image reference names one fixed build: by
// digest, or by a release tag, which releases don't move to a later build.
// Anything else, such as latest, no tag, a rolling tag like mysql80, or a
// release-looking development tag like v25.0.0-SNAPSHOT, can start a newer
// binary without the reference changing.
func pinnedImage(image string) bool {
	if strings.Contains(image, "@") {
		return true
	}
	name := image[strings.LastIndex(image, "/")+1:]
	_, tag, ok := strings.Cut(name, ":")
	return ok && releaseTag.MatchString(tag)
}
