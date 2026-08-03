//go:build acs

// Package cycle1249 materialises the acceptance criteria for this lane's single
// triage-COMMITTED (## top_n) task, artifact-ready-crosspoll-debounce.
//
// SCOPE CORRECTION, stated up front because it changes what these predicates
// gate. Scout reported the cross-poll debounce as entirely absent. That reading
// was taken against the MAIN tree, which indeed lacks it. In THIS lane's
// worktree the debounce is already present at HEAD (0d879c78 carries
// const artifactStableTicks + the (size, mtime) window at completion.go:38-43,
// :248-265, folded in from cycle-1233 by 841e676f), it is wired into the
// production wait loop (withFinalPoll, driver_tmux_repl.go:585), it is covered
// by completion_debounce_test.go, and cycle-1236 predicate 004 already
// verify-and-closed it. Re-implementing it would stack a second counter on
// d.stable and double every artifact phase's completion latency for no defect
// closed. So 001-002 below are VERIFY-AND-CLOSE of the landed half — expected
// GREEN from the first run, their job being to fail loudly if this cycle's edits
// to the shared completion.go disturb it.
//
// The RESIDUAL, which is this cycle's actual build work and is genuinely RED.
// artifactDetector.poll opens with artifactReady(d.cfg) (completion.go:223), and
// artifactReady RELOCATES a non-canonical fallback the instant it sees it
// non-empty (driver_common.go, the cycle-108/141 tolerance) — before a single
// stability observation has been made about that fallback. The window therefore
// runs strictly DOWNSTREAM of an irreversible move. On relocateFile's rename
// branch that survives (the agent's open fd follows the inode). On its
// copy+remove cross-device branch it does not: a partial file is snapshotted
// into the canonical path and the source the agent is still appending to is
// deleted, leaving a permanently stable, permanently TRUNCATED deliverable that
// the debounce then declares finished. That is the very mid-write-truncation
// class deliverable.go:180 names this mechanism as the source-side closure of,
// reached through the path scout independently flagged (hypothesis 2) as
// highest-risk because it carries the extra copy step. Predicate 003 gates it.
//
// Predicate strategy. artifactDetector, its poll method, artifactReady and the
// wait loop are all UNEXPORTED, so these predicates cannot import them. Each
// instead shells `go test -run -v` at the SINGLE named package internal/bridge
// over the contract tests, and asserts the per-test `--- PASS: <name>` receipt.
// The receipt check is load-bearing anti-gaming: `go test -run <deleted-test>`
// matches nothing and exits 0, so an exit-code-only predicate would go GREEN if
// Builder deleted the contract instead of satisfying it (cycle-1113 lesson).
// ONE named package, never a `./...` sweep — a whole-repo run is the regression
// suite's job and a false-red generator under fleet load (cycles 1173/1175/1178).
//
// Caller proof. 002 drives the REAL production entry point
// (Engine.LaunchArgs → runTmuxREPL → detector.poll at driver_tmux_repl.go:601),
// so a debounce living on a struct nothing reaches cannot satisfy it. 003's
// fault site IS artifactDetector.poll's own first statement — the seam and the
// caller are the same line — and it asserts on the filesystem SIDE EFFECT (did
// the file move?) rather than on the return value, which is already correct
// today; that is what makes it un-passable by tuning the counter.
//
// Diversity. 003 pairs a negative (an in-flight fallback must NOT be moved) with
// a positive (a settled one MUST still be moved, so "never relocate" is not a
// passing fix) and a latency regression axis (gating the move must not stack a
// second window). 001 carries both negative axes of the landed window — a
// still-growing file, and the same-SIZE rewrite a size-only key is blind to.
package cycle1249

import (
	"path/filepath"
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
//     Builder implements — is a NON-ZERO EXIT, not a launch failure.
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

// TestC1249_001_CanonicalPathDebounceStillHolds is the verify-and-close of the
// landed half of the task: artifactDetector must not complete on the first
// sighting of a deliverable at the canonical path, must reset its counter on a
// still-growing file (size axis), must reset on a same-SIZE rewrite (mtime axis,
// the reason mtime is in the key at all), must still short-circuit for the wait
// loop's final post-cancel look, and its window constant must remain a real
// window rather than being flattened to 1. Expected GREEN on the first run.
func TestC1249_001_CanonicalPathDebounceStillHolds(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestArtifactDetector_ReadyOnlyAfterCrossPollStability",
		"TestArtifactDetector_NotReadyWhileArtifactStillGrowing",
		"TestArtifactDetector_NotReadyOnSameSizeRewrite",
		"TestArtifactDetector_CtxCancelledShortCircuitsDebounce",
		"TestArtifactStableTicks_IsAMeaningfulWindow",
	)
	if !ok {
		report(t, "the landed cross-poll (size, mtime) stability window at the canonical "+
			"path no longer holds — this cycle's edits to the shared completion.go "+
			"regressed the sibling contract (cycle-1198/1233)", missing, out)
	}
}

// TestC1249_002_DebounceReachableFromProductionWaitLoop is the reachability
// proof for the landed half. A debounce implemented on a detector the driver
// never consults is dead code; this drives the real
// Engine.LaunchArgs → runTmuxREPL → detector.poll path and requires that a
// deliverable rewritten on every tick does NOT exit ExitOK. Expected GREEN.
func TestC1249_002_DebounceReachableFromProductionWaitLoop(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestRunTmuxREPL_ArtifactDebounceWiredIntoWaitLoop",
	)
	if !ok {
		report(t, "the artifact stability window is no longer reached from the production "+
			"wait loop (driver_tmux_repl.go) — a churning deliverable completed the phase",
			missing, out)
	}
}

// TestC1249_003_RelocationGatedByStabilityWindow is this cycle's BUILD gate and
// the genuinely RED predicate: the stability window must gate the relocation of
// a non-canonical fallback, not merely follow it.
//
// Negative axis — a fallback still being appended to must be left exactly where
// it is; relocating it mid-write is what truncates the deliverable on
// relocateFile's copy+remove branch (the source is removed while the agent still
// holds it open). Positive axis — a fallback that has settled must STILL be
// relocated and still carry the single-shot "wrote to the wrong place"
// diagnostic, so "never relocate" cannot pass. Latency axis — the common
// complete-single-write fallback must not pay a SECOND stacked window.
func TestC1249_003_RelocationGatedByStabilityWindow(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestArtifactDetector_RelocationDeferredWhileFallbackStillGrowing",
		"TestArtifactDetector_RelocationHappensOnceFallbackSettles",
		"TestArtifactDetector_RelocatedCompleteFallbackStillCompletes",
		"TestArtifactDetector_RelocationNoteSurvivesUntilStable",
	)
	if !ok {
		report(t, "a non-canonical fallback artifact is still relocated on FIRST SIGHT, before "+
			"any stability observation (artifactDetector.poll → artifactReady, "+
			"completion.go:223). On relocateFile's copy+remove branch that snapshots a "+
			"partial file into the canonical path and deletes the source the agent is "+
			"still writing — a permanently stable, permanently truncated deliverable. "+
			"The window must gate the MOVE", missing, out)
	}
}
