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

package v2

// GetFailedJobsLimit returns the number of failed jobs to keep.
// Returns -1 if the value was not specified by the user.
func (vbsc *VitessBackupSchedule) GetFailedJobsLimit() int32 {
	if vbsc.Spec.FailedJobsHistoryLimit == nil {
		return -1
	}
	return *vbsc.Spec.FailedJobsHistoryLimit
}

// GetSuccessfulJobsLimit returns the number of failed jobs to keep.
// Returns -1 if the value was not specified by the user.
func (vbsc *VitessBackupSchedule) GetSuccessfulJobsLimit() int32 {
	if vbsc.Spec.SuccessfulJobsHistoryLimit == nil {
		return -1
	}
	return *vbsc.Spec.SuccessfulJobsHistoryLimit
}

// DefaultAllowedMissedRuns is the default that will be used in case of bug in the operator,
// which could be caused by the apiserver's clock for instance. In the event of such bug,
// the VitessBackupSchedule will try catching up the missed scheduled runs one by one
// this can be extremely lengthy in the even of a big clock skew, if the number of missed scheduled
// jobs reaches either DefaultAllowedMissedRuns or the value specified by the user, the controller
// will give up looking for the previously missed run and error out.
// Setting the default to 100 is fair, catching up a up to 100 missed scheduled runs is not lengthy.
const DefaultAllowedMissedRuns = 100

// GetMissedRunsLimit returns the maximum number of missed run we can allow.
// Returns DefaultAllowedMissedRuns if the value was not specified by the user.
func (vbsc *VitessBackupSchedule) GetMissedRunsLimit() int {
	if vbsc.Spec.AllowedMissedRuns == nil {
		return DefaultAllowedMissedRuns
	}
	return *vbsc.Spec.AllowedMissedRuns
}
