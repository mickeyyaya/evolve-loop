//go:build acs

// Package cycle1252 materialises the acceptance criteria for this lane's single
// triage-COMMITTED (## top_n) task, artifact-ready-crosspoll-debounce.
//
// SCOPE, stated up front because it decides what is RED and what is a
// verify-and-close. Scout read the defect against the MAIN tree, which does
// lack the debounce entirely. In THIS lane's worktree the fix is already
// present at HEAD (5741bdba, the ADR-0076 continuation-on-fail salvage
// snapshot): const artifactStableTicks = 2 (completion.go:38-43), the
// cross-poll (path, size, mtime) window (:215-266), the window-GATED
// relocation in complete() (:275-285), the live final-poll context in the
// wait loop (driver_tmux_repl.go:583-590), and the three contract test files
// (completion_debounce_test.go, completion_relocate_stability_test.go,
// completion_cancel_parity_test.go). This cycle's work is therefore
// "verify, correct, land", not "implement" — and predicates 001-004 are
// verify-and-close of the landed behaviour, expected GREEN from the first run.
// Their job is to fail loudly if this cycle's edits to the shared
// completion.go / driver_tmux_repl.go disturb any of it before it lands.
//
// The genuine RED is 005: completion.go's finality short-circuit still
// documents itself through the WRONG function. The comment claims the check is
// "Checked AFTER artifactReady so finality can never manufacture completion
// from nothing", but the code checks finality after artifactLocate (:226) and
// reaches artifactReady only later, via complete(). The guarantee holds — a
// found artifact is already non-empty — but it is asserted through a function
// the code does not call there. That is the SAME defect class this cycle
// exists to close: deliverable.go:179-181 claimed a source-side debounce that
// did not exist. Shipping the debounce while leaving a second stale
// forward-reference in the file it points at would reproduce the bug in
// miniature.
//
// Predicate strategy. artifactDetector, its poll method, artifactReady,
// artifactLocate and the wait loop are ALL UNEXPORTED, so these predicates
// cannot import them. 001-004 each shell `go test -run -v` at the SINGLE named
// package ./internal/bridge/ over the contract tests and assert the per-test
// `--- PASS: <name>` receipt. The receipt check is load-bearing anti-gaming:
// `go test -run <deleted-test>` matches nothing and exits 0, so an
// exit-code-only predicate would go GREEN if the contract were deleted rather
// than satisfied (cycle-1113 lesson). ONE named package, always narrowed by
// -run, never a `./...` sweep — a whole-repo run is the regression suite's job
// and a false-red generator under fleet load (cycles 1173/1175/1178).
//
// Caller proof. 003 drives the REAL production entry point
// (Engine.LaunchArgs -> runTmuxREPL -> detector.poll, driver_tmux_repl.go:613
// for the main loop and :586 for the post-cancel final poll), so a window
// living on a struct nothing reaches cannot satisfy it. Both paths are
// covered: wired into one only is the same defect.
//
// Diversity. 001 carries the negative axes (a still-growing file must NOT
// complete; a same-SIZE rewrite must NOT complete — the reason mtime is in the
// key at all). 002 pairs a negative (an in-flight fallback must NOT be moved)
// with a positive (a settled one MUST still be moved, so "never relocate"
// cannot pass). 003 pairs the wiring proof with the cancel-parity edge, where
// over-strictness turns the fix into a worse false-FAIL generator
// (ExitArtifactTimeout on a delivered phase, cycle-1236). 004 is the
// boundary/OOD axis on the window constant itself.
package cycle1252

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// bridgePkg is the ONE named package these predicates run.
const bridgePkg = "./internal/bridge/"

// goDir is the worktree's go module root — predicates read the CYCLE's source,
// not main's (worktree isolation; acsassert.RepoRoot resolves the worktree).
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// runContract runs `go test -C <worktree>/go -v -run ^(names...)$ ./internal/bridge/`
// and reports whether EVERY named test both ran and passed.
//
// Two failure shapes are distinguished deliberately:
//   - code < 0 is a genuine "could not launch" and is fatal (cycle-574 lesson);
//     a compile failure in the target package — an expected RED signal before
//     the implementation exists — is a NON-ZERO EXIT, not a launch failure.
//   - a zero exit with a missing `--- PASS: <name>` receipt means the test is
//     gone, not that it passed. That is reported as a miss, not a pass.
func runContract(t *testing.T, names ...string) (ok bool, missing []string, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go",
		"test", "-C", goDir(t), "-count=1", "-v",
		"-run", "^("+strings.Join(names, "|")+")$", bridgePkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to LAUNCH for %s: code=%d err=%v\n%s", bridgePkg, code, err, tail(out, 30))
	}
	for _, n := range names {
		if !strings.Contains(out, "--- PASS: "+n) {
			missing = append(missing, n)
		}
	}
	return code == 0 && len(missing) == 0, missing, out
}

// tail returns the last n lines — diagnostics stay readable in the verdict.
func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// report renders a uniform RED message naming both halves of a failure: the
// contract that did not hold, and any test that vanished rather than passing.
func report(t *testing.T, what string, missing []string, out string) {
	t.Helper()
	if len(missing) > 0 {
		t.Errorf("RED: %s — and these contract tests did not report PASS (deleted or skipped): %v\n%s",
			what, missing, tail(out, 40))
		return
	}
	t.Errorf("RED: %s\n%s", what, tail(out, 40))
}

// TestC1252_001_CrossPollStabilityWindowGatesReady verifies the core acceptance
// criterion: artifactDetector must not report ready on the FIRST sighting of a
// non-empty deliverable. It must observe the same (path, size, mtime) for
// artifactStableTicks consecutive ticks, resetting on a still-growing file
// (size axis) and on a same-SIZE rewrite (mtime axis — a fix-up Edit of equal
// length is exactly what a size-only key is blind to).
//
// Verify-and-close of the salvaged half: expected GREEN on the first run,
// RED against the main tree, and RED again if this cycle's edits to the shared
// completion.go flatten the window back to first-sight completion (cycle-1198).
func TestC1252_001_CrossPollStabilityWindowGatesReady(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestArtifactDetector_ReadyOnlyAfterCrossPollStability",
		"TestArtifactDetector_NotReadyWhileArtifactStillGrowing",
		"TestArtifactDetector_NotReadyOnSameSizeRewrite",
	)
	if !ok {
		report(t, "artifactDetector no longer requires a cross-poll (size, mtime) stability "+
			"window before ready=true — a still-growing or mid-Edit deliverable can complete "+
			"the phase on first sight, which is the cycle-1198 truncated-read defect this "+
			"cycle exists to close", missing, out)
	}
}

// TestC1252_002_RelocationIsGatedByTheWindow verifies that the window gates the
// RELOCATION of a non-canonical fallback rather than merely following it.
//
// Negative axis — a fallback still being appended to must be left exactly where
// the agent put it. Relocating mid-write is the worst variant of the bug:
// relocateFile's cross-device branch copies a PARTIAL file to the canonical
// path and then REMOVES the source the agent still holds open, leaving a
// permanently stable, permanently truncated deliverable that the window then
// certifies as finished. Positive axis — a fallback that has settled must STILL
// be relocated and must still carry its single-shot diagnostic note, so
// "never relocate" cannot pass. Latency axis — a complete single-write fallback
// must not pay a SECOND stacked window.
func TestC1252_002_RelocationIsGatedByTheWindow(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestArtifactDetector_RelocationDeferredWhileFallbackStillGrowing",
		"TestArtifactDetector_RelocationHappensOnceFallbackSettles",
		"TestArtifactDetector_RelocatedCompleteFallbackStillCompletes",
		"TestArtifactDetector_RelocationNoteSurvivesUntilStable",
	)
	if !ok {
		report(t, "a non-canonical fallback artifact is relocated before the stability window "+
			"closes. artifactReady (the destructive mover) must be reachable ONLY from "+
			"artifactDetector.complete(); relocating on first sight snapshots a partial file "+
			"into the canonical path and deletes the source still being written",
			missing, out)
	}
}

// TestC1252_003_WindowReachableFromProductionWaitLoop is the caller proof, and
// it covers BOTH production poll sites, not one.
//
// Main loop (driver_tmux_repl.go:613): a deliverable rewritten on every tick
// must NOT exit ExitOK — a window on a detector the driver never consults is
// dead code. Post-cancel final poll (:586): the detector receives a LIVE,
// finalPollGrace-bounded context carrying the explicit finality marker, and a
// session that delivered before the cancel must complete rather than laundering
// into ExitArtifactTimeout. That second half is the regression-sensitive edge —
// an over-strict window turns a truncated-read fix into a worse false-FAIL
// generator, and the stdout/git contracts (which shell CapturePane and git, and
// cannot fork on a dead context) exited 81 on delivered phases before it
// (cycle-1236).
func TestC1252_003_WindowReachableFromProductionWaitLoop(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestRunTmuxREPL_ArtifactDebounceWiredIntoWaitLoop",
		"TestArtifactDetector_CtxCancelledShortCircuitsDebounce",
		"TestTmuxREPL_StdoutContract_CancelAfterIdle_CompletesNotTimeout",
		"TestTmuxREPL_StdoutContract_CancelWhileStreaming_StillTimesOut",
		"TestTmuxREPL_GitContract_CancelAfterEvidenceCommit_CompletesNotTimeout",
		"TestTmuxREPL_GitContract_CancelWithoutEvidenceCommit_StillTimesOut",
	)
	if !ok {
		report(t, "the artifact stability window is not reached from BOTH production poll sites "+
			"(driver_tmux_repl.go:613 main loop, :586 post-cancel final poll), or the final "+
			"poll regressed: a delivered session must complete at the buzzer instead of "+
			"exiting ExitArtifactTimeout, and an undelivered one must still time out",
			missing, out)
	}
}

// TestC1252_004_StableTicksIsAMeaningfulWindow is the boundary axis on the
// window constant. artifactStableTicks must remain >= 2: one observation is
// just the legacy first-sight read, so flattening the constant to 1 disables
// the fix while leaving every structural trace of it in place — the cheapest
// way to make 001 pass without the behaviour. It must also stay a compiled
// constant (the readGraceWindow precedent in deliverable.go), not a flag or a
// phase setting: an I/O robustness bound is not an operator dial.
func TestC1252_004_StableTicksIsAMeaningfulWindow(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestArtifactStableTicks_IsAMeaningfulWindow",
	)
	if !ok {
		report(t, "artifactStableTicks is no longer a real window (>= 2 consecutive unchanged "+
			"observations). At 1 the detector is byte-equivalent to the legacy first-sight "+
			"path and the debounce is decorative", missing, out)
	}
}

// staleFinalityRef matches completion.go's finality short-circuit documenting
// itself through artifactReady — the function the code does NOT call at that
// point. \s* spans the comment's line wrap and its `//` continuation marker.
var staleFinalityRef = regexp.MustCompile(`Checked AFTER\s*(?://\s*)?artifactReady`)

// correctFinalityRef matches the same sentence naming artifactLocate, which is
// what actually precedes the check and what actually supplies the guarantee.
var correctFinalityRef = regexp.MustCompile(`Checked AFTER\s*(?://\s*)?artifactLocate`)

// TestC1252_005_FinalityShortCircuitDocumentsTheRightGuard is this cycle's
// genuinely RED predicate.
//
// completion.go's finality short-circuit (:242) explains why it cannot
// manufacture completion from nothing by asserting it is "Checked AFTER
// artifactReady". It is not: poll checks finality after artifactLocate (:226),
// and artifactReady is reached only later through complete() (:276) — an
// ordering the cycle-1249 relocation gate deliberately established. The
// guarantee itself is intact (artifactLocate's found already proves a non-empty
// file), but it is attributed to the wrong function.
//
// This is the exact defect class the cycle is closing. deliverable.go:179-181
// pointed at "the bridge artifact-ready cross-poll debounce (completion.go)"
// for a mechanism that did not exist; landing the debounce makes that sentence
// true, and it would be incoherent to make one forward-reference honest while
// leaving a second stale one inside the file it points at. A reader who trusts
// this comment looks for the ordering invariant at the wrong seam and can
// reintroduce first-sight relocation without noticing they broke it.
//
// Not gameable by adding a magic string: the assertion is that the FALSE claim
// is gone AND the true one is present. Deleting the sentence outright fails the
// second half; the only passing edit is the correction.
func TestC1252_005_FinalityShortCircuitDocumentsTheRightGuard(t *testing.T) {
	path := filepath.Join(goDir(t), "internal", "bridge", "completion.go")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}
	if staleFinalityRef.Match(src) {
		t.Errorf("RED: completion.go's finality short-circuit still claims it is checked after "+
			"artifactReady. poll() checks finality after artifactLocate (:226); artifactReady "+
			"is reached only via complete() (:276). Correct the reference — a comment asserting "+
			"a guarantee through a function the code does not call there is the same stale "+
			"forward-reference defect as deliverable.go:179-181, which this cycle closes (%s)", path)
	}
	if !correctFinalityRef.Match(src) {
		t.Errorf("RED: completion.go's finality short-circuit does not state which guard "+
			"actually precedes it. The sentence must name artifactLocate, whose found result is "+
			"what proves a non-empty artifact exists before finality short-circuits. Deleting "+
			"the explanation is not a fix: the ordering invariant (locate during the window, "+
			"relocate only on close) is load-bearing and must stay documented at the seam (%s)", path)
	}
}
