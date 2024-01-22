/*
Copyright 2024 PlanetScale Inc.

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

package mysql

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"planetscale.dev/vitess-operator/pkg/operator/environment"
	"planetscale.dev/vitess-operator/pkg/operator/vitess"
	"vitess.io/vitess/go/vt/sqlparser"
)

var imageVersionRegExp = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)`)

func UpdateMySQLServerVersion(flags vitess.Flags, mysqldImage string) {
	value, err := dockerImageGetVersionToString(mysqldImage)
	if err != nil {
		return
	}
	flags["mysql_server_version"] = value
	environment.MySQLServerVersion = value
}

func dockerImageGetVersionToString(currentVersionImage string) (string, error) {
	currentVersionSlice := strings.SplitN(currentVersionImage, ":", 2)
	if len(currentVersionSlice) != 2 {
		return "", fmt.Errorf("could not parse the image name and image label, got: %s, but expected xx:xx", currentVersionImage)
	}

	label := currentVersionSlice[1]
	_, err := sqlparser.ConvertMySQLVersionToCommentVersion(label)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-Vitess", label), nil
}

func DockerImageSafeUpgrade(currentVersionImage, desiredVersionImage string) (bool, error) {
	if currentVersionImage == "" || desiredVersionImage == "" {
		// No action if we have unknown versions.
		return false, nil
	}

	// Quick check so no regexp matching is needed for the most common
	// case where nothing changes.
	if desiredVersionImage == currentVersionImage {
		return false, nil
	}

	currentParts := strings.SplitN(currentVersionImage, ":", 2)
	if len(currentParts) != 2 {
		return false, nil
	}

	desiredParts := strings.SplitN(desiredVersionImage, ":", 2)
	if len(desiredParts) != 2 {
		return false, nil
	}

	current := currentParts[1]
	desired := desiredParts[1]

	curStrParts := imageVersionRegExp.FindStringSubmatch(current)
	if len(curStrParts) != 4 {
		// Invalid version, assume that we need to do a safe upgrade.
		return true, nil
	}
	dstStrParts := imageVersionRegExp.FindStringSubmatch(desired)
	if len(dstStrParts) != 4 {
		// Invalid version, assume that we need to do a safe upgrade.
		return true, nil
	}
	if slices.Equal(curStrParts, dstStrParts) {
		return false, nil
	}
	dstParts := make([]int, len(dstStrParts)-1)
	curParts := make([]int, len(curStrParts)-1)
	for i, part := range dstStrParts[1:] {
		// We already matched with `\d_` so there's no
		// way this can trigger an error.
		dstParts[i], _ = strconv.Atoi(part)
	}

	for i, part := range curStrParts[1:] {
		// We already matched with `\d_` so there's no
		// way this can trigger an error.
		curParts[i], _ = strconv.Atoi(part)
	}

	if dstParts[0] < curParts[0] {
		return false, fmt.Errorf("cannot downgrade major version from %s to %s", current, desired)
	}
	if dstParts[0] == curParts[1] && dstParts[1] < curParts[1] {
		return false, fmt.Errorf("cannot downgrade minor version from %s to %s", current, desired)
	}

	// Alright, here it gets more tricky. MySQL has had a complicated release history. For the 8.0 series,
	// up to 8.0.34 at least (known at this point), it was not supported to downgrade patch releases
	// as patch release could also include on-disk data format changes. This happened a number of times
	// in practice as well, so this concern is real.
	//
	// MySQL though has announced a new release strategy, see:
	// https://dev.mysql.com/blog-archive/introducing-mysql-innovation-and-long-term-support-lts-versions/
	//
	// With that release strategy, it will become possible that patch releases will be safe to downgrade
	// as well and since the data format doesn't change on-disk anymore, it's also safe to upgrade with
	// fast shutdown enabled.
	// Specifically, it calls out that "MySQL 8.0.34+ will become bugfix only release (red)". This means
	// that we use that version as a cut-off point here for when we need to disable fast shutdown or not.
	if dstParts[0] == 8 && dstParts[1] == 0 && curParts[0] == 8 && curParts[1] == 0 {
		// Our upgrade process stays within the 8.0.x version range.
		if dstParts[2] >= 34 && curParts[2] >= 34 {
			// No need for safe upgrade if both versions are 8.0.34 or higher.
			return false, nil
		}
		// We can't downgrade within the 8.0.x series before 8.0.34.
		if dstParts[2] < curParts[2] {
			return false, fmt.Errorf("cannot downgrade patch version from %s to %s", current, desired)
		}
		// Always need safe upgrade if we change the patch release for 8.0.x before 8.0.34.
		return dstParts[2] != curParts[2], nil
	}

	// For any major or minor version change we always need safe upgrade.
	return dstParts[0] != curParts[0] || dstParts[1] != curParts[1], nil
}
