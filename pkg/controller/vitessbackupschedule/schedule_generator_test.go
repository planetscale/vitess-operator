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

package vitessbackupschedule

import (
	"testing"
	"time"

	"github.com/robfig/cron"
	"github.com/stretchr/testify/require"
)

func requireExactCadence(t *testing.T, expr string, start time.Time, interval time.Duration, iterations int) {
	t.Helper()
	sched, err := cron.ParseStandard(expr)
	require.NoError(t, err)

	prev := sched.Next(start)
	for i := range iterations {
		next := sched.Next(prev)
		require.Equal(t, interval, next.Sub(prev), "iteration %d", i)
		prev = next
	}
}

func TestGenerateCronFromFrequency_Determinism(t *testing.T) {
	freq := 24 * time.Hour
	cron1, err := generateCronFromFrequency(freq, "cluster1", "commerce", "-80", "daily")
	require.NoError(t, err)
	require.Regexp(t, " * * *$", cron1, "unexpected cron expression for daily frequency")
	cron2, err := generateCronFromFrequency(freq, "cluster1", "commerce", "-80", "daily")
	require.NoError(t, err)
	require.Regexp(t, " * * *$", cron2, "unexpected cron expression for daily frequency")
	require.Equal(t, cron1, cron2, "expected deterministic output")
}

func TestGenerateCronFromFrequency_Distribution(t *testing.T) {
	freq := 24 * time.Hour
	shards := []string{"-10", "10-20", "20-30", "30-40", "40-50", "50-60", "60-70", "70-80", "80-90", "90-a0", "a0-b0", "b0-c0", "c0-d0", "d0-e0", "e0-f0", "f0-"}
	results := make(map[string]string)
	for _, shard := range shards {
		expr, err := generateCronFromFrequency(freq, "cluster1", "commerce", shard, "daily")
		require.NoError(t, err, "shard %s", shard)
		results[shard] = expr
	}
	// Different shards should produce different schedules.
	seen := make(map[string]bool)
	unique := true
	for _, expr := range results {
		if seen[expr] {
			unique = false
			break
		}
		seen[expr] = true
	}
	require.True(t, unique, "expected different schedules for different shards")
}

func TestGenerateCronFromFrequency_CommonIntervals(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
	}{
		{"30m", 30 * time.Minute},
		{"1h", 1 * time.Hour},
		{"2h", 2 * time.Hour},
		{"4h", 4 * time.Hour},
		{"6h", 6 * time.Hour},
		{"8h", 8 * time.Hour},
		{"12h", 12 * time.Hour},
		{"24h", 24 * time.Hour},
	}
	start := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := generateCronFromFrequency(tt.duration, "cluster1", "ks1", "-", "sched1")
			require.NoError(t, err)
			require.NotEmpty(t, expr)
			// Verify it's parseable.
			_, err = cron.ParseStandard(expr)
			require.NoError(t, err, "generated cron %q should be parseable", expr)
			requireExactCadence(t, expr, start, tt.duration, 10)
		})
	}
}

func TestGenerateCronFromFrequency_SubMinuteRejects(t *testing.T) {
	_, err := generateCronFromFrequency(30*time.Second, "c", "k", "s", "n")
	require.Error(t, err, "expected error for sub-minute frequency")
}

func TestGenerateCronFromFrequency_RejectsUnsupportedIntervals(t *testing.T) {
	unsupported := []time.Duration{
		45 * time.Minute,
		90 * time.Minute,
		48 * time.Hour,
	}

	for _, interval := range unsupported {
		t.Run(interval.String(), func(t *testing.T) {
			_, err := generateCronFromFrequency(interval, "c", "k", "s", "n")
			require.Error(t, err)
		})
	}
}

func TestGenerateCronFromFrequency_AllOutputsParseable(t *testing.T) {
	// Test many different shards across several intervals to ensure all outputs are valid cron.
	intervals := []time.Duration{
		15 * time.Minute,
		30 * time.Minute,
		1 * time.Hour,
		2 * time.Hour,
		6 * time.Hour,
		12 * time.Hour,
		24 * time.Hour,
	}
	shards := []string{"-", "-80", "80-", "-40", "40-80", "80-c0", "c0-"}
	start := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

	for _, interval := range intervals {
		for _, shard := range shards {
			expr, err := generateCronFromFrequency(interval, "test-cluster", "test-ks", shard, "test-sched")
			require.NoError(t, err, "interval=%s shard=%s", interval, shard)
			_, err = cron.ParseStandard(expr)
			require.NoError(t, err, "interval=%s shard=%s: generated %q should be parseable", interval, shard, expr)
			requireExactCadence(t, expr, start, interval, 5)
		}
	}
}

func TestCronFromInterval_Daily(t *testing.T) {
	// 24h = 1440 minutes, offset of 90 minutes = 1:30 AM.
	expr, err := cronFromInterval(1440, 90)
	require.NoError(t, err)
	require.Equal(t, "30 1 * * *", expr)
}

func TestCronFromInterval_SixHourly(t *testing.T) {
	// 6h = 360 minutes, offset of 45 = minute 45 of the first hour window.
	expr, err := cronFromInterval(360, 45)
	require.NoError(t, err)
	// minute = 45%60 = 45, startHour = (45/60) % 6 = 0.
	require.Equal(t, "45 0/6 * * *", expr)
}

func TestCronFromInterval_Hourly(t *testing.T) {
	// 1h = 60 minutes, offset of 15 = :15 of every hour.
	expr, err := cronFromInterval(60, 15)
	require.NoError(t, err)
	require.Equal(t, "15 * * * *", expr)
}

func TestCronFromInterval_SubHourly(t *testing.T) {
	// 30 minutes, offset of 7 = start at :07, every 30 min.
	expr, err := cronFromInterval(30, 7)
	require.NoError(t, err)
	require.Equal(t, "7/30 * * * *", expr)
}

func TestCronFromInterval_InvalidOffset(t *testing.T) {
	_, err := cronFromInterval(60, 60)
	require.Error(t, err, "expected error for offset >= totalMinutes")

	_, err = cronFromInterval(60, -1)
	require.Error(t, err, "expected error for negative offset")
}
