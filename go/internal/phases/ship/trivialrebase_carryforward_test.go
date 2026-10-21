//go:build integration

package ship

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func trPassingGates() map[string]string {
	return map[string]string{"compile": "pass", "test": "pass", "acs": "pass", "apicover": "pass"}
}

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

type trScenario struct {
	repo        string
	auditedHead string
	newHead     string
	patchID     string
	auditRef    string
}

func trSetup(t *testing.T) trScenario {
	t.Helper()
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nlane change\n")
	seedAudit(t, repo, "PASS")
	auditedHead := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))
	patchID := trPatchID(t, repo)
	mustWrite(t, filepath.Join(repo, ".evolve", "runs", "cycle-1", "audited.diff"),
		runGitOut(t, repo, "diff", "HEAD"))

	runGit(t, repo, "reset", "--", "fixture.txt")

	mustWrite(t, filepath.Join(repo, "other-lane.txt"), "another lane landed\n")
	runGit(t, repo, "add", "other-lane.txt")
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "other lane lands on main")
	newHead := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))

	auditRef := mustHashFile(t, filepath.Join(repo, ".evolve", "runs", "cycle-1", "audit-report.md"))
	return trScenario{repo: repo, auditedHead: auditedHead, newHead: newHead, patchID: patchID, auditRef: auditRef}
}

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

func TestVerifyAuditBinding_ARetiredTrivialRebaseRecordNeverCarriesAMovedHead(t *testing.T) {
	t.Parallel()
	s := trSetup(t)
	mustWrite(t, filepath.Join(s.repo, "fixture.txt"), "fixture line 1\nlane change\n")
	trAppendLedgerLine(t, s.repo, s.entry(t, s.patchID, trPassingGates()))

	err := verifyAuditBinding(context.Background(), auditOpts(t, s.repo), &RunResult{})

	wantShipErr(t, err, core.CodeAuditBindingHeadMoved, core.ShipClassPrecondition, "git HEAD has moved since audit")
}
