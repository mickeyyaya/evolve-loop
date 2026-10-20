package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
)

func TestWaveSupply_PrintsTheClassesTheRefusalsAndWritesNothing(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	seedSupplyInbox(t, p.evolveDir())
	before := treeBytes(t, p.root)
	h := newWaveHarness()

	rc := h.run("supply", "--project-root", p.root)

	if rc != 0 {
		t.Fatalf("rc = %d, want 0; stderr:\n%s", rc, h.stderr.String())
	}
	out := h.stdout.String()
	for _, want := range []string{
		"supply: 2 admissible of 3 item(s), window 1\n",
		"  admissible         2\n",
		"  console_route      1\n",
		"  console_protected  0\n",
		"refused:\n  console_route  console-x: route:console\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, out)
		}
	}
	if after := treeBytes(t, p.root); !reflect.DeepEqual(before, after) {
		t.Errorf("the verb changed the plane: it must read the inbox and write no record, no signal and no lock\nbefore %v\nafter  %v", before, after)
	}
}

func TestWaveSupply_JSONIsTheCensusWithAllSixClassesAndTheCauseOfEachRefusal(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	seedSupplyInbox(t, p.evolveDir())
	h := newWaveHarness()

	rc := h.run("supply", "--json", "--project-root", p.root)

	var got orchestrator.Supply
	if err := json.Unmarshal(h.stdout.Bytes(), &got); err != nil || rc != 0 {
		t.Fatalf("rc = %d, decode %v:\n%s", rc, err, h.stdout.String())
	}
	want := map[string]int{"admissible": 2, "waiting_cell": 0, "waiting_deps": 0, "console_route": 1, "console_kind": 0, "console_protected": 0}
	if !reflect.DeepEqual(got.Counts, want) || got.Window != 1 || !got.At.Equal(waveTestClock) {
		t.Errorf("census = counts %v window %d at %v, want %v, window 1 at the clock of the verb", got.Counts, got.Window, got.At, want)
	}
	if len(got.Refusals) != 1 || got.Refusals[0].ID != "console-x" || got.Refusals[0].Class != orchestrator.ClassConsoleRoute || got.Refusals[0].Reason != "route:console" {
		t.Errorf("refusals = %+v, want the console route item with its cause", got.Refusals)
	}
}

func TestWaveSupply_WithTheModuleGraphTwoTodosOfOnePackageWaitForTheCell(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	seedSupplyInbox(t, p.evolveDir())
	loads := 0
	h := newWaveHarness()
	h.graph = constantGraph(&loads)

	rc := h.run("supply", "--json", "--project-root", p.root)

	var got orchestrator.Supply
	if err := json.Unmarshal(h.stdout.Bytes(), &got); err != nil || rc != 0 {
		t.Fatalf("rc = %d, decode %v; stderr:\n%s", rc, err, h.stderr.String())
	}
	if got.Counts[orchestrator.ClassAdmissible] != 1 || got.Counts[orchestrator.ClassWaitingCell] != 1 || len(got.Warnings) != 0 || loads != 1 {
		t.Errorf("counts = %v warnings %v loads %d, want 1 admissible and 1 waiting for the alpha cell, no warning, one load", got.Counts, got.Warnings, loads)
	}
}

func TestWaveSupply_AModuleGraphThatDoesNotLoadWarnsAndCountsEveryReadyItem(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	seedSupplyInbox(t, p.evolveDir())
	h := newWaveHarness()
	h.graph = func(context.Context, string, int) (*admission.ModuleGraph, error) {
		return nil, errors.New("go list is gone")
	}

	rc := h.run("supply", "--json", "--project-root", p.root)

	var got orchestrator.Supply
	if err := json.Unmarshal(h.stdout.Bytes(), &got); err != nil || rc != 0 {
		t.Fatalf("rc = %d, decode %v:\n%s", rc, err, h.stdout.String())
	}
	if got.Counts[orchestrator.ClassAdmissible] != 2 || len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "module graph") {
		t.Errorf("counts = %v warnings %v, want both ready items admissible and the record's warning about the module graph", got.Counts, got.Warnings)
	}
	if want := "evolve wave: WARN: the module graph did not load: go list is gone\n"; !strings.Contains(h.stderr.String(), want) {
		t.Errorf("stderr = %q, want %q", h.stderr.String(), want)
	}
}

func TestWaveSupply_AnItemOfAProtectedSurfaceIsAConsoleItemWithItsFile(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	inbox := filepath.Join(p.evolveDir(), "inbox")
	u13WriteJSON(t, filepath.Join(inbox, "lane-a.json"), map[string]any{"id": "lane-a", "weight": 0.9, "priority_class": "feature", "files": []string{"go/internal/alpha/a.go"}})
	u13WriteJSON(t, filepath.Join(inbox, "guard-fix.json"), map[string]any{"id": "guard-fix", "weight": 0.8, "priority_class": "feature", "files": []string{"go/internal/guards/phase.go"}})
	h := newWaveHarness()

	rc := h.run("supply", "--json", "--project-root", p.root)

	var got orchestrator.Supply
	if err := json.Unmarshal(h.stdout.Bytes(), &got); err != nil || rc != 0 {
		t.Fatalf("rc = %d, decode %v:\n%s", rc, err, h.stdout.String())
	}
	if got.Counts[orchestrator.ClassAdmissible] != 1 || got.Counts[orchestrator.ClassConsoleProtected] != 1 {
		t.Fatalf("counts = %v, want 1 admissible and 1 console_protected: the census must judge the protected surface", got.Counts)
	}
	if len(got.Refusals) != 1 || got.Refusals[0].ID != "guard-fix" || !strings.Contains(got.Refusals[0].Reason, "go/internal/guards/phase.go") {
		t.Errorf("refusals = %+v, want guard-fix with the protected file in its reason", got.Refusals)
	}
}

func TestWaveSupply_AnItemWithNoFilesTakesTheCellOfThePathItsTextNames(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	inbox := filepath.Join(p.evolveDir(), "inbox")
	u13WriteJSON(t, filepath.Join(inbox, "lane-a.json"), map[string]any{"id": "lane-a", "weight": 0.9, "priority_class": "feature", "files": []string{"go/internal/alpha/a.go"}})
	u13WriteJSON(t, filepath.Join(inbox, "text-only.json"), map[string]any{"id": "text-only", "weight": 0.8, "priority_class": "feature", "title": "Fix go/internal/alpha/c.go"})
	loads := 0
	h := newWaveHarness()
	h.graph = constantGraph(&loads)

	rc := h.run("supply", "--json", "--project-root", p.root)

	var got orchestrator.Supply
	if err := json.Unmarshal(h.stdout.Bytes(), &got); err != nil || rc != 0 {
		t.Fatalf("rc = %d, decode %v:\n%s", rc, err, h.stdout.String())
	}
	if got.Counts[orchestrator.ClassAdmissible] != 1 || got.Counts[orchestrator.ClassWaitingCell] != 1 || len(got.Refusals) != 1 || got.Refusals[0].ID != "text-only" {
		t.Errorf("counts = %v refusals %+v, want text-only to wait for the alpha cell that lane-a holds: its title names go/internal/alpha", got.Counts, got.Refusals)
	}
}

func TestWaveSupply_AnUnreadableRecurrenceLedgerIsAWarningAndTheCensusGoesOn(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	seedSupplyInbox(t, p.evolveDir())
	p.write(".evolve/recurrence-ledger.json", "{not json")
	h := newWaveHarness()

	rc := h.run("supply", "--json", "--project-root", p.root)

	var got orchestrator.Supply
	if err := json.Unmarshal(h.stdout.Bytes(), &got); err != nil || rc != 0 {
		t.Fatalf("rc = %d, decode %v:\n%s", rc, err, h.stdout.String())
	}
	if !strings.Contains(h.stderr.String(), "[census] WARN: recurrence ledger unreadable") {
		t.Errorf("stderr = %q, want the rank input warning of the broken ledger", h.stderr.String())
	}
	if got.Counts[orchestrator.ClassAdmissible] != 2 {
		t.Errorf("counts = %v, want the census to go on with the ranking it has", got.Counts)
	}
}

func TestWaveSupply_UsageAndPlaneErrors(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		args   []string
		rootEr error
		inbox  bool
		want   int
		stderr string
	}{
		"a stray operand":              {args: []string{"now"}, want: exitUsage, stderr: "evolve wave supply [--json] [--project-root P]"},
		"an unknown flag":              {args: []string{"--bogus"}, want: exitUsage, stderr: "usage: evolve wave next"},
		"a root that does not resolve": {args: nil, rootEr: os.ErrPermission, want: exitIO, stderr: "evolve wave: "},
		"an inbox that is a file":      {args: nil, inbox: true, want: exitIO, stderr: "evolve wave: orchestrator: census:"},
	} {
		t.Run(name, func(t *testing.T) {
			p := newWavePlane(t)
			if tc.inbox {
				p.write(".evolve/inbox", "not a directory")
			}
			h := newWaveHarness()
			h.rootErr = tc.rootEr

			rc := h.run(append(append([]string{"supply"}, tc.args...), "--project-root", p.root)...)

			if rc != tc.want || !strings.Contains(h.stderr.String(), tc.stderr) {
				t.Errorf("rc = %d stderr %q, want %d with %q", rc, h.stderr.String(), tc.want, tc.stderr)
			}
		})
	}
}

func TestWriteWaveSupplyText_PrintsTheProposalsAndTheWarningsOfTheCensus(t *testing.T) {
	t.Parallel()
	s := orchestrator.Supply{
		Window: 4,
		Counts: map[string]int{orchestrator.ClassAdmissible: 1, orchestrator.ClassConsoleProtected: 2},
		Refusals: []orchestrator.Refusal{
			{ID: "g-prot", Class: orchestrator.ClassConsoleProtected, Reason: "protected fix surface: go/internal/guards/g.go"},
			{ID: "h-mixed", Class: orchestrator.ClassConsoleProtected, Reason: "protected fix surface: go/internal/guards/h.go"},
		},
		Proposals: []orchestrator.Proposal{{Item: "h-mixed", Console: "h-mixed-console", Lane: "h-mixed-lane"}},
		Warnings:  []string{"the module graph is not loaded"},
	}
	var out bytes.Buffer

	writeWaveSupplyText(&out, s)

	for _, want := range []string{
		"supply: 1 admissible of 3 item(s), window 4\n",
		"  console_protected  2\n",
		"  console_protected  g-prot: protected fix surface: go/internal/guards/g.go\n",
		"split proposals:\n  h-mixed -> h-mixed-console + h-mixed-lane\n",
		"warnings:\n  the module graph is not loaded\n",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
	var none bytes.Buffer
	writeWaveSupplyText(&none, orchestrator.Supply{Counts: map[string]int{}})
	if strings.Contains(none.String(), "refused:") || strings.Contains(none.String(), "split proposals:") || strings.Contains(none.String(), "warnings:") {
		t.Errorf("an empty census printed a section it has nothing for:\n%s", none.String())
	}
}

func TestWaveSupply_ASkippedInboxFileReachesTheOperatorAsAWarning(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	seedSupplyInbox(t, p.evolveDir())
	inbox := filepath.Join(p.evolveDir(), "inbox")
	if err := os.WriteFile(filepath.Join(inbox, "bad.json"), []byte(`{"id":"gamma","weight":"high"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "num.json"), []byte(`{"id":"numeric","priority":0.86}`), 0o644); err != nil {
		t.Fatal(err)
	}
	h := newWaveHarness()

	rc := h.run("supply", "--project-root", p.root)

	out := h.stdout.String()
	if rc != 0 || !strings.Contains(out, "warnings:\n") || !strings.Contains(out, "  WARN skipped bad.json: ") ||
		!strings.Contains(out, "  WARN num.json: priority 0.86 is a number; it is read as the weight\n") ||
		!strings.Contains(out, "of 4 item(s)") {
		t.Errorf("rc = %d stdout:\n%s", rc, out)
	}
}
