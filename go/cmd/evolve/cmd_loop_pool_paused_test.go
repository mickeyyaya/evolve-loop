package main

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

func TestPausedPlan_AGrantedScopeWhoseInboxItemIsGoneIsNotPutFirst(t *testing.T) {
	t.Parallel()
	triage := []fleet.Todo{{ID: "fresh", Files: []string{"f.go"}}}
	paused := []core.PausedScope{{Cycle: 7, Scope: []string{"gone"}, AutoMaxAttempts: 3}}

	got := pausedPlan{paused: paused, now: time.Now(), adoptable: func(core.PausedScope) error { return errors.New("gone") }, warn: io.Discard}.order(triage)

	if !reflect.DeepEqual(got, triage) {
		t.Fatalf("backlog = %+v, want %+v: a scope with no triage todo and no inbox item has nothing to resume", got, triage)
	}
}

func TestPausedPlan_AnUnreadableResetTimeLeadsAndWarns(t *testing.T) {
	t.Parallel()
	paused := []core.PausedScope{{Cycle: 2147, Scope: []string{"walled"}, AutoMaxAttempts: 3, QuotaResetAt: "soon"}}
	var warn bytes.Buffer
	located := func(core.PausedScope) ([]string, error) { return []string{"w.go"}, nil }

	got := pausedPlan{paused: paused, now: time.Now(), adoptable: adoptAll, itemFiles: located, warn: &warn}.order(nil)

	if want := []fleet.Todo{{ID: "walled", Files: []string{"w.go"}}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("backlog = %+v, want %+v", got, want)
	}
	if !strings.Contains(warn.String(), "paused cycle 2147") || !strings.Contains(warn.String(), `"soon"`) {
		t.Errorf("warn = %q, want a warning that names the cycle and the value", warn.String())
	}
}
