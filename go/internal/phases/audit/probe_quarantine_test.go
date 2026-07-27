package audit

// probe_quarantine_test.go — the observer must not perturb the observed
// system. The adversarial auditor legitimately writes probe tests to try to
// refute the build (cycles 1115/1117: TestZZAudit_QuoteSpillSuppressed,
// TestZZAuditProbe_BulletBannerSuppressed) — but it wrote them INTO the
// package under test, so the EGPS run inherited the probes' engineered
// failures as if the BUILDER had regressed a sibling: auditor-graded PASS
// alongside a red gate, a false cycle FAIL. The probes were then discarded
// with the worktree, leaving the finding unrecoverable.
//
// quarantineAuditProbes runs after the auditor agent and before acssuite.Run:
// untracked *_test.go files created SINCE the audit agent was dispatched
// (mtime after the audit-prompt artifact — builder files predate it) are
// PRESERVED into the run dir and REMOVED from the tree, loudly. The safe
// probe idiom remains `go test -overlay` (cycle-1106's PoCA/PoCB), which
// never touches the tree at all.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// probeRepo builds a worktree-shaped git repo: a committed builder test, an
// UNTRACKED builder test older than the cutoff, and an untracked auditor
// probe newer than the cutoff.
func probeRepo(t *testing.T) (worktree, workspace string, cutoff time.Time) {
	t.Helper()
	worktree = t.TempDir()
	workspace = t.TempDir()
	gitInAudit(t, worktree, "init", "-q")
	gitInAudit(t, worktree, "commit", "-q", "--allow-empty", "-m", "base")

	pkg := filepath.Join(worktree, "go", "internal", "bridge")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-1 * time.Hour)
	// Builder's untracked deliverable test — predates the audit dispatch.
	builderTest := filepath.Join(pkg, "feature_c9999_test.go")
	if err := os.WriteFile(builderTest, []byte("package bridge\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(builderTest, old, old); err != nil {
		t.Fatal(err)
	}

	cutoff = time.Now().Add(-1 * time.Minute) // audit-prompt.txt mtime
	// Auditor probe — created after dispatch.
	probe := filepath.Join(pkg, "zz_audit_probe_test.go")
	if err := os.WriteFile(probe, []byte("package bridge\nimport \"testing\"\nfunc TestZZAuditProbe_X(t *testing.T){t.Fatal(\"engineered\")}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return worktree, workspace, cutoff
}

// TestQuarantineAuditProbes_PreservesAndExcludesAuditAuthoredTests pins all
// three obligations at once: the probe leaves the tree, lands in the run dir,
// and the action is loudly logged.
func TestQuarantineAuditProbes_PreservesAndExcludesAuditAuthoredTests(t *testing.T) {
	worktree, workspace, cutoff := probeRepo(t)
	var log bytes.Buffer

	moved, err := quarantineAuditProbes(worktree, workspace, cutoff, &log)
	if err != nil {
		t.Fatalf("quarantineAuditProbes: %v", err)
	}
	if len(moved) != 1 || !strings.Contains(moved[0], "zz_audit_probe_test.go") {
		t.Fatalf("moved = %v, want exactly the auditor probe", moved)
	}
	if _, err := os.Stat(filepath.Join(worktree, "go", "internal", "bridge", "zz_audit_probe_test.go")); !os.IsNotExist(err) {
		t.Error("probe still present in the worktree — the ACS run will inherit its engineered failure as a builder regression (the 1115/1117 false-FAIL shape)")
	}
	preserved := filepath.Join(workspace, "audit-probes", "go", "internal", "bridge", "zz_audit_probe_test.go")
	if _, err := os.Stat(preserved); err != nil {
		t.Errorf("probe not preserved at %s — discarding it makes the auditor's finding unrecoverable (1115/1117: only a truncated FAIL line survived)", preserved)
	}
	if !strings.Contains(log.String(), "zz_audit_probe_test.go") {
		t.Error("quarantine was silent — excluding a file from the gate without a loud log is itself a gate-weakening path")
	}
}

// TestQuarantineAuditProbes_LeavesBuilderWorkAlone (negative): untracked
// builder tests that predate the audit dispatch are the cycle's DELIVERABLE —
// quarantining them would hide the diff from its own predicates.
func TestQuarantineAuditProbes_LeavesBuilderWorkAlone(t *testing.T) {
	worktree, workspace, cutoff := probeRepo(t)
	if _, err := quarantineAuditProbes(worktree, workspace, cutoff, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(worktree, "go", "internal", "bridge", "feature_c9999_test.go")); err != nil {
		t.Errorf("builder's untracked deliverable test was quarantined — it predates the audit dispatch and is part of the diff: %v", err)
	}
}

// TestQuarantineAuditProbes_NoProbesIsANoOp (negative): the common case must
// touch nothing and log nothing.
func TestQuarantineAuditProbes_NoProbesIsANoOp(t *testing.T) {
	worktree, workspace, _ := probeRepo(t)
	var log bytes.Buffer
	moved, err := quarantineAuditProbes(worktree, workspace, time.Now().Add(time.Hour), &log)
	if err != nil {
		t.Fatal(err)
	}
	if len(moved) != 0 || log.Len() != 0 {
		t.Errorf("future cutoff must quarantine nothing (moved=%v log=%q)", moved, log.String())
	}
}

// TestQuarantineAuditProbes_CommittedFilesNeverTouched (adversarial): a file
// in HEAD — genuinely tracked — must never be quarantined even when modified
// post-dispatch: a tracked modification is visible in the diff and an auditor
// edit of one is a different violation with a different guard.
func TestQuarantineAuditProbes_CommittedFilesNeverTouched(t *testing.T) {
	worktree, workspace, cutoff := probeRepo(t)
	tracked := filepath.Join(worktree, "go", "internal", "bridge", "tracked_test.go")
	if err := os.WriteFile(tracked, []byte("package bridge\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInAudit(t, worktree, "add", "go/internal/bridge/tracked_test.go")
	gitInAudit(t, worktree, "commit", "-q", "-m", "builder ships the test")
	if err := os.WriteFile(tracked, []byte("package bridge\n// modified\n"), 0o644); err != nil {
		t.Fatal(err) // post-dispatch modification of a COMMITTED file
	}
	if _, err := quarantineAuditProbes(worktree, workspace, cutoff, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tracked); err != nil {
		t.Errorf("committed test file was removed by quarantine: %v", err)
	}
}

// TestQuarantineAuditProbes_StagedLeftoverProbeIsQuarantined (review M9): a
// preserved continuation worktree's pre-loop `git add -A` stages a prior
// attempt's probe, so it arrives as `A `-status, not `??`. Staged-NEW is
// still new — it must quarantine like an untracked probe.
func TestQuarantineAuditProbes_StagedLeftoverProbeIsQuarantined(t *testing.T) {
	worktree, workspace, cutoff := probeRepo(t)
	gitInAudit(t, worktree, "add", "go/internal/bridge/zz_audit_probe_test.go")
	moved, err := quarantineAuditProbes(worktree, workspace, cutoff, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if len(moved) != 1 || !strings.Contains(moved[0], "zz_audit_probe_test.go") {
		t.Errorf("staged-new probe not quarantined (moved=%v) — pre-loop git add -A would launder a leftover probe past a ??-only filter", moved)
	}
}

// TestClassify_QuarantinesProbesEvenWhenVerdictPreStaged is the WIRING proof
// at the phase level: the quarantine fires in Classify BEFORE the
// verdict-exists gate, so an auditor that pre-writes acs-verdict.json (which
// skips genVerdict entirely — the persona instructs exactly that) cannot skip
// the quarantine with it.
func TestClassify_QuarantinesProbesEvenWhenVerdictPreStaged(t *testing.T) {
	worktree, workspace, _ := probeRepo(t)
	workspace = filepath.Join(workspace, "runs", "cycle-9999")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	promptPath := filepath.Join(workspace, auditPromptArtifact)
	if err := os.WriteFile(promptPath, []byte("prompt"), 0o644); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-1 * time.Minute)
	if err := os.Chtimes(promptPath, past, past); err != nil {
		t.Fatal(err)
	}
	// Verdict PRE-STAGED: genVerdict must not run — and quarantine must anyway.
	if err := os.WriteFile(filepath.Join(workspace, "acs-verdict.json"),
		[]byte(`{"schema_version":"1.0","cycle":9999,"results":[],"green_count":1,"red_count":0,"verdict":"PASS","ship_eligible":true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	h := hooks{genVerdict: func(core.PhaseRequest) error {
		t.Fatal("genVerdict must not run when acs-verdict.json is pre-staged")
		return nil
	}}
	h.Classify("## Verdict\n**PASS**", core.PhaseRequest{
		Cycle: 9999, Worktree: worktree, ProjectRoot: worktree, Workspace: workspace,
	}, core.BridgeResponse{})

	if _, err := os.Stat(filepath.Join(worktree, "go", "internal", "bridge", "zz_audit_probe_test.go")); !os.IsNotExist(err) {
		t.Error("pre-staged verdict bypassed the probe quarantine — the persona-sanctioned write-verdict-yourself path re-opens the 1115/1117 poisoned-gate hole")
	}
	if _, err := os.Stat(filepath.Join(workspace, auditProbesDir, "go", "internal", "bridge", "zz_audit_probe_test.go")); err != nil {
		t.Errorf("probe not preserved under the run dir: %v", err)
	}
}

// TestQuarantineProbesForRequest_AnchorSurvivesRedispatch (diff-review HIGH):
// bridge.Engine.Launch rewrites audit-prompt.txt on EVERY dispatch, so a
// re-dispatched audit gets a NEWER anchor than a dead first attempt's leftover
// probe — mtime-vs-latest-prompt would classify the probe as builder work and
// leave it to poison the gate (the 1115/1117 shape, reopened). The anchor must
// be the FIRST dispatch of the cycle, persisted across attempts.
func TestQuarantineProbesForRequest_AnchorSurvivesRedispatch(t *testing.T) {
	worktree, workspace, _ := probeRepo(t)
	workspace = filepath.Join(workspace, "runs", "cycle-9999")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	promptPath := filepath.Join(workspace, auditPromptArtifact)

	// Attempt 1 dispatched two hours ago; its quarantine ran (establishing the
	// anchor) BEFORE the auditor wrote anything, then the phase died. Remove
	// probeRepo's pre-made probe first so attempt 1 genuinely sees a clean tree.
	probe := filepath.Join(worktree, "go", "internal", "bridge", "zz_audit_probe_test.go")
	if err := os.Remove(probe); err != nil {
		t.Fatal(err)
	}
	first := time.Now().Add(-2 * time.Hour)
	if err := os.WriteFile(promptPath, []byte("attempt-1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(promptPath, first, first); err != nil {
		t.Fatal(err)
	}
	req := core.PhaseRequest{Cycle: 9999, Worktree: worktree, ProjectRoot: worktree, Workspace: workspace}
	if err := quarantineProbesForRequest(req); err != nil {
		t.Fatalf("attempt-1 quarantine: %v", err)
	}

	// Attempt 1's auditor writes the probe (between the two dispatches), then dies.
	if err := os.WriteFile(probe, []byte("package bridge\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mid := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(probe, mid, mid); err != nil {
		t.Fatal(err)
	}

	// Attempt 2: the prompt artifact is REWRITTEN with a fresh mtime.
	if err := os.WriteFile(promptPath, []byte("attempt-2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := quarantineProbesForRequest(req); err != nil {
		t.Fatalf("attempt-2 quarantine: %v", err)
	}

	if _, err := os.Stat(probe); !os.IsNotExist(err) {
		t.Error("attempt-1's leftover probe survived attempt-2's quarantine — the anchor followed the REWRITTEN prompt " +
			"mtime instead of the first dispatch, reopening the 1115/1117 poisoned-gate path on every audit retry")
	}
}
