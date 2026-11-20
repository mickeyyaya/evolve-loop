package tmuxtest

import (
	"errors"
	"testing"
)

func TestMain_ASocketThatCannotBeSelectedRunsNothingAndFails(t *testing.T) {
	prev := setenv
	t.Cleanup(func() { setenv = prev })
	setenv = func(string, string) error { return errors.New("env frozen") }
	ran := false

	code := Main(runFunc(func() int { ran = true; return 0 }))

	if code != 1 || ran {
		t.Fatalf("Main = %d, ran = %v; want 1 without running the tests", code, ran)
	}
}
