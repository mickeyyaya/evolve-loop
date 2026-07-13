//go:build integration

// trivialrebase_carryforward_test.go — cycle-786 TDD contract for merge
// ladder RUNG 0 (inbox merge-rung0-trivial-rebase-carryforward; research
// knowledge-base/research/merge-concurrency-2026: verdicts follow the CHANGE
// via git patch-id, gates follow the TREE).
//
// Contract encoded here, at the existing verifyAuditBinding seam:
//
//   - A "composition-verdict" ledger entry (kind=composition-verdict,
//     method=trivial-rebase) written after a conflict-free rebase whose
//     git patch-id is UNCHANGED vs the audited diff lets verifyAuditBinding
//     accept the audit+composition chain instead of hard-failing
//     CodeAuditBindingHeadMoved — no fresh Auditor dispatch.
//   - Ship must kernel-recompute the composed tree's patch-id live (git
//     diff HEAD | git patch-id --stable) and reject an entry whose recorded
//     patch_id does not match — drift falls through to full re-audit.
//   - The entry's gate_results (full native gate set on the composed tree,
//     repo-wide per ADR-0069, run via ciparity runners) must all be "pass";
//     any other value keeps the fast path closed.
//
// Entry schema (the Builder contract — fields the fast path reads):
//
//	{
//	  "ts": ..., "cycle": N, "kind": "composition-verdict",
//	  "method": "trivial-rebase",
//	  "lane_audit_ref":      <artifact_sha256 of the bound auditor entry>,
//	  "patch_id":            <git patch-id --stable of the audited lane diff>,
//	  "audited_base":        <git_head the audit bound>,
//	  "new_base":            <the moved main head the lane was rebased onto>,
//	  "git_head":            <composed HEAD ship verifies against>,
//	  "tree_state_sha":      <sha256(git diff HEAD) of the composed tree>,
//	  "audited_diff_path":   <persisted audited diff (kernel-recompute input)>,
//	  "composed_diff_path":  <persisted composed diff (kernel-recompute input)>,
//	  "gate_results":        {"compile":"pass","test":"pass","acs":"pass","apicover":"pass"}
//	}
//
// RED status at authoring (cycle 786): TestTrivialRebase_CarriesAuditForward
// FAILS (verifyAuditBinding returns CodeAuditBindingHeadMoved — the fast path
// does not exist). The two rejection tests are pre-existing GREEN guards: they
// pass today because HEAD-moved rejects everything, and once the fast path
// lands they pin that it never over-accepts (drifted patch-id, failed gates).
package ship

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// trPassingGates is the full native gate set the composed tree must pass
// (compile, go test, ACS suite, apicover — repo-wide per ADR-0069).
func trPassingGates() map[string]string {
	return map[string]string{"compile": "pass", "test": "pass", "acs": "pass", "apicover": "pass"}
}

// trPatchID pipes `git diff HEAD` through `git patch-id --stable` and returns
// the patch-id — the offset-insensitive identity of the lane's change.
func trPatchID(t *testing.T, repo string) string {
	t.Helper()
	diff := runGitOut(t, repo, "diff", "HEAD")
	cmd := exec.Command("git", "patch-id", "--stable")
	cmd.Dir = repo
	cmd.Env = filteredEnv()
	cmd.Stdin = strings.NewReader(diff)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git patch-id --stable: %v", err)
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		t.Fatalf("git patch-id produced no output (empty diff?)")
	}
	return fields[0]
}

// trAppendLedgerLine appends one raw JSONL entry to the repo's ledger,
// preserving the auditor entry seedAudit wrote before it.
func trAppendLedgerLine(t *testing.T, repo string, entry map[string]any) {
	t.Helper()
	line, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal composition entry: %v", err)
	}
	path := filepath.Join(repo, ".evolve", "ledger.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open ledger: %v", err)
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		t.Fatalf("append ledger: %v", err)
	}
}

// trScenario is the trivial-rebase fixture: a lane change audited at
// auditedHead, then main moved to newHead via an unrelated landing while the
// lane's (uncommitted) change — and therefore its patch-id — stayed intact.
type trScenario struct {
	repo        string
	auditedHead string
	newHead     string
	patchID     string // patch-id of the AUDITED lane diff
	auditRef    string // artifact_sha256 of the bound auditor entry
}

func trSetup(t *testing.T) trScenario {
	t.Helper()
	repo := makeRepo(t)
	// The lane's change: uncommitted edit, exactly what the audit binds
	// (worktree flow: git_head = base, tree_state_sha = the uncommitted diff).
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nlane change\n")
	seedAudit(t, repo, "PASS")
	auditedHead := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))
	patchID := trPatchID(t, repo)
	mustWrite(t, filepath.Join(repo, ".evolve", "runs", "cycle-1", "audited.diff"),
		runGitOut(t, repo, "diff", "HEAD"))

	// Main moves: another lane lands an UNRELATED file, so the rebase is
	// conflict-free and the lane diff's patch-id is unchanged.
	mustWrite(t, filepath.Join(repo, "other-lane.txt"), "another lane landed\n")
	runGit(t, repo, "add", "other-lane.txt")
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "other lane lands on main")
	newHead := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))

	auditRef := mustHashFile(t, filepath.Join(repo, ".evolve", "runs", "cycle-1", "audit-report.md"))
	return trScenario{repo: repo, auditedHead: auditedHead, newHead: newHead, patchID: patchID, auditRef: auditRef}
}

// entry builds a composition-verdict ledger entry for the scenario's CURRENT
// composed tree, claiming the given patch_id and gate results.
func (s trScenario) entry(t *testing.T, patchID string, gates map[string]string) map[string]any {
	t.Helper()
	composedDiff := filepath.Join(s.repo, ".evolve", "runs", "cycle-1", "composed.diff")
	mustWrite(t, composedDiff, runGitOut(t, s.repo, "diff", "HEAD"))
	return map[string]any{
		"ts":                 "2026-04-27T00:05:00Z",
		"cycle":              1,
		"kind":               "composition-verdict",
		"method":             "trivial-rebase",
		"lane_audit_ref":     s.auditRef,
		"patch_id":           patchID,
		"audited_base":       s.auditedHead,
		"new_base":           s.newHead,
		"git_head":           s.newHead,
		"tree_state_sha":     treeStateSHA(t, s.repo),
		"audited_diff_path":  filepath.Join(s.repo, ".evolve", "runs", "cycle-1", "audited.diff"),
		"composed_diff_path": composedDiff,
		"gate_results":       gates,
	}
}

// TestTrivialRebase_CarriesAuditForward: clean rebase, unchanged patch-id,
// all composed-tree gates pass → verifyAuditBinding accepts the
// audit+composition chain (nil error), so ship proceeds with NO fresh
// auditor dispatch. RED until the fast path exists (today:
// CodeAuditBindingHeadMoved).
func TestTrivialRebase_CarriesAuditForward(t *testing.T) {
	s := trSetup(t)
	trAppendLedgerLine(t, s.repo, s.entry(t, s.patchID, trPassingGates()))

	opts := auditOpts(t, s.repo)
	res := &RunResult{}
	if err := verifyAuditBinding(context.Background(), opts, res); err != nil { //nolint:staticcheck
		t.Fatalf("trivial-rebase carry-forward: want verifyAuditBinding to accept the audit+composition chain (nil), got: %v", err)
	}
}

// TestTrivialRebase_PatchIdDriftFallsBackToReaudit: the lane's diff changed
// semantically after the audit (patch-id drift), but the composition entry
// still claims the audited patch_id. Ship's live kernel recompute must catch
// the mismatch and REJECT — falling back to the existing full re-audit path.
//
// Pre-existing GREEN guard: today every moved-HEAD ship is rejected
// (CodeAuditBindingHeadMoved), so this passes vacuously; once the fast path
// lands it is the test that keeps drift from carrying a verdict forward.
func TestTrivialRebase_PatchIdDriftFallsBackToReaudit(t *testing.T) {
	s := trSetup(t)
	// Semantic drift after the audit: the composed diff no longer matches
	// the audited patch-id the entry claims.
	mustWrite(t, filepath.Join(s.repo, "fixture.txt"), "fixture line 1\nlane change\npost-audit drift\n")
	trAppendLedgerLine(t, s.repo, s.entry(t, s.patchID, trPassingGates()))

	opts := auditOpts(t, s.repo)
	if err := verifyAuditBinding(context.Background(), opts, &RunResult{}); err == nil { //nolint:staticcheck
		t.Fatalf("patch-id drift: verifyAuditBinding accepted a composition entry whose recorded patch_id does not match the composed tree — drift must fall back to full re-audit")
	}
}

// TestTrivialRebase_FailedComposedGatesRejected: patch-id genuinely unchanged,
// but the composed-tree native gate set did not all pass (gates follow the
// TREE — they must re-run and be green even when the verdict carries forward).
// A composition entry recording any non-"pass" gate keeps the fast path closed.
//
// Pre-existing GREEN guard (same vacuous-today reasoning as the drift test).
func TestTrivialRebase_FailedComposedGatesRejected(t *testing.T) {
	s := trSetup(t)
	gates := trPassingGates()
	gates["test"] = "fail"
	trAppendLedgerLine(t, s.repo, s.entry(t, s.patchID, gates))

	opts := auditOpts(t, s.repo)
	if err := verifyAuditBinding(context.Background(), opts, &RunResult{}); err == nil { //nolint:staticcheck
		t.Fatalf("failed composed-tree gates: verifyAuditBinding accepted a composition entry with gate_results.test=fail — gates bind to the tree and must be green")
	}
}
