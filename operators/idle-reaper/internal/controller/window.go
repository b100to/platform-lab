/*
Copyright 2026.

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

package controller

import (
	"fmt"
	"time"

	// Embed the timezone database. The runtime image is distroless and has no
	// system tzdata, so time.LoadLocation("Asia/Seoul") would fail there while
	// working fine on a developer machine.
	_ "time/tzdata"

	"github.com/robfig/cron/v3"

	finopsv1alpha1 "github.com/b100to/platform-lab/operators/idle-reaper/api/v1alpha1"
)

// windowState is the result of comparing a schedule against the clock.
type windowState struct {
	// asleep is true when now falls inside the idle window.
	asleep bool
	// next is when asleep is expected to flip.
	next time.Time
}

// evaluateWindow decides whether now is inside the idle window.
//
// A cron expression only answers "when does this next fire", never "are we
// between two firings". Rather than search backwards for the previous
// occurrence, compare which boundary comes first from here:
//
//	next wake before next sleep  ->  a wake is pending, so we are asleep
//	next sleep before next wake  ->  a sleep is pending, so we are awake
//
// This holds for any pair of recurring schedules and needs no state carried
// between reconciles, which is what lets a restarted controller recover by
// looking only at the clock.
func evaluateWindow(spec finopsv1alpha1.IdleWindowSpec, now time.Time) (windowState, error) {
	loc, err := time.LoadLocation(spec.Timezone)
	if err != nil {
		return windowState{}, fmt.Errorf("timezone %q: %w", spec.Timezone, err)
	}

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	sleepSched, err := parser.Parse(spec.SleepAt)
	if err != nil {
		return windowState{}, fmt.Errorf("sleepAt %q: %w", spec.SleepAt, err)
	}
	wakeSched, err := parser.Parse(spec.WakeAt)
	if err != nil {
		return windowState{}, fmt.Errorf("wakeAt %q: %w", spec.WakeAt, err)
	}

	local := now.In(loc)
	nextSleep := sleepSched.Next(local)
	nextWake := wakeSched.Next(local)

	state := windowState{asleep: nextWake.Before(nextSleep)}
	if state.asleep {
		state.next = nextWake
	} else {
		state.next = nextSleep
	}
	return state, nil
}

// requeueAfter is how long to wait before the next reconcile.
//
// Waking up exactly on the boundary is fragile: a few milliseconds of clock
// skew lands the controller on the wrong side of it. Add a small margin, and
// cap the interval so a window months away still gets periodic reconciliation
// — drift correction is the whole point of a controller, and it cannot correct
// drift while asleep for a month.
func requeueAfter(now, next time.Time) time.Duration {
	const (
		margin = 5 * time.Second
		maxGap = 10 * time.Minute
		minGap = time.Second
	)

	d := next.Sub(now) + margin
	switch {
	case d > maxGap:
		return maxGap
	case d < minGap:
		return minGap
	default:
		return d
	}
}
