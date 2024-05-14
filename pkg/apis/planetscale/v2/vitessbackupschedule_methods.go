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

const DefaultAllowedMissedRuns = 100

// GetMissedRunsLimit returns the maximum number of missed run we can allow.
// Returns DefaultAllowedMissedRuns if the value was not specified by the user.
func (vbsc *VitessBackupSchedule) GetMissedRunsLimit() int {
	if vbsc.Spec.AllowedMissedRun == nil {
		return DefaultAllowedMissedRuns
	}
	return *vbsc.Spec.AllowedMissedRun
}
