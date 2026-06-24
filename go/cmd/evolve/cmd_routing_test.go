package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// WS3-S4 (ADR-0052): `evolve routing explain --cycle N` is a READ-ONLY render
// of a recorded routing decision — the clamped plan (run/skip + justification),
// the integrity-floor clamps that fired, and the OTel decision span — so an
// operator can debug WHY a cycle ran the phases it did. Missing artifacts are a
// clean message, not an error (a partially-recorded cycle still explains).

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	buf, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRoutingExplain_RendersPlanAndClampsForCycle(t *testing.T) {
	t.Parallel()
	pr := t.TempDir()
	ws := filepath.Join(pr, ".evolve", "runs", "cycle-5")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}

	writeJSON(t, filepath.Join(ws, "phase-plan.json"), []router.PhasePlanEntry{
		{Phase: "scout", Run: true, Justification: "fresh discovery work"},
		{Phase: "triage", Run: false, Justification: "carryover already queued"},
	})
	writeJSON(t, filepath.Join(ws, "routing-decision-1.json"), router.RouterDecision{
		NextPhase: "audit",
		Clamps:    []router.Clamp{{Rule: "ship-requires-audit", Proposed: "ship", Forced: "audit"}},
	})
	writeJSON(t, filepath.Join(ws, "advisor-span-plan.json"), core.AdvisorSpan{
		Model: "opus", System: "claude", PromptSHA: "deadbeefprompt", ResponseSHA: "cafef00dresp", DurationMS: 1234,
	})

	var out, errb bytes.Buffer
	code := runRouting([]string{"explain", "--cycle", "5", "--project-root", pr}, nil, &out, &errb)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, errb.String())
	}
	s := out.String()
	for _, want := range []string{
		"scout", "RUN", "fresh discovery work", // plan: run entry
		"triage", "SKIP", "carryover already queued", // plan: skip entry
		"ship-requires-audit", "ship", "audit", // clamp
		"opus", "claude", "deadbeefprompt", "1234", // span
	} {
		if !bytes.Contains([]byte(s), []byte(want)) {
			t.Errorf("explain output missing %q:\n%s", want, s)
		}
	}
}

func TestRoutingExplain_MissingArtifactsAreCleanExitZero(t *testing.T) {
	t.Parallel()
	pr := t.TempDir() // no .evolve/runs/cycle-9 at all

	var out, errb bytes.Buffer
	code := runRouting([]string{"explain", "--cycle", "9", "--project-root", pr}, nil, &out, &errb)
	if code != 0 {
		t.Fatalf("a cycle with no recorded artifacts must still exit 0, got %d; stderr=%s", code, errb.String())
	}
	// A clean "nothing recorded" message, not a crash or an error.
	if !bytes.Contains(out.Bytes(), []byte("no phase plan")) {
		t.Errorf("want a clean 'no phase plan' message for an unrecorded cycle:\n%s", out.String())
	}
}

// WS3-S5: `evolve routing replay --cycle N` reparses the captured response and
// compares its run-set to the recorded phase-plan.json — MATCH (exit 0) when
// the capture still reproduces the recorded plan, MISMATCH (non-zero) when it
// diverges (a tampered/corrupted capture, or a prompt/model regression).
func TestRoutingReplay_MatchesRecordedRunSet(t *testing.T) {
	t.Parallel()
	pr := t.TempDir()
	ws := filepath.Join(pr, ".evolve", "runs", "cycle-7")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	// A no-build, no-ship plan: the floor imposes nothing, so clamped == parsed.
	raw := `[{"phase":"scout","run":true,"justification":"x"},{"phase":"triage","run":false,"justification":"queued"}]`
	if err := os.WriteFile(filepath.Join(ws, "advisor-response-plan.txt"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(ws, "phase-plan.json"), []router.PhasePlanEntry{
		{Phase: "scout", Run: true, Justification: "x"},
		{Phase: "triage", Run: false, Justification: "queued"},
	})

	var out, errb bytes.Buffer
	if code := runRouting([]string{"replay", "--cycle", "7", "--project-root", pr}, nil, &out, &errb); code != 0 {
		t.Fatalf("replay of a faithful capture must MATCH (exit 0), got %d; out=%s err=%s", code, out.String(), errb.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("MATCH")) {
		t.Errorf("want MATCH in output:\n%s", out.String())
	}

	// Now a recorded plan whose run-set DIVERGES from the capture → MISMATCH.
	writeJSON(t, filepath.Join(ws, "phase-plan.json"), []router.PhasePlanEntry{
		{Phase: "scout", Run: false, Justification: "tampered"},
	})
	out.Reset()
	errb.Reset()
	if code := runRouting([]string{"replay", "--cycle", "7", "--project-root", pr}, nil, &out, &errb); code == 0 {
		t.Errorf("a diverging recorded plan must be MISMATCH (non-zero); out=%s", out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("MISMATCH")) {
		t.Errorf("want MISMATCH in output:\n%s", out.String())
	}
}

func TestRoutingReplay_MissingCaptureIsCleanExitZero(t *testing.T) {
	t.Parallel()
	pr := t.TempDir() // no capture at all
	var out, errb bytes.Buffer
	if code := runRouting([]string{"replay", "--cycle", "3", "--project-root", pr}, nil, &out, &errb); code != 0 {
		t.Fatalf("no captured response ⇒ clean exit 0, got %d", code)
	}
	if !bytes.Contains(out.Bytes(), []byte("no captured")) {
		t.Errorf("want a clean 'no captured response' message:\n%s", out.String())
	}
}

func TestRouting_RequiresCycleAndKnownSubcommand(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	if code := runRouting([]string{"explain", "--project-root", t.TempDir()}, nil, &out, &errb); code == 0 {
		t.Error("explain without --cycle must be a usage error (non-zero)")
	}
	errb.Reset()
	if code := runRouting([]string{"bogus"}, nil, &out, &errb); code == 0 {
		t.Error("unknown subcommand must be a usage error (non-zero)")
	}
}
