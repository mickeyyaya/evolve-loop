package bridge

// completion_baseline_test.go — the pre-dispatch artifact baseline
// (cycle-1550; red test salvaged from the 1554 lane's ADR-0076 continuation
// snapshot). A correction re-dispatch whose prior failed attempt left its
// report at the canonical path had those UNCHANGED bytes certified as
// completion after two stability ticks — no post-dispatch write required —
// so a stale FAIL verdict re-graded itself on every retry. The contract: an
// observation identical to the pre-dispatch baseline never begins a stability
// window and never completes, the finality concession included; the moment
// the agent actually writes (mtime/size change), everything behaves as before.
//
// newArtifactDetectorAt (the baseline-FREE helper) deliberately stays for the
// existing suite: those tests write files as harness setup for
// "agent-wrote-mid-session" scenarios, which is exactly what an absent
// baseline models. Only these tests construct with a captured baseline, the
// way production does before prompt delivery.

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newArtifactDetectorWithBaselineAt(ws, artifact string) *artifactDetector {
	cfg := &Config{Workspace: ws, Artifact: artifact}
	return &artifactDetector{cfg: cfg, baseline: captureArtifactBaseline(cfg)}
}

// The 1554 lane's red test, verbatim intent: pre-existing artifact, fresh
// detector with the baseline captured — unchanged bytes must never certify;
// the first post-dispatch write begins a normal stability window.
func TestArtifactDetector_PreExistingArtifactRequiresPostDispatchWrite(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "report.md")
	writeArtifact(t, canonical, "verdict: FAIL\n", fixedMTime)
	d := newArtifactDetectorWithBaselineAt(ws, canonical)

	for poll := 1; poll <= artifactStableTicks+1; poll++ {
		ready, _, _, err := d.poll(context.Background())
		if err != nil {
			t.Fatalf("pre-existing poll %d: unexpected error: %v", poll, err)
		}
		if ready {
			t.Fatalf("pre-existing poll %d completed from the unchanged prior artifact; "+
				"a correction re-dispatch must require a post-dispatch write", poll)
		}
	}

	writeArtifact(t, canonical, "verdict: PASS\n", fixedMTime.Add(time.Second))
	if ready, _, _, err := d.poll(context.Background()); ready || err != nil {
		t.Fatalf("first post-dispatch write: got (ready=%v, err=%v), want (false, nil)", ready, err)
	}
	if ready, _, _, err := d.poll(context.Background()); !ready || err != nil {
		t.Fatalf("settled post-dispatch write: got (ready=%v, err=%v), want (true, nil)", ready, err)
	}
}

// The finality concession stops at the baseline: at the buzzer, an artifact
// byte-identical to the pre-dispatch snapshot must NOT complete — timeout is
// the honest outcome for an agent that wrote nothing all session.
func TestArtifactDetector_FinalPollRefusesUnchangedBaseline(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "report.md")
	writeArtifact(t, canonical, "verdict: FAIL\n", fixedMTime)
	d := newArtifactDetectorWithBaselineAt(ws, canonical)

	ctx, cancel := withFinalPoll(context.Background())
	defer cancel()
	if ready, _, _, err := d.poll(ctx); ready || err != nil {
		t.Fatalf("final poll on the unchanged pre-dispatch artifact: got (ready=%v, err=%v), "+
			"want (false, nil) — completing here launders the prior attempt's verdict", ready, err)
	}
}

// …but an artifact the agent DID rewrite keeps the concession: finality still
// completes a post-dispatch write even without a closed stability window.
func TestArtifactDetector_FinalPollStillCompletesPostDispatchWrite(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "report.md")
	writeArtifact(t, canonical, "verdict: FAIL\n", fixedMTime)
	d := newArtifactDetectorWithBaselineAt(ws, canonical)

	writeArtifact(t, canonical, "verdict: PASS\nfinished at the buzzer\n", fixedMTime.Add(time.Second))
	ctx, cancel := withFinalPoll(context.Background())
	defer cancel()
	if ready, _, _, err := d.poll(ctx); !ready || err != nil {
		t.Fatalf("final poll on a rewritten artifact: got (ready=%v, err=%v), want (true, nil) — "+
			"the bounded concession must survive the baseline gate", ready, err)
	}
}

// No pre-dispatch artifact = absent baseline: behavior is byte-identical to
// the pre-fix detector (the 15 existing debounce/finality tests pin the rest).
func TestCaptureArtifactBaseline(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "report.md")
	if b := captureArtifactBaseline(&Config{Workspace: ws, Artifact: canonical}); len(b.entries) != 0 {
		t.Fatalf("absent artifact captured a baseline: %+v", b)
	}
	writeArtifact(t, canonical, "old\n", fixedMTime)
	b := captureArtifactBaseline(&Config{Workspace: ws, Artifact: canonical})
	e, ok := b.entries[canonical]
	if !ok || e.size != 4 || !e.modTime.Equal(fixedMTime) {
		t.Fatalf("baseline = %+v, want an entry with size/mtime of the pre-dispatch file", b)
	}
}

// PRODUCTION-PATH wiring pin (the layer the unit tests cannot see): the
// baseline capture happens inside runTmuxREPL itself, BEFORE prompt delivery,
// and is threaded into the detector. Driven through the full engine
// (LaunchArgs → runTmuxREPL): a stale artifact left at the canonical path by
// a prior failed attempt, with a live session that never writes, must drive
// the run to ExitArtifactTimeout — pre-fix, the detector certified the stale
// bytes after two stability ticks and the run "completed" with the prior
// attempt's verdict (cycle-1550's re-grade loop).
func TestRunTmuxREPL_StalePreDispatchArtifactTimesOutInsteadOfCompleting(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	writeArtifact(t, fx.artifact, "verdict: FAIL\nprior attempt's report\n", fixedMTime)
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}} // boots; agent never writes
	rev := &scriptedReviewer{verdicts: []ReviewVerdict{{Action: ReviewPause, Reason: "agent idle"}}}
	// Explicit REAL capture: the shared runner defaults fakes to
	// zeroBaselineCapture, and this test exists to drive the production gate.
	code, stderr := runTmuxOnStopReview(t, fx, tmux, rev, nil,
		Deps{ArtifactTimeoutS: 2, ArtifactMaxExtends: 5, CaptureBaseline: captureArtifactBaseline}, "--allow-bypass")
	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d — completing here would re-grade the prior attempt's stale "+
			"report as this dispatch's deliverable (cycle-1550); stderr=%q", code, ExitArtifactTimeout, stderr)
	}
}

// Size is load-bearing in the baseline key: on a coarse-mtime filesystem a
// fast post-dispatch write can land inside the same mtime second as the prior
// attempt's report — the size delta is then the only post-dispatch evidence,
// and the write must still certify normally.
func TestArtifactDetector_SameMTimeDifferentSizeWriteIsPostDispatch(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "report.md")
	writeArtifact(t, canonical, "verdict: FAIL\n", fixedMTime)
	d := newArtifactDetectorWithBaselineAt(ws, canonical)

	writeArtifact(t, canonical, "verdict: PASS\nlonger fresh report\n", fixedMTime)
	if ready, _, _, err := d.poll(context.Background()); ready || err != nil {
		t.Fatalf("first sight of the same-mtime rewrite: got (ready=%v, err=%v), want (false, nil)", ready, err)
	}
	if ready, _, _, err := d.poll(context.Background()); !ready || err != nil {
		t.Fatalf("settled same-mtime rewrite: got (ready=%v, err=%v), want (true, nil) — size alone must count as post-dispatch evidence", ready, err)
	}
}

// zeroBaselineCapture declares a harness's pre-seeded artifact to be a stand-in
// for a MID-SESSION write (fake sessions cannot write files): no pre-dispatch
// baseline exists in the modeled scenario.
func zeroBaselineCapture(*Config) artifactBaseline { return artifactBaseline{} }

// The production default must remain the REAL capture: if withDefaults ever
// stops wiring captureArtifactBaseline, the stale-artifact gate silently
// disappears from every live dispatch while explicit-injection tests stay
// green — this pin is the tripwire.
func TestDepsWithDefaults_CaptureBaselineIsReal(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "report.md")
	writeArtifact(t, canonical, "prior\n", fixedMTime)
	d := Deps{}.withDefaults()
	b := d.CaptureBaseline(&Config{Workspace: ws, Artifact: canonical})
	if _, ok := b.entries[canonical]; !ok {
		t.Fatalf("withDefaults CaptureBaseline = %+v, want the real pre-dispatch snapshot", b)
	}
}

// Design-review note 1 (the side door): a pre-dispatch STRAY at a fallback
// location, shadowed by the stale canonical at capture time, must also be
// baselined — if the canonical vanishes mid-session (rm-then-rewrite gap),
// the stray gets located and would otherwise start a fresh window and launder
// the same class through the fallback path.
func TestArtifactDetector_ShadowedFallbackStrayIsAlsoBaselined(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "report.md")
	fallbackDir := filepath.Join(ws, "workspace")
	if err := os.MkdirAll(fallbackDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fallback := filepath.Join(fallbackDir, "report.md")
	writeArtifact(t, canonical, "stale canonical\n", fixedMTime)
	writeArtifact(t, fallback, "stale stray\n", fixedMTime)
	d := newArtifactDetectorWithBaselineAt(ws, canonical)

	if err := os.Remove(canonical); err != nil { // the rm-then-rewrite gap
		t.Fatal(err)
	}
	for poll := 1; poll <= artifactStableTicks+1; poll++ {
		if ready, _, _, err := d.poll(context.Background()); ready || err != nil {
			t.Fatalf("poll %d after canonical vanished: got (ready=%v, err=%v) — the shadowed "+
				"pre-dispatch stray at the fallback must not certify", poll, ready, err)
		}
	}
}
