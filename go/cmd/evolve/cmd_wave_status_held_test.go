package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/heldproposal"
)

func TestWaveStatus_ShowsEachCycleHeldForTheOperator(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	p.seedMeasuredRuns()
	evolveDir := filepath.Join(p.root, ".evolve")
	for _, r := range []heldproposal.Record{
		{Cycle: 41, Status: heldproposal.StatusHeld, Tree: "tree41", At: time.Now()},
		{Cycle: 42, Status: heldproposal.StatusWouldHold, Tree: "tree42", At: time.Now()},
	} {
		if err := heldproposal.WriteRecord(evolveDir, r); err != nil {
			t.Fatal(err)
		}
	}

	text := newWaveHarness()
	text.run("status", "--project-root", p.root)
	js := newWaveHarness()
	js.run("status", "--json", "--project-root", p.root)

	if out := text.stdout.String(); !strings.Contains(out, "held:    cycle 41 held for operator (tree tree41)") || strings.Contains(out, "cycle 42 held") {
		t.Errorf("text:\n%s\nwant cycle 41 held and the shadow record of cycle 42 not shown", out)
	}
	var got waveStatusReport
	if err := json.Unmarshal(js.stdout.Bytes(), &got); err != nil || len(got.Held) != 1 || got.Held[0].Cycle != 41 {
		t.Errorf("json held = %+v, %v, want cycle 41 only", got.Held, err)
	}
}

func TestWaveStatus_AnUnreadableHoldRecordIsAWarning(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	p.seedMeasuredRuns()
	held := heldproposal.HeldDir(filepath.Join(p.root, ".evolve"))
	if err := os.MkdirAll(held, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(held, "cycle-5.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := newWaveHarness()
	h.run("status", "--project-root", p.root)
	if out := h.stdout.String(); !strings.Contains(out, "WARN:    held proposals:") {
		t.Errorf("text:\n%s\nwant a held-proposals warning", out)
	}
}
