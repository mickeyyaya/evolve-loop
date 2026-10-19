package loopwave

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type admissionCall struct {
	wave  int
	todos []fleet.Todo
	texts map[string]string
	cfg   policy.OrchestratorConfig
}

func fakeAdmission(shadow AdmissionShadow, err error, calls *[]admissionCall) AdmissionFn {
	return func(wave int, todos []fleet.Todo, text func(string) string, cfg policy.OrchestratorConfig) (AdmissionShadow, error) {
		texts := map[string]string{}
		for _, td := range todos {
			texts[td.ID] = text(td.ID)
		}
		*calls = append(*calls, admissionCall{wave: wave, todos: todos, texts: texts, cfg: cfg})
		return shadow, err
	}
}

func threeCardPlan(context.Context, int) ([]byte, []string, error) {
	return []byte(`{"top_n":[{"id":"a","files":["go/internal/inboxrank/a.go"]},{"id":"b","files":["go/internal/quotastate/b.go"]},{"id":"d","files":["go/internal/inboxrank/d.go"]}]}`), nil, nil
}

func orchFC(count int) policy.FleetConfig {
	fc := waveFC(count)
	fc.Orchestrator = policy.OrchestratorConfig{Stage: "shadow", ScanK: 32, HubMinImporters: 120}
	return fc
}

func TestAdmit_ShadowRecordsTheWaitAndChangesNothing(t *testing.T) {
	plain := newHarness(t)
	without := &recorder{}
	if _, err := plain.e.Dispatch(context.Background(), request(7, orchFC(3), pass, threeCardPlan, without)); err != nil {
		t.Fatal(err)
	}

	h := newHarness(t)
	writeJSON(t, filepath.Join(h.evolveDir, "inbox", "d.json"), map[string]any{"id": "d", "title": "Ties in rank", "summary": "touches go/internal/inboxrank"})
	writeJSON(t, filepath.Join(h.evolveDir, "inbox", "b.json"), map[string]any{"id": "b", "title": "Window", "summary": map[string]string{"note": "go/internal/quotastate"}})
	var calls []admissionCall
	shadow := AdmissionShadow{Admitted: 2, Waits: []AdmissionWait{{ID: "d", Rank: 2, Cells: []string{"internal/inboxrank"}, BlockedBy: "a"}}}
	h.e = New(Roots{ProjectRoot: h.root, EvolveDir: h.evolveDir}, h.ports, h.stderr,
		WithSignals(func() *signalcenter.Center { return h.center }), WithAdmission(fakeAdmission(shadow, nil, &calls)))
	with := &recorder{}
	out, err := h.e.Dispatch(context.Background(), request(7, orchFC(3), pass, threeCardPlan, with))
	if err != nil || !out.Ran {
		t.Fatalf("Dispatch = %+v, %v", out, err)
	}

	if !reflect.DeepEqual(with.calls, without.calls) {
		t.Fatalf("the shadow changed the launched lanes:\nwith    %+v\nwithout %+v", with.calls, without.calls)
	}
	if len(calls) != 1 {
		t.Fatalf("admission ran %d times, want once over the planned todos", len(calls))
	}
	if calls[0].wave != 7 {
		t.Errorf("admission got wave %d, want 7: the wave number lets the caller share one module graph per wave", calls[0].wave)
	}
	if got := todoIDs(calls[0].todos); !reflect.DeepEqual(got, []string{"a", "b", "d"}) {
		t.Errorf("admission saw todos %v, want the partition's todos [a b d] in rank order", got)
	}
	if calls[0].texts["d"] != "Ties in rank\ntouches go/internal/inboxrank" || calls[0].texts["a"] != "" ||
		calls[0].texts["b"] != "Window\n{\"note\":\"go/internal/quotastate\"}" {
		t.Errorf("texts = %q, want the inbox title and summary of d, and nothing for a", calls[0].texts)
	}
	if calls[0].cfg != orchFC(3).Orchestrator {
		t.Errorf("cfg = %+v, want the fleet.orchestrator block", calls[0].cfg)
	}

	ev := h.only(t, CodeOrchAdmitWait)
	want := map[string]string{"wave": "7", "todo": "d", "rank": "2", "cells": "internal/inboxrank", "blocked_by": "a", "blocked_by_lane": "1", "stage": "shadow"}
	if !reflect.DeepEqual(ev.Fields, want) {
		t.Errorf("fields = %v, want %v", ev.Fields, want)
	}
	if !strings.Contains(ev.Reason, `"d"`) || !strings.Contains(ev.Reason, `"a"`) {
		t.Errorf("reason = %q, want it to name the waiting todo and its blocker", ev.Reason)
	}
	line := "[loop] wave 7: admission shadow (stage shadow): admission launches 2 lane(s), the file partition launches 3; 1 todo(s) wait for a cell\n"
	if !strings.Contains(h.stderr.String(), line) {
		t.Errorf("stderr = %q, want the line %q", h.stderr.String(), line)
	}
}

func todoIDs(todos []fleet.Todo) []string {
	out := make([]string, len(todos))
	for i, td := range todos {
		out[i] = td.ID
	}
	return out
}

func TestAdmit_TheWouldBeWidthIsCappedByTheCount(t *testing.T) {
	h := newHarness(t)
	var calls []admissionCall
	h.e = New(Roots{ProjectRoot: h.root, EvolveDir: h.evolveDir}, h.ports, h.stderr,
		WithAdmission(fakeAdmission(AdmissionShadow{Admitted: 9, Waits: []AdmissionWait{{ID: "z", BlockedBy: "gone"}}}, nil, &calls)))
	if _, err := h.e.Dispatch(context.Background(), request(2, orchFC(2), pass, twoCardPlan, &recorder{})); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.stderr.String(), "admission launches 2 lane(s), the file partition launches 2; 1 todo(s) wait for a cell") {
		t.Errorf("stderr = %q, want the admission width capped at the count 2", h.stderr.String())
	}
}

func TestAdmit_ABlockerInNoLaneIsReportedAsNone(t *testing.T) {
	if got := laneOf("gone", []fleet.CycleSpec{{Scope: []string{"a"}}}); got != "none" {
		t.Errorf("laneOf = %q, want none", got)
	}
	if got := laneOf("b", []fleet.CycleSpec{{Scope: []string{"a"}}, {Scope: []string{"c", "b"}}}); got != "2" {
		t.Errorf("laneOf = %q, want 2", got)
	}
}

func TestAdmit_AnAdmissionFailureIsOneWarnAndChangesNothing(t *testing.T) {
	h := newHarness(t)
	var calls []admissionCall
	h.e = New(Roots{ProjectRoot: h.root, EvolveDir: h.evolveDir}, h.ports, h.stderr,
		WithSignals(func() *signalcenter.Center { return h.center }), WithAdmission(fakeAdmission(AdmissionShadow{}, errors.New("go list: boom"), &calls)))
	l := &recorder{}
	out, err := h.e.Dispatch(context.Background(), request(3, orchFC(2), pass, twoCardPlan, l))
	if err != nil || !out.Ran || len(l.calls) != 1 || len(l.calls[0]) != 2 {
		t.Fatalf("the wave launches its two lanes: %+v %v %+v", out, err, l.calls)
	}
	if len(*h.events) != 0 {
		t.Errorf("events = %v, want none", h.codes())
	}
	if !strings.Contains(h.stderr.String(), "[loop] WARN: wave 3: admission shadow skipped: go list: boom") {
		t.Errorf("stderr = %q, want one WARN that names the failure", h.stderr.String())
	}
}

func TestAdmit_TheRepairAndAnEngineWithoutAdmissionRunNoShadow(t *testing.T) {
	h := newHarness(t)
	if _, err := h.e.Dispatch(context.Background(), request(1, orchFC(2), pass, twoCardPlan, &recorder{})); err != nil {
		t.Fatal(err)
	}
	var calls []admissionCall
	h.e = New(Roots{ProjectRoot: h.root, EvolveDir: h.evolveDir}, h.ports, h.stderr, WithAdmission(fakeAdmission(AdmissionShadow{}, nil, &calls)))
	if _, err := h.e.ForceOneLane(context.Background(), request(1, orchFC(2), pass, twoCardPlan, &recorder{})); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 0 || strings.Contains(h.stderr.String(), "admission shadow") {
		t.Errorf("calls = %d, stderr = %q: only the wave dispatch runs the shadow", len(calls), h.stderr.String())
	}
}

func TestAdmit_AnUnreadableInboxGivesNoText(t *testing.T) {
	h := newHarness(t)
	inbox := filepath.Join(h.evolveDir, "inbox")
	if err := os.RemoveAll(inbox); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inbox, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	var calls []admissionCall
	h.e = New(Roots{ProjectRoot: h.root, EvolveDir: h.evolveDir}, h.ports, h.stderr, WithAdmission(fakeAdmission(AdmissionShadow{}, nil, &calls)))
	h.e.shadowAdmission(4, 2, orchFC(2).Orchestrator, []fleet.Todo{{ID: "a"}}, nil)()
	if len(calls) != 1 || calls[0].texts["a"] != "" {
		t.Fatalf("calls = %+v, want one call with no text", calls)
	}
	if !strings.Contains(h.stderr.String(), "[loop] WARN: wave 4: admission shadow reads no inbox text:") {
		t.Errorf("stderr = %q, want one WARN for the unreadable inbox", h.stderr.String())
	}
}

func TestCodes_TheAdmitWaitCodeIsRegisteredUnderLoop(t *testing.T) {
	if m, ok := signalcenter.IsRegistered(CodeOrchAdmitWait); !ok || m != signalcenter.ModuleLoop || CodeOrchAdmitWait != "LOOP_ORCH_ADMIT_WAIT" {
		t.Errorf("LOOP_ORCH_ADMIT_WAIT must be registered under loop (got %q, %v)", m, ok)
	}
}

func TestAdmissionWired_ReportsTheOption(t *testing.T) {
	h := newHarness(t)
	if h.e.AdmissionWired() {
		t.Error("an engine without WithAdmission runs no shadow")
	}
	var calls []admissionCall
	if !New(Roots{}, h.ports, h.stderr, WithAdmission(fakeAdmission(AdmissionShadow{}, nil, &calls))).AdmissionWired() {
		t.Error("WithAdmission wires the shadow")
	}
}

type hookLauncher struct {
	inner *recorder
	hook  func()
}

func (l *hookLauncher) Run(ctx context.Context, specs []fleet.CycleSpec) []fleet.Result {
	l.hook()
	return l.inner.Run(ctx, specs)
}

func TestAdmit_ASlowShadowNeverDelaysTheLaunch(t *testing.T) {
	plain := newHarness(t)
	without := &recorder{}
	if _, err := plain.e.Dispatch(context.Background(), request(4, orchFC(2), pass, twoCardPlan, without)); err != nil {
		t.Fatal(err)
	}

	h := newHarness(t)
	release := make(chan struct{})
	started := make(chan struct{})
	slow := func(int, []fleet.Todo, func(string) string, policy.OrchestratorConfig) (AdmissionShadow, error) {
		close(started)
		<-release
		return AdmissionShadow{Admitted: 1}, nil
	}
	h.e = New(Roots{ProjectRoot: h.root, EvolveDir: h.evolveDir}, h.ports, h.stderr, WithAdmission(slow))
	with := &recorder{}
	launched := false
	l := &hookLauncher{inner: with, hook: func() {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Error("the shadow never started")
		}
		launched = true
		close(release)
	}}
	out, err := h.e.Dispatch(context.Background(), request(4, orchFC(2), pass, twoCardPlan, l))
	if err != nil || !out.Ran || !launched {
		t.Fatalf("Dispatch = %+v, %v, launched %v: the launch must run while the shadow is still blocked", out, err, launched)
	}
	if !reflect.DeepEqual(with.calls, without.calls) {
		t.Errorf("the slow shadow changed the launched lanes:\nwith    %+v\nwithout %+v", with.calls, without.calls)
	}
	if !strings.Contains(h.stderr.String(), "admission shadow (stage shadow)") {
		t.Errorf("stderr = %q, want the shadow line once it finishes (joined after the launch)", h.stderr.String())
	}
}

func TestAdmit_APanickingShadowNeverFailsTheLaunch(t *testing.T) {
	plain := newHarness(t)
	without := &recorder{}
	if _, err := plain.e.Dispatch(context.Background(), request(5, orchFC(2), pass, twoCardPlan, without)); err != nil {
		t.Fatal(err)
	}

	h := newHarness(t)
	boom := func(int, []fleet.Todo, func(string) string, policy.OrchestratorConfig) (AdmissionShadow, error) {
		panic("graph exploded")
	}
	h.e = New(Roots{ProjectRoot: h.root, EvolveDir: h.evolveDir}, h.ports, h.stderr, WithAdmission(boom))
	with := &recorder{}
	out, err := h.e.Dispatch(context.Background(), request(5, orchFC(2), pass, twoCardPlan, with))
	if err != nil || !out.Ran {
		t.Fatalf("Dispatch = %+v, %v: a panicking shadow must not fail the wave", out, err)
	}
	if !reflect.DeepEqual(with.calls, without.calls) {
		t.Errorf("the panicking shadow changed the launched lanes:\nwith    %+v\nwithout %+v", with.calls, without.calls)
	}
	if !strings.Contains(h.stderr.String(), "[loop] WARN: wave 5: admission shadow panicked and was dropped: graph exploded") {
		t.Errorf("stderr = %q, want one WARN naming the panic", h.stderr.String())
	}
}
