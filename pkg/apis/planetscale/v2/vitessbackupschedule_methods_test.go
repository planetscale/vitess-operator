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

package v2

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestValidateBackupFrequency(t *testing.T) {
	tests := []struct {
		name      string
		frequency time.Duration
		wantErr   bool
	}{
		{name: "one minute", frequency: time.Minute},
		{name: "thirty minutes", frequency: 30 * time.Minute},
		{name: "six hours", frequency: 6 * time.Hour},
		{name: "daily", frequency: 24 * time.Hour},
		{name: "sub minute rejected", frequency: 30 * time.Second, wantErr: true},
		{name: "forty five minutes rejected", frequency: 45 * time.Minute, wantErr: true},
		{name: "ninety minutes rejected", frequency: 90 * time.Minute, wantErr: true},
		{name: "forty eight hours rejected", frequency: 48 * time.Hour, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBackupFrequency(tt.frequency)
			if tt.wantErr {
				require.Error(t, err, "ValidateBackupFrequency(%s) error = nil, want error", tt.frequency)
			} else {
				require.NoError(t, err, "ValidateBackupFrequency(%s) error = %v, want nil", tt.frequency, err)
			}
		})
	}
}

func TestValidateScheduleConfigRejectsUnsupportedFrequency(t *testing.T) {
	template := &VitessBackupScheduleTemplate{
		Name:      "unsupported",
		Frequency: "90m",
		Strategy: []VitessBackupScheduleStrategy{{
			Name:     "s1",
			Keyspace: "commerce",
			Shard:    "-",
		}},
	}

	require.Error(t, template.ValidateScheduleConfig(), "ValidateScheduleConfig() error = nil, want error")
}

func TestNewVitessBackupScheduleStatusInitializesMaps(t *testing.T) {
	status := NewVitessBackupScheduleStatus(VitessBackupScheduleStatus{})

	require.NotNil(t, status.LastScheduledTimes, "LastScheduledTimes not initialized")
	require.NotNil(t, status.GeneratedSchedules, "GeneratedSchedules not initialized")
	require.NotNil(t, status.NextScheduledTimes, "NextScheduledTimes not initialized")
}

func TestVitessBackupScheduleTemplateTolerationsJSONSemantics(t *testing.T) {
	var omitted VitessBackupScheduleTemplate
	require.NoError(t, json.Unmarshal([]byte(`{}`), &omitted))
	require.Nil(t, omitted.Tolerations)

	var explicitEmpty VitessBackupScheduleTemplate
	require.NoError(t, json.Unmarshal([]byte(`{"tolerations":[]}`), &explicitEmpty))
	require.NotNil(t, explicitEmpty.Tolerations)
	require.Empty(t, *explicitEmpty.Tolerations)

	var explicitValues VitessBackupScheduleTemplate
	require.NoError(t, json.Unmarshal([]byte(`{"tolerations":[{"key":"backup","operator":"Exists"}]}`), &explicitValues))
	require.NotNil(t, explicitValues.Tolerations)
	require.Equal(t, []corev1.Toleration{{
		Key:      "backup",
		Operator: corev1.TolerationOpExists,
	}}, *explicitValues.Tolerations)
}
