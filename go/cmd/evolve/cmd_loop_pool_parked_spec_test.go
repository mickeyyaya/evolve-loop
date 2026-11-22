package main

import (
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

func TestParked_UntriagedParkedScopeKeepsRecordedSpecWhenResumed(t *testing.T) {
	t.Parallel()
	spec := committedset.LaneSpec{DependsOn: []string{"dep-p"}, Priority: 5, OutputContract: "pr"}
	paused := []core.PausedScope{{Cycle: 11, Scope: []string{"parked-u"}, AutoMaxAttempts: 3, Reason: core.MainRedReason, Spec: spec}}
	itemFiles := func(core.PausedScope) ([]string, error) { return []string{"p.go"}, nil }

	got := pausedPlan{paused: paused, now: time.Now(), adoptable: adoptAll, itemFiles: itemFiles, warn: io.Discard, parkedResume: true}.order(nil)

	want := []fleet.Todo{{ID: "parked-u", Files: []string{"p.go"}, DependsOn: []string{"dep-p"}, Priority: 5, OutputContract: "pr", Resume: true}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("backlog = %+v, want %+v", got, want)
	}
}
