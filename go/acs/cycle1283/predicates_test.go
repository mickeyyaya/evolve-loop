//go:build acs

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

func retroPrompts() *prompts.Loader {
	return prompts.NewFromFS(fstest.MapFS{
		"agents/evolve-retrospective.md": &fstest.MapFile{
			Data: []byte("---\nname: evolve-retrospective\n---\nbody"),
		},
	})
}

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

func prunedLanePath(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "worktrees", "cycle-42824668-9999")
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("fixture path %q must not exist (stat err=%v)", p, err)
	}
	return p
}

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

func TestC1283_002_LiveFleetWorktreePassesThroughVerbatim(t *testing.T) {
	live := t.TempDir()
	workspace := t.TempDir()

	got, _ := dispatchRetro(t, t.TempDir(), workspace, live, "1")

	if got != live {
		t.Fatalf("retro replaced the LIVE lane worktree %q with %q — a fallback that fires on an existing worktree strands every normal fleet retro in a repo-less scratch dir", live, got)
	}
	if _, err := os.Stat(filepath.Join(workspace, "retro-scratch-cwd")); err == nil {
		t.Errorf("a scratch cwd was minted under the workspace even though a real worktree was provisioned — wasted state and a signal the fallback fired unconditionally")
	}
}

func TestC1283_003_FleetNeverDispatchesANonExistentPath(t *testing.T) {
	stale := prunedLanePath(t)

	got, _ := dispatchRetro(t, t.TempDir(), "", stale, "1")

	if got == "" {
		return
	}
	if fi, err := os.Stat(got); err != nil || !fi.IsDir() {
		t.Fatalf("retro dispatched the non-existent path %q with no workspace to mint under (input was the stale %q) — every non-empty worktree retro emits must clear the driver's isDir() guard", got, stale)
	}
}

func TestC1283_004_NonFleetStalePathPassesThroughVerbatim(t *testing.T) {
	stale := prunedLanePath(t)

	got, _ := dispatchRetro(t, t.TempDir(), t.TempDir(), stale, "0")

	if got != stale {
		t.Fatalf("non-fleet dispatch rewrote the operator's designated worktree %q to %q — the fallback exists for the fleet driver's fail-closed window ONLY", stale, got)
	}
}

// acs-predicate: config-check — the deliverable under assertion IS a document,
func TestC1283_005_LandingRecordedInBatchIntegrityReview(t *testing.T) {
	doc := filepath.Join(acsassert.RepoRoot(t), "docs", "operations", "batch-integrity-review-2026-08-04.md")
	if !acsassert.FileExists(t, doc) {
		t.Fatalf("%s is missing — the operator directive (operating-policy §3.8) requires the issue/gap/solution record to live in this doc", doc)
	}
	if !acsassert.FileContains(t, doc, "cycle-1283") {
		t.Errorf("the review doc does not record the cycle-1283 landing — F1 stays indistinguishable from the 1270/1272 'verified closed' claims it was filed to correct")
	}
	for _, needle := range []string{"**Issue.**", "**Gap.**", "**Solution.**"} {
		if !acsassert.FileContains(t, doc, needle) {
			t.Errorf("the review doc is missing the %q marker required by the issue/gap/solution format (operating-policy §3.8)", needle)
		}
	}
}
