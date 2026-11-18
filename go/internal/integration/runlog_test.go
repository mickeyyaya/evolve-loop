package integration

import (
	"context"
	"testing"
)

func TestGhCIWatchLogsTheRunItFoundForTheTip(t *testing.T) {
	t.Parallel()
	s := &ghScript{reply: okReply("777")}
	ci := newGhCI(t, s)
	ci.Runs = NewRunLog()
	if _, err := ci.Watch(context.Background(), PR{Tip: sha('a')}); err != nil {
		t.Fatal(err)
	}
	if got := ci.Runs.Run(sha('a')); got != "777" {
		t.Errorf("Run(tip) = %q, want 777", got)
	}
	if got := ci.Runs.Run(sha('b')); got != "" {
		t.Errorf("Run(other tip) = %q, want none", got)
	}
}

func TestGhCIWatchLogsTheRunEvenWhenTheVerdictFails(t *testing.T) {
	t.Parallel()
	s := &ghScript{reply: replyWith("55", "cancelled")}
	ci := newGhCI(t, s)
	ci.Runs = NewRunLog()
	if _, err := ci.Watch(context.Background(), PR{Tip: sha('a')}); err == nil {
		t.Fatal("a cancelled run must fail the watch")
	}
	if got := ci.Runs.Run(sha('a')); got != "55" {
		t.Errorf("Run(tip) = %q, want 55", got)
	}
}

func TestGhCIWatchWithoutARunLogStillWatches(t *testing.T) {
	t.Parallel()
	ci := newGhCI(t, &ghScript{reply: okReply("1")})
	if v, err := ci.Watch(context.Background(), PR{Tip: sha('a')}); err != nil || v != Green {
		t.Fatalf("Watch = %v, %v, want green", v, err)
	}
	var none *RunLog
	if got := none.Run(sha('a')); got != "" {
		t.Errorf("Run on a nil log = %q, want none", got)
	}
}

func TestRunLogKeepsTheLastRunOfATip(t *testing.T) {
	t.Parallel()
	log := NewRunLog()
	log.note(sha('a'), "1")
	log.note(sha('a'), "2")
	if got := log.Run(sha('a')); got != "2" {
		t.Errorf("Run = %q, want the last run 2", got)
	}
}
