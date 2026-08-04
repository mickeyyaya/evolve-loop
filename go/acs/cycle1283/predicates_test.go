//go:build acs

// Package cycle1283 materialises the acceptance criteria for the single
// fleet-scoped task pinned to this lane: `retro-fleet-stale-worktree-fallback`
// — the verified-OPEN half of the cycle-1255 CRITICAL that the
// 1255→1268→1270→1272 salvage chain progressively narrowed to the EMPTY-worktree
// shape and then sealed as "verified closed" in the CHANGELOG (68322bdf).
//
// The defect. A torn-down (or never-provisioned) fleet lane leaves
// cs.ActiveWorktree NON-EMPTY but pointing at a directory that no longer exists
// (cyclerun.go:456 is the sole assignment). retroWorktree's original guard fired
// only on the empty string, so the dead path passed through verbatim into the
// BridgeRequest, where the fleet driver's isDir() check refuses the launch
// (driver_tmux_repl.go, ExitBadFlags, stderr only, no error return) — the lane
// loses its retrospective entirely. A failure in the failure-handler.
//
// Predicate strategy — why these drive Phase.Run and not retroWorktree.
// retroWorktree is unexported, and a predicate that called it would prove only
// that a helper behaves; the value the guard actually reads is
// core.BridgeRequest.Worktree, produced by the exported Run method that the
// orchestrator dispatches (cyclerun_dispatch.go builds the PhaseRequest from
// cr.cs.ActiveWorktree and hands it to the registered PhaseRunner). Every
// predicate here therefore constructs the phase through its real constructor,
// calls Run, and asserts on the BridgeRequest a recording bridge captured —
// the production value on the production path. None of them greps source
// (the cycle-85 degenerate-predicate ban); the sole file assertion is 005,
// which checks an operator-mandated DOC deliverable, declared with a waiver.
//
// Discrimination is proven, not assumed: against the cycle base SHA 9b129565
// (the pre-fix retroWorktree) predicates 001 and 003 FAIL, while 002 and 004 —
// the anti-over-widening axes — stay GREEN. See test-report.md's RED Run Output.
package cycle1283

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/retro"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// recordingBridge stands in for the tmux bridge and captures the BridgeRequest.
// It writes the artifact the phase expects so Run completes its normal path
// rather than short-circuiting on a bridge error — the dispatched worktree must
// be correct on the SUCCESS path, which is the path a real retro takes.
type recordingBridge struct {
	gotReq  core.BridgeRequest
	called  bool
	content string
}

func (b *recordingBridge) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.gotReq = req
	b.called = true
	if req.ArtifactPath != "" && b.content != "" {
		_ = os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755)
		_ = os.WriteFile(req.ArtifactPath, []byte(b.content), 0o644)
	}
	return core.BridgeResponse{Stdout: b.content}, nil
}

func (b *recordingBridge) Probe(ctx context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

// retroPrompts supplies the retrospective agent body in-memory so the predicate
// never depends on the on-disk prompt tree (which differs between the main tree
// and a lane worktree).
func retroPrompts() *prompts.Loader {
	return prompts.NewFromFS(fstest.MapFS{
		"agents/evolve-retrospective.md": &fstest.MapFile{
			Data: []byte("---\nname: evolve-retrospective\n---\nbody"),
		},
	})
}

// dispatchRetro drives the REAL production seam: the exported Run method the
// orchestrator invokes for the retro phase, on the FAIL path (a PASS verdict
// short-circuits to SKIPPED with no bridge launch at all). It returns the
// worktree the bridge was actually handed.
func dispatchRetro(t *testing.T, projectRoot, workspace, worktree, fleet string) (string, *recordingBridge) {
	t.Helper()
	fb := &recordingBridge{content: "# Retrospective\n\n## Root Cause\nx\n"}
	phase := retro.New(retro.Config{Bridge: fb, Prompts: retroPrompts()})
	req := core.PhaseRequest{
		Cycle:       1283,
		ProjectRoot: projectRoot,
		Workspace:   workspace,
		Worktree:    worktree,
		Env:         map[string]string{ipcenv.FleetKey: fleet},
		Context:     map[string]string{"previous_verdict": core.VerdictFAIL},
	}
	if _, err := phase.Run(context.Background(), req); err != nil {
		t.Fatalf("retro.Run returned a hard error (%v) — retro is the failure-handler; a hard error aborts the whole batch", err)
	}
	if !fb.called {
		t.Fatalf("retro never reached the bridge — the dispatched worktree this predicate asserts on was never produced")
	}
	return fb.gotReq.Worktree, fb
}

// prunedLanePath returns a path guaranteed NOT to exist — the exact shape a
// torn-down fleet lane leaves behind in cs.ActiveWorktree.
func prunedLanePath(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "worktrees", "cycle-42824668-9999")
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("fixture path %q must not exist (stat err=%v)", p, err)
	}
	return p
}

// TestC1283_001_StaleFleetWorktreeDispatchesLiveDirectory is the crux (AC1).
// Fleet mode + a pruned lane's stale path: the worktree handed to the bridge
// must satisfy the driver's own predicate — an EXISTING DIRECTORY — and must
// live under the workspace retro already owns. Asserting via os.Stat rather
// than "!= the input" is what makes this the guard's predicate: any other
// fabricated string strands the lane just as thoroughly.
func TestC1283_001_StaleFleetWorktreeDispatchesLiveDirectory(t *testing.T) {
	projectRoot, workspace := t.TempDir(), t.TempDir()
	stale := prunedLanePath(t)

	got, _ := dispatchRetro(t, projectRoot, workspace, stale, "1")

	if got == stale {
		t.Fatalf("retro dispatched the pruned lane's stale worktree %q verbatim — the fleet driver refuses it at isDir() (ExitBadFlags, stderr only) and the lane loses its retrospective entirely (cycle-1255 CRITICAL, reproduced exit 1)", got)
	}
	if got == "" {
		t.Fatalf("retro dispatched an EMPTY worktree despite owning workspace %q — under fleet the driver then refuses with errWorktreeRequired: the same lost retrospective by a different exit code", workspace)
	}
	fi, err := os.Stat(got)
	if err != nil || !fi.IsDir() {
		t.Fatalf("dispatched worktree %q is not an existing directory (%v) — a fabricated path is the exact shape this must never produce", got, err)
	}
	if got == projectRoot || strings.HasPrefix(got, projectRoot+string(filepath.Separator)) {
		t.Errorf("dispatched worktree %q resolves inside the shared main tree %q — worktree is the write-authority predicate (refuted by PR #400)", got, projectRoot)
	}
	if cwd, cerr := os.Getwd(); cerr == nil && got == cwd {
		t.Errorf("dispatched worktree is the dispatching process cwd (%q) — the exact leak the fleet guard exists to close", got)
	}
	if !strings.HasPrefix(got, workspace+string(filepath.Separator)) {
		t.Errorf("dispatched worktree %q is not under the workspace retro owns (%q) — a disposable cwd must live where the lane already holds write authority", got, workspace)
	}
}

// TestC1283_002_LiveFleetWorktreePassesThroughVerbatim is the regression axis
// (AC2) and the reason the fix cannot be "always substitute". A fallback that
// fired on a LIVE worktree would strand every NORMAL fleet retro in an empty
// scratch dir with no repo — trading a rare lost retrospective for a universal
// one. This predicate is GREEN both before and after the fix by design: it
// exists to fail an over-broad implementation.
func TestC1283_002_LiveFleetWorktreePassesThroughVerbatim(t *testing.T) {
	live := t.TempDir() // a real, provisioned lane worktree
	workspace := t.TempDir()

	got, _ := dispatchRetro(t, t.TempDir(), workspace, live, "1")

	if got != live {
		t.Fatalf("retro replaced the LIVE lane worktree %q with %q — a fallback that fires on an existing worktree strands every normal fleet retro in a repo-less scratch dir", live, got)
	}
	if _, err := os.Stat(filepath.Join(workspace, "retro-scratch-cwd")); err == nil {
		t.Errorf("a scratch cwd was minted under the workspace even though a real worktree was provisioned — wasted state and a signal the fallback fired unconditionally")
	}
}

// TestC1283_003_FleetNeverDispatchesANonExistentPath is the edge axis (AC3).
// With no owned workspace there is nowhere safe to mint, so the honest degenerate
// answer is "" — let the bridge decide exactly as it does today. What retro may
// NEVER do is hand the driver a non-empty path that fails isDir(): that is the
// whole defect class, independent of which fallback happens to be available.
func TestC1283_003_FleetNeverDispatchesANonExistentPath(t *testing.T) {
	stale := prunedLanePath(t)

	got, _ := dispatchRetro(t, t.TempDir(), "" /* no owned workspace */, stale, "1")

	if got == "" {
		return // nothing to mint, nothing fabricated — the honest degenerate case
	}
	if fi, err := os.Stat(got); err != nil || !fi.IsDir() {
		t.Fatalf("retro dispatched the non-existent path %q with no workspace to mint under (input was the stale %q) — every non-empty worktree retro emits must clear the driver's isDir() guard", got, stale)
	}
}

// TestC1283_004_NonFleetStalePathPassesThroughVerbatim is the negative axis on
// the MODE dimension (AC4). Outside fleet the driver keeps its process-cwd
// fallback and reports a bad dir loudly to the operator; retro must not silently
// rewrite the operator's designated worktree. Widening the condition without
// keeping it fleet-gated changes single-driver semantics — this predicate fails
// that mistake and is GREEN before and after the correct fix.
func TestC1283_004_NonFleetStalePathPassesThroughVerbatim(t *testing.T) {
	stale := prunedLanePath(t)

	got, _ := dispatchRetro(t, t.TempDir(), t.TempDir(), stale, "0")

	if got != stale {
		t.Fatalf("non-fleet dispatch rewrote the operator's designated worktree %q to %q — the fallback exists for the fleet driver's fail-closed window ONLY", stale, got)
	}
}

// TestC1283_005_LandingRecordedInBatchIntegrityReview enforces the operator
// directive recorded in operating-policy §3.8 and in the review doc itself:
// every fix lands with an issue/gap/solution entry. It is load-bearing HERE
// specifically because this defect's history IS a documentation failure — the
// 1255 CRITICAL was narrowed by a rename and then sealed "verified closed" while
// the stale-path half was still live. Without a written landing record the next
// reviewer inherits the same false closure.
//
// acs-predicate: config-check — the deliverable under assertion IS a document,
// so its content is the artifact, not a proxy for behaviour. The behavioural
// half of this AC set is covered by 001–004 above.
func TestC1283_005_LandingRecordedInBatchIntegrityReview(t *testing.T) {
	doc := filepath.Join(acsassert.RepoRoot(t), "docs", "operations", "batch-integrity-review-2026-08-04.md")
	if !acsassert.FileExists(t, doc) {
		t.Fatalf("%s is missing — the operator directive (operating-policy §3.8) requires the issue/gap/solution record to live in this doc", doc)
	}
	// The landing section must name the cycle that closed it, so a future reader
	// can distinguish "prescribed" from "landed" — the exact distinction the
	// 1270 false-closure collapsed.
	if !acsassert.FileContains(t, doc, "cycle-1283") {
		t.Errorf("the review doc does not record the cycle-1283 landing — F1 stays indistinguishable from the 1270/1272 'verified closed' claims it was filed to correct")
	}
	// The doc's established format is inline bold markers (see F1–F6), not
	// headings — the landing entry must match the file's own convention.
	for _, needle := range []string{"**Issue.**", "**Gap.**", "**Solution.**"} {
		if !acsassert.FileContains(t, doc, needle) {
			t.Errorf("the review doc is missing the %q marker required by the issue/gap/solution format (operating-policy §3.8)", needle)
		}
	}
}
