// RED contract for cycle-1255 task `retro-fleet-worktree-empty-fallback`.
//
// The window: when a fleet lane's worktree is gone (post-teardown) or was never
// provisioned (exhausted retries), retro dispatches with req.Worktree == "". The
// bridge's fleet guard (errWorktreeRequired, driver_tmux_repl.go:27) then refuses
// the launch and the knowledge-capture phase degrades to FAIL-not-adopted every
// time this window is hit.
//
// The fix retro must make: supply its OWN disposable cwd under the workspace it
// already owns (the applyScratchCwd precedent, scratch_cwd.go:22) so the launch
// carries a real, isolated, writable directory. Retro is read-mostly and
// Evaluate-archetype, so a scratch cwd is sufficient for it.
//
// Anti-goals these tests pin, because the naive fixes are the dangerous ones:
//   - NEVER point retro at the shared main tree (req.ProjectRoot) — PR #400 was
//     refuted on exactly that; worktree is the write-authority predicate.
//   - NEVER fall through to the dispatching process's cwd — that is the very
//     leak the fleet guard exists to close.
//   - NEVER widen the bridge guard itself (covered by the untouched
//     driver_tmux_repl_workdir_test.go fleet-refusal tests, which must stay green).
//   - NEVER clobber a real worktree when one WAS provisioned.
package retro

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// retroFailReq builds the minimal PhaseRequest that drives retro down the bridge
// path (previous verdict FAIL — a PASS short-circuits to SKIPPED with no launch).
func retroFailReq(projectRoot, workspace, worktree string, env map[string]string) core.PhaseRequest {
	return core.PhaseRequest{
		Cycle:       1255,
		ProjectRoot: projectRoot,
		Workspace:   workspace,
		Worktree:    worktree,
		Env:         env,
		Context:     map[string]string{"previous_verdict": core.VerdictFAIL},
	}
}

// TestRetro_EmptyWorktree_FallsBackToScratchUnderWorkspace — AC1 (the crux).
// Fleet mode, empty worktree: retro must still dispatch, and the BridgeRequest it
// hands the bridge must carry a NON-EMPTY worktree that is a real directory
// living under the workspace retro already owns. Asserting on the captured
// BridgeRequest (not on a source string) is what makes this behavioural: it is
// the exact value the fleet guard reads.
func TestRetro_EmptyWorktree_FallsBackToScratchUnderWorkspace(t *testing.T) {
	ws := t.TempDir()
	projectRoot := t.TempDir()
	fb := &fakeBridge{writeArtifact: "# Retrospective\n\n## Root Cause\nx\n"}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})

	if _, err := phase.Run(context.Background(), retroFailReq(projectRoot, ws, "", map[string]string{"EVOLVE_FLEET": "1"})); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := fb.gotReq.Worktree
	if got == "" {
		t.Fatalf("BridgeRequest.Worktree is empty — the fleet guard refuses this launch and retro is lost for the whole lane")
	}
	if !strings.HasPrefix(got, ws+string(os.PathSeparator)) {
		t.Fatalf("BridgeRequest.Worktree = %q, want a directory under the owned workspace %q", got, ws)
	}
	fi, err := os.Stat(got)
	if err != nil || !fi.IsDir() {
		t.Fatalf("fallback cwd %q is not an existing directory (err=%v) — the bridge rejects a non-existent working dir", got, err)
	}
}

// TestRetro_EmptyWorktree_NeverMainTreeOrProcessCwd — AC2, the NEGATIVE axis.
// The two shapes that would satisfy AC1's "non-empty" letter while reintroducing
// the exact defect the guard exists to prevent: pointing at the shared main tree
// (the refuted PR #400 pattern) or at the dispatching process's cwd.
func TestRetro_EmptyWorktree_NeverMainTreeOrProcessCwd(t *testing.T) {
	ws := t.TempDir()
	projectRoot := t.TempDir()
	fb := &fakeBridge{writeArtifact: "# Retrospective\n\n## Root Cause\nx\n"}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})

	if _, err := phase.Run(context.Background(), retroFailReq(projectRoot, ws, "", map[string]string{"EVOLVE_FLEET": "1"})); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := fb.gotReq.Worktree
	if got == "" {
		t.Fatalf("BridgeRequest.Worktree is empty — the launch is refused outright, so the safe-destination contract below is untested")
	}
	if got == projectRoot || strings.HasPrefix(got, projectRoot+string(os.PathSeparator)) {
		t.Fatalf("BridgeRequest.Worktree = %q resolves inside the shared main tree %q — refuted by PR #400: worktree is the write-authority predicate", got, projectRoot)
	}
	if cwd, err := os.Getwd(); err == nil && got == cwd {
		t.Fatalf("BridgeRequest.Worktree = %q is the dispatching process cwd — the exact leak the fleet guard closes", got)
	}
}

// TestRetro_RealWorktree_PassedThroughUnchanged — AC3. A lane that DID provision
// its worktree must be dispatched against that worktree verbatim. A fallback that
// fires unconditionally would silently strand every normal retro in a scratch dir
// with no repo — the blind-widen regression this task's own notes name.
func TestRetro_RealWorktree_PassedThroughUnchanged(t *testing.T) {
	ws := t.TempDir()
	worktree := t.TempDir()
	fb := &fakeBridge{writeArtifact: "# Retrospective\n\n## Root Cause\nx\n"}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})

	if _, err := phase.Run(context.Background(), retroFailReq(t.TempDir(), ws, worktree, map[string]string{"EVOLVE_FLEET": "1"})); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if fb.gotReq.Worktree != worktree {
		t.Fatalf("BridgeRequest.Worktree = %q, want the provisioned worktree %q left untouched", fb.gotReq.Worktree, worktree)
	}
	if _, err := os.Stat(filepath.Join(ws, "bridge-scratch-cwd")); err == nil {
		t.Errorf("a scratch cwd was minted under the workspace even though a real worktree was provisioned")
	}
}

// TestRetro_EmptyWorktreeAndWorkspace_NoFabricatedPath — AC4, the EDGE axis.
// With no owned workspace there is nowhere safe to mint a scratch dir. Retro must
// degrade to today's behaviour (leave Worktree empty and let the bridge decide)
// rather than fabricate a path — and it must not panic or return a hard error,
// because a failure in the failure-handler must never abort the batch (GAP 9).
func TestRetro_EmptyWorktreeAndWorkspace_NoFabricatedPath(t *testing.T) {
	fb := &fakeBridge{}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})

	if _, err := phase.Run(context.Background(), retroFailReq(t.TempDir(), "", "", map[string]string{"EVOLVE_FLEET": "1"})); err != nil {
		t.Fatalf("Run returned a hard error with no workspace (%v) — retro must never abort the batch", err)
	}
	if got := fb.gotReq.Worktree; got != "" {
		t.Fatalf("BridgeRequest.Worktree = %q with no owned workspace — a fabricated path outside any owned dir is exactly the leak surface", got)
	}
}
