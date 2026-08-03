//go:build acs

// Package cycle1256 materialises the acceptance criteria for this lane's single
// triage-COMMITTED (## top_n) task, artifact-ready-crosspoll-debounce.
//
// SCOPE CORRECTION, stated up front because it changes what these predicates
// gate and what Builder is expected to do.
//
// Scout reported the cross-poll debounce as entirely ABSENT (scout-report.md
// Finding 1: "No such debounce exists in completion.go"). That reading was
// taken against the MAIN tree, which indeed lacks it — `git show
// main:go/internal/bridge/completion.go` has no artifactStableTicks, no
// artifactLocate, and an artifactDetector.poll that completes on first sight.
// THIS lane's worktree is not main. Its base commit (b0d89a71, the ADR-0076
// salvage snapshot) already carries the whole mechanism, landed by cycles
// 1233 (the window) and 1249 (gating the relocation with it):
//
//   - const artifactStableTicks = 2                  completion.go:38-43
//   - the (size, mtime, path) cross-poll window       completion.go:215-270
//   - artifactLocate, the read-only half              driver_common.go:198-…
//   - complete(), the ONLY caller of artifactReady    completion.go:272-…
//   - contract tests                                  completion_debounce_test.go,
//     completion_relocate_stability_test.go
//
// Re-implementing it would stack a second counter on d.stable and double every
// artifact phase's completion latency while closing no defect. So 001-003 are
// VERIFY-AND-CLOSE of the landed work: expected GREEN from the first run, their
// job being to fail loudly if this cycle's edits disturb the sibling contract.
// This is declared, not hidden — see test-report.md's AC-Materialization table.
//
// The RESIDUAL, which is this cycle's actual build work and is genuinely RED.
// Scout's task text has two halves; only the first landed. The second —
// "Update deliverable.go:178-181's comment to describe the debounce accurately"
// — is untouched: that comment is byte-identical to main's and still only NAMES
// the mechanism ("closed at the SOURCE by the bridge artifact-ready cross-poll
// debounce (completion.go)") without describing it. That is the exact doc/code
// drift scout opened the report with, merely inverted: the claim was false when
// written and is now true but unverifiable from the text, so the next reader
// cannot tell which. 004 gates it.
//
// Predicate strategy. artifactDetector, its poll method, artifactReady and the
// wait loop are all UNEXPORTED, so these predicates cannot import them. 001-003
// instead shell `go test -run -v` at the SINGLE named package internal/bridge
// over the contract tests and assert the per-test `--- PASS: <name>` receipt.
// The receipt check is load-bearing anti-gaming: `go test -run <deleted-test>`
// matches nothing and exits 0, so an exit-code-only predicate would go GREEN if
// Builder deleted the contract instead of satisfying it (cycle-1113 lesson).
// ONE named package, -C-anchored, never a `./...` sweep — a whole-repo run is
// the regression suite's job and a false-red generator under fleet load
// (cycles 1173/1175/1178).
//
// Caller proof. 002 drives the REAL production entry point
// (Engine.LaunchArgs → runTmuxREPL → detector.poll, driver_tmux_repl.go), so a
// debounce living on a struct nothing reaches cannot satisfy it. 003 asserts on
// the filesystem SIDE EFFECT (did the fallback move?), not on a return value.
//
// Diversity. 001 carries both negative axes of the window — a still-growing
// file (size) and the same-SIZE rewrite a size-only key is blind to (mtime) —
// plus the never-settles case scout named in AC-4. 003 pairs a negative (an
// in-flight fallback must NOT be moved) with a positive (a settled one MUST
// still be moved, so "never relocate" cannot pass) and a latency axis. 004
// pairs its doc assertion with a code-side existence check, so the comment
// cannot be made to describe a mechanism that is not there.
package cycle1256

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

// TestC1256_001_CrossPollStabilityWindowHolds is the verify-and-close of AC-1
// and AC-4. artifactDetector must not complete on the first sighting of a
// deliverable, must RESET its counter on a still-growing file (size axis — this
// is also scout's AC-4 "never stabilizes" negative: the file changes on every
// one of six consecutive polls and must never report ready), must reset on a
// same-SIZE rewrite (mtime axis, the reason mtime is in the key at all), must
// still short-circuit for the wait loop's final post-cancel look, and its window
// constant must remain a real window rather than being flattened to 1.
//
// Expected GREEN on the first run: landed by cycles 1233/1249, carried in this
// lane's base commit. It fails only if this cycle regresses the shared
// completion.go.
func TestC1256_001_CrossPollStabilityWindowHolds(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestArtifactDetector_ReadyOnlyAfterCrossPollStability",
		"TestArtifactDetector_NotReadyWhileArtifactStillGrowing",
		"TestArtifactDetector_NotReadyOnSameSizeRewrite",
		"TestArtifactDetector_CtxCancelledShortCircuitsDebounce",
		"TestArtifactStableTicks_IsAMeaningfulWindow",
		"TestArtifactDetector_Poll",
	)
	if !ok {
		report(t, "the cross-poll (size, mtime) stability window in artifactDetector.poll no "+
			"longer holds — a deliverable was declared finished on a single non-empty "+
			"observation, or a file that changes on every tick was allowed to settle "+
			"(scout AC-1/AC-4, cycle-1198/1233 regression)", missing, out)
	}
}

// TestC1256_002_DebounceReachableFromProductionWaitLoop is the REACHABILITY
// proof. A debounce implemented on a detector the driver never consults is dead
// code, and a predicate that calls the seam directly would pass on it. This
// drives the real Engine.LaunchArgs → runTmuxREPL → detector.poll path and
// requires that a deliverable rewritten on every tick does NOT exit ExitOK.
// Expected GREEN.
func TestC1256_002_DebounceReachableFromProductionWaitLoop(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestRunTmuxREPL_ArtifactDebounceWiredIntoWaitLoop",
	)
	if !ok {
		report(t, "the artifact stability window is no longer reached from the production "+
			"wait loop (driver_tmux_repl.go) — a churning deliverable completed the "+
			"phase, i.e. the debounce is dead code from the caller's point of view",
			missing, out)
	}
}

// TestC1256_003_RelocationStaysGatedByTheWindow is the verify-and-close of the
// destructive half. The window must gate the RELOCATION of a non-canonical
// fallback, not merely follow it: on relocateFile's copy+remove cross-device
// branch, moving a still-growing fallback snapshots a partial file into the
// canonical path and deletes the source the agent still holds open, leaving a
// permanently stable, permanently TRUNCATED deliverable the window then
// certifies as finished.
//
// Three axes so no degenerate fix passes: negative (in-flight fallback stays
// put), positive (a settled fallback MUST still move, and still carry the
// single-shot "wrote to the wrong place" note), latency (a complete
// single-write fallback must not pay a SECOND stacked window). Expected GREEN.
func TestC1256_003_RelocationStaysGatedByTheWindow(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestArtifactDetector_RelocationDeferredWhileFallbackStillGrowing",
		"TestArtifactDetector_RelocationHappensOnceFallbackSettles",
		"TestArtifactDetector_RelocatedCompleteFallbackStillCompletes",
		"TestArtifactDetector_RelocationNoteSurvivesUntilStable",
	)
	if !ok {
		report(t, "a non-canonical fallback artifact is relocated before the stability "+
			"window closes (artifactDetector.poll → complete → artifactReady). On "+
			"relocateFile's copy+remove branch that truncates the deliverable "+
			"permanently — the window must gate the MOVE, not follow it (cycle-1249)",
			missing, out)
	}
}

// TestC1256_004_DeliverableDocDescribesTheRealMechanism is this cycle's BUILD
// gate and the genuinely RED predicate.
//
// acs-predicate: config-check — WAIVER RATIONALE. This AC is a documentation
// accuracy criterion (scout task text, second half: "Update deliverable.go's
// comment to describe the debounce accurately"). A prose claim has no runtime
// behaviour to invoke, so the assertion is necessarily over the text. It is not
// the cycle-85 degenerate shape for two reasons: (a) it is not the only
// load-bearing predicate in this package — 001-003 are behavioural and carry
// the mechanism itself; (b) the second half below asserts against
// completion.go's CODE, so the comment cannot be greened by describing a
// mechanism that does not exist. Builder cannot satisfy it with a magic string.
//
// Why it is RED. deliverable.go's LAYERING comment is byte-identical to main's
// and reads only: "mid-write truncation is closed at the SOURCE by the bridge
// artifact-ready cross-poll debounce (completion.go)". It names a mechanism
// without describing it, which is precisely the drift scout opened the report
// with (Finding 1) — when that sentence was written the mechanism did not
// exist, and nothing in the text let a reader tell. The fix is to state what
// the debounce actually does: consecutive poll ticks keyed on (size, mtime),
// and that the window gates the relocation.
func TestC1256_004_DeliverableDocDescribesTheRealMechanism(t *testing.T) {
	root := acsassert.RepoRoot(t)
	deliverable := filepath.Join(root, "go", "internal", "deliverable", "deliverable.go")
	completion := filepath.Join(root, "go", "internal", "bridge", "completion.go")

	// Half 1 — the doc must DESCRIBE, not merely name. Each concept is checked
	// with variants so Builder is pinned to the meaning, not to one phrasing.
	concepts := []struct {
		what     string
		variants []string
	}{
		{"the mtime half of the stability key (a size-only window is blind to an equal-length fix-up Edit)",
			[]string{"mtime", "modtime", "modification time"}},
		{"that stability is measured across CONSECUTIVE poll ticks, not within one poll",
			[]string{"consecutive", "successive", "across polls", "cross-poll tick"}},
		{"the named window constant, so the doc points at the real code",
			[]string{"artifactStableTicks"}},
	}
	for _, c := range concepts {
		if !acsassert.FileContainsAny(deliverable, c.variants...) {
			t.Errorf("RED: go/internal/deliverable/deliverable.go still only NAMES the "+
				"cross-poll debounce without describing %s. The comment is byte-identical "+
				"to main's, where the mechanism did not exist — a reader cannot tell a "+
				"true claim from the false one scout found (Finding 1). Expected one of "+
				"%v in the LAYERING comment", c.what, c.variants)
		}
	}

	// Both halves of the stability KEY must appear together on one line, so the
	// doc states what is compared rather than gesturing at it. A bare mention of
	// "size" anywhere in the file is not evidence — the word already occurs in
	// this file's unrelated prose, which is exactly the kind of accidental pass
	// a predicate must not take credit for.
	if !acsassert.LineContainsAll(deliverable, "size", "mtime") {
		t.Errorf("RED: go/internal/deliverable/deliverable.go never states the stability " +
			"KEY. The debounce compares (size, mtime) together — a size-only window is " +
			"blind to an equal-length fix-up Edit, which is why mtime is in the key at " +
			"all (completion.go). Name both on one line")
	}

	// Half 2 — anti-fiction: the mechanism the doc points at must actually be
	// there. This is what makes half 1 un-gameable by prose alone.
	if !acsassert.FileContains(t, completion, "artifactStableTicks") {
		t.Errorf("RED: deliverable.go is required to cite artifactStableTicks, but " +
			"go/internal/bridge/completion.go does not define it — the doc would be " +
			"describing a mechanism that does not exist, which is the cycle-1256 defect " +
			"class inverted rather than fixed")
	}
}
