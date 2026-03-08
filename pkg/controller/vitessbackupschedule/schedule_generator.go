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
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/robfig/cron"
	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

// generateCronFromFrequency generates a deterministic cron expression for a given frequency
// and identity (cluster, keyspace, shard, scheduleName). The same inputs always produce
// the same output, but different shards get staggered across the interval.
func generateCronFromFrequency(frequency time.Duration, cluster, keyspace, shard, scheduleName string) (string, error) {
	if err := planetscalev2.ValidateBackupFrequency(frequency); err != nil {
		return "", err
	}
	totalMinutes := int(frequency.Minutes())

	// Hash the identity to produce a deterministic offset
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%s|%s", cluster, keyspace, shard, scheduleName)
	sum := h.Sum(nil)
	hashVal := binary.BigEndian.Uint64(sum[:8])

	offsetMinutes := int(hashVal % uint64(totalMinutes))
	return cronFromInterval(totalMinutes, offsetMinutes)
}

// cronFromInterval generates a cron expression from a total interval in minutes and an offset.
func cronFromInterval(totalMinutes, offsetMinutes int) (string, error) {
	if totalMinutes < 1 {
		return "", fmt.Errorf("interval must be at least 1 minute, got %d", totalMinutes)
	}
	if offsetMinutes < 0 || offsetMinutes >= totalMinutes {
		return "", fmt.Errorf("offset %d out of range [0, %d)", offsetMinutes, totalMinutes)
	}

	var expr string

	switch {
	case totalMinutes >= 1440 && totalMinutes%1440 == 0:
		// Daily or multi-day interval: run once per day at a specific time.
		// Cap offset to 24h for the time-of-day component.
		dayOffset := offsetMinutes % 1440
		hh := dayOffset / 60
		mm := dayOffset % 60
		if totalMinutes == 1440 {
			expr = fmt.Sprintf("%d %d * * *", mm, hh)
		} else {
			days := totalMinutes / 1440
			expr = fmt.Sprintf("%d %d */%d * *", mm, hh, days)
		}

	case totalMinutes >= 60 && totalMinutes%60 == 0:
		// Hourly intervals (e.g. 1h, 2h, 3h, 4h, 6h, 8h, 12h)
		stepHours := totalMinutes / 60
		mm := offsetMinutes % 60
		startHour := (offsetMinutes / 60) % stepHours
		if stepHours == 1 {
			expr = fmt.Sprintf("%d * * * *", mm)
		} else {
			expr = fmt.Sprintf("%d %d/%d * * *", mm, startHour, stepHours)
		}

	case totalMinutes < 60:
		// Sub-hourly intervals (e.g. 30m, 15m, 10m)
		startMinute := offsetMinutes % totalMinutes
		expr = fmt.Sprintf("%d/%d * * * *", startMinute, totalMinutes)

	default:
		// Non-standard intervals that don't divide evenly into hours or days.
		// Use the offset to pick a specific minute and hour, running daily.
		hh := offsetMinutes / 60
		mm := offsetMinutes % 60
		expr = fmt.Sprintf("%d %d * * *", mm, hh)
	}

	// Validate the generated expression
	if _, err := cron.ParseStandard(expr); err != nil {
		return "", fmt.Errorf("generated invalid cron expression %q from interval=%dm offset=%dm: %v", expr, totalMinutes, offsetMinutes, err)
	}

	return expr, nil
}
