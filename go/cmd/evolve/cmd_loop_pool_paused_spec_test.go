package main

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

func TestPausedPlan_AnUntriagedScopeLeadsWithItsRecordedDepsPriorityAndContract(t *testing.T) {
	t.Parallel()
	spec := committedset.LaneSpec{DependsOn: []string{"dep-a", "dep-b"}, Priority: 4, OutputContract: "pr"}
	paused := []core.PausedScope{{Cycle: 9, Scope: []string{"u"}, AutoMaxAttempts: 3, Spec: spec}}
	itemFiles := func(core.PausedScope) ([]string, error) { return []string{"u.go"}, nil }

	got := pausedPlan{paused: paused, now: time.Now(), adoptable: adoptAll, itemFiles: itemFiles, warn: io.Discard}.order(nil)

	want := []fleet.Todo{{ID: "u", Files: []string{"u.go"}, DependsOn: []string{"dep-a", "dep-b"}, Priority: 4, OutputContract: "pr"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("backlog = %+v, want %+v", got, want)
	}
}

func TestPausedPlan_ATriagedScopeKeepsTheValuesOfItsTriageTodoOverTheRecordedSpec(t *testing.T) {
	t.Parallel()
	triage := []fleet.Todo{{ID: "t", Files: []string{"t.go"}, DependsOn: []string{"triage-dep"}, Priority: 7, OutputContract: "triage-contract"}}
	spec := committedset.LaneSpec{DependsOn: []string{"recorded-dep"}, Priority: 1, OutputContract: "recorded-contract"}
	paused := []core.PausedScope{{Cycle: 9, Scope: []string{"t"}, AutoMaxAttempts: 3, Spec: spec}}

	got := pausedPlan{paused: paused, now: time.Now(), adoptable: adoptAll, warn: io.Discard}.order(triage)

	if !reflect.DeepEqual(got, triage) {
		t.Fatalf("backlog = %+v, want the triage todo %+v unchanged", got, triage)
	}
}

func TestPausedPlan_AnUntriagedScopeWithNoRecordedSpecLeadsWithOnlyItsIDAndFiles(t *testing.T) {
	t.Parallel()
	paused := []core.PausedScope{{Cycle: 9, Scope: []string{"u"}, AutoMaxAttempts: 3}}
	itemFiles := func(core.PausedScope) ([]string, error) { return []string{"u.go"}, nil }

	got := pausedPlan{paused: paused, now: time.Now(), adoptable: adoptAll, itemFiles: itemFiles, warn: io.Discard}.order(nil)

	if want := []fleet.Todo{{ID: "u", Files: []string{"u.go"}}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("backlog = %+v, want %+v", got, want)
	}
}

func TestPausedPlan_AGrantedScopeWhoseItemLeftTheInboxIsNotPutFirstEvenWithARecordedSpec(t *testing.T) {
	t.Parallel()
	spec := committedset.LaneSpec{DependsOn: []string{"dep"}, Priority: 4, OutputContract: "pr"}
	paused := []core.PausedScope{{Cycle: 9, Scope: []string{"gone"}, AutoMaxAttempts: 3, Spec: spec}}
	evolveDir := t.TempDir()
	writeE2File(t, filepath.Join(evolveDir, "inbox", "2026-11-03T00-00-00Z-other.json"), `{"id":"other","files":["o.go"]}`)
	itemFiles := func(core.PausedScope) ([]string, error) {
		return nil, errors.New("must not be asked: the scope is not adoptable")
	}
	var warn bytes.Buffer

	none := pausedPlan{paused: paused, now: time.Now(), adoptable: inboxAdoptable(evolveDir, nil), itemFiles: itemFiles, warn: &warn}.order(nil)
	triage := []fleet.Todo{{ID: "gone", Files: []string{"g.go"}, Priority: 2}}
	kept := pausedPlan{paused: paused, now: time.Now(), adoptable: inboxAdoptable(evolveDir, nil), itemFiles: itemFiles, warn: io.Discard}.order(triage)

	if len(none) != 0 {
		t.Errorf("no triage todo and no inbox item: backlog = %+v, want none", none)
	}
	if !reflect.DeepEqual(kept, triage) {
		t.Errorf("a triage todo of a scope whose item is gone: backlog = %+v, want its plain triage place %+v", kept, triage)
	}
	if warn.Len() == 0 {
		t.Error("a scope that is not put first writes no warning")
	}
}
