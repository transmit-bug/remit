package builtin_test

import (
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core/schedule"

	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/ext/builtin"
)

// The scheduler plugin answers policy and identity: a family, a version and the
// retention to aim for, and nothing about arithmetic.
func TestSchedulerPolicy(t *testing.T) {
	runner, err := builtin.Scheduler(nil, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer runner.Close()

	var policy struct {
		Family    string  `json:"family"`
		Version   string  `json:"version"`
		Retention float64 `json:"retention"`
	}
	if err := runner.Call("policy", nil, &policy); err != nil {
		t.Fatalf("policy: %v", err)
	}
	if policy.Family != "fsrs" || policy.Version == "" {
		t.Errorf("the policy names %q/%q", policy.Family, policy.Version)
	}
	if policy.Retention <= 0 || policy.Retention > 1 {
		t.Errorf("retention is %v", policy.Retention)
	}

	var identity string
	if err := runner.Call("identity", nil, &identity); err != nil {
		t.Fatalf("identity: %v", err)
	}
	if identity == "" {
		t.Error("the scheduler names itself nothing")
	}
}

// The built-in summary is quiet and says nothing of its own: the host words the
// numbers.
func TestSchedulerSummaryIsQuiet(t *testing.T) {
	runner, err := builtin.Scheduler(nil, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer runner.Close()

	var summary struct {
		Tone  string   `json:"tone"`
		Lines []string `json:"lines"`
	}
	err = runner.Call("summary", map[string]any{"reviewed": 12, "dueTomorrow": 3, "accuracy": 0.9}, &summary)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.Tone != "quiet" {
		t.Errorf("the built-in summary is %q, want quiet", summary.Tone)
	}
	if len(summary.Lines) != 0 {
		t.Errorf("the built-in summary invents lines: %v", summary.Lines)
	}
}

// The plugin's policy is the one the answers were frozen with: the four ratings
// from a fresh card schedule exactly as the default parameters do. If the policy
// ever moves, this fails and the golden file has to be re-frozen on purpose.
func TestSchedulerPolicySchedulesAsTheGoldens(t *testing.T) {
	runner, err := builtin.Scheduler(nil, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer runner.Close()

	var answer struct {
		Retention float64 `json:"retention"`
	}
	if err := runner.Call("policy", nil, &answer); err != nil {
		t.Fatalf("policy: %v", err)
	}
	if answer.Retention != schedule.DefaultRetention {
		t.Fatalf("the plugin aims for %v retention, the frozen answers for %v",
			answer.Retention, schedule.DefaultRetention)
	}

	at := time.Date(2026, time.November, 7, 9, 0, 0, 0, time.UTC)
	plugin := schedule.Params{DesiredRetention: answer.Retention}
	frozen := schedule.DefaultParams()
	for r := schedule.Again; r <= schedule.Easy; r++ {
		got := plugin.Step(schedule.State{}, schedule.Event{Rating: r, At: at})
		want := frozen.Step(schedule.State{}, schedule.Event{Rating: r, At: at})
		if !got.Due.Equal(want.Due) || got.Memory != want.Memory {
			t.Errorf("%v: the plugin's policy schedules %v, the frozen answers %v", r, got.Due, want.Due)
		}
	}
}
