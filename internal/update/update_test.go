package update

import (
	"context"
	"errors"
	"testing"

	"github.com/transmit-bug/remit/internal/settings"
)

func TestOffSendsNothing(t *testing.T) {
	called := false
	original := check
	check = func(context.Context) (Result, error) {
		called = true
		return Result{Version: "9.9.9"}, nil
	}
	defer func() { check = original }()

	got, err := Check(context.Background(), settings.Settings{})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if called {
		t.Error("the check ran while the setting was off")
	}
	if got.Version != "" {
		t.Errorf("result = %+v, want nothing", got)
	}
}

func TestOnAsksOnce(t *testing.T) {
	calls := 0
	original := check
	check = func(context.Context) (Result, error) {
		calls++
		return Result{Version: "0.2.0", Notes: "fixed things"}, nil
	}
	defer func() { check = original }()

	got, err := Check(context.Background(), settings.Settings{UpdateCheck: true})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if calls != 1 {
		t.Errorf("check ran %d times, want 1", calls)
	}
	if got.Version != "0.2.0" || got.Notes != "fixed things" {
		t.Errorf("result = %+v", got)
	}
}

func TestErrorsSurface(t *testing.T) {
	original := check
	check = func(context.Context) (Result, error) { return Result{}, errors.New("network down") }
	defer func() { check = original }()

	if _, err := Check(context.Background(), settings.Settings{UpdateCheck: true}); err == nil {
		t.Error("Check hid the failure")
	}
}
