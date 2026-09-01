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
	"testing"
	"time"

	finopsv1alpha1 "github.com/b100to/platform-lab/operators/idle-reaper/api/v1alpha1"
)

// TestEvaluateWindow walks a full day past a 20:00–09:00 window.
//
// Run it with -v to see the table: for each moment it shows the next sleep,
// the next wake, and which of the two arrives first — which is the entire
// decision.
//
//	go test ./internal/controller -run TestEvaluateWindow -v
func TestEvaluateWindow(t *testing.T) {
	spec := finopsv1alpha1.IdleWindowSpec{
		SleepAt:  "0 20 * * *",
		WakeAt:   "0 9 * * *",
		Timezone: "Asia/Seoul",
	}
	seoul, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		clock  string
		asleep bool
		why    string
	}{
		{"03:00", true, "밤중 — 아침 09시 기상이 먼저"},
		{"08:59", true, "기상 1분 전 — 여전히 기상이 먼저"},
		{"09:00", false, "기상 시각 — 이제 저녁 20시 취침이 먼저"},
		{"12:00", false, "한낮"},
		{"19:59", false, "취침 1분 전"},
		{"20:00", true, "취침 시각 — 다시 기상이 먼저"},
		{"23:00", true, "밤"},
	}

	t.Logf("%-7s │ %-11s │ %-11s │ %-8s │ %s",
		"지금", "다음 취침", "다음 기상", "판정", "먼저 오는 쪽")
	t.Logf("────────┼─────────────┼─────────────┼──────────┼──────────────")

	for _, c := range cases {
		now := mustParse(t, seoul, c.clock)

		state, err := evaluateWindow(spec, now)
		if err != nil {
			t.Fatalf("%s: %v", c.clock, err)
		}

		// 같은 값을 다시 계산해 표에 함께 보여준다.
		nextSleep := mustNext(t, spec.SleepAt, now)
		nextWake := mustNext(t, spec.WakeAt, now)

		verdict, first := "깨어 있음", "취침"
		if state.asleep {
			verdict, first = "자는 중", "기상"
		}
		t.Logf("%-7s │ %-11s │ %-11s │ %-8s │ %s이 먼저",
			c.clock,
			nextSleep.Format("01/02 15:04"),
			nextWake.Format("01/02 15:04"),
			verdict, first)

		if state.asleep != c.asleep {
			t.Errorf("%s: asleep=%v, 기대 %v (%s)", c.clock, state.asleep, c.asleep, c.why)
		}
		// next 는 두 경계 중 먼저 오는 쪽이어야 한다.
		want := nextWake
		if !state.asleep {
			want = nextSleep
		}
		if !state.next.Equal(want) {
			t.Errorf("%s: next=%v, 기대 %v", c.clock, state.next, want)
		}
	}
}

// TestEvaluateWindowSurvivesRestart shows why no state is carried between
// reconciles: the answer depends only on the clock, so a controller that has
// just started gets the same verdict as one that has been running all night.
func TestEvaluateWindowSurvivesRestart(t *testing.T) {
	spec := finopsv1alpha1.IdleWindowSpec{
		SleepAt:  "0 20 * * *",
		WakeAt:   "0 9 * * *",
		Timezone: "Asia/Seoul",
	}
	seoul, _ := time.LoadLocation("Asia/Seoul")

	// 컨트롤러가 08시에 죽어 09시 기상을 놓치고 10시에 살아났다고 하자.
	afterRestart := mustParse(t, seoul, "10:00")

	state, err := evaluateWindow(spec, afterRestart)
	if err != nil {
		t.Fatal(err)
	}
	if state.asleep {
		t.Fatal("10시는 깨어 있어야 한다 — 기상 시각을 놓쳤어도 시계만 보면 알 수 있다")
	}
	t.Logf("09시 기상을 놓치고 10시에 재기동해도 판정은 '깨어 있음' — 놓친 이벤트를 몰라도 된다")
}

func mustParse(t *testing.T, loc *time.Location, clock string) time.Time {
	t.Helper()
	parsed, err := time.ParseInLocation("2006-01-02 15:04", "2026-08-26 "+clock, loc)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func mustNext(t *testing.T, expr string, now time.Time) time.Time {
	t.Helper()
	next, err := nextOccurrence(expr, now)
	if err != nil {
		t.Fatal(err)
	}
	return next
}
