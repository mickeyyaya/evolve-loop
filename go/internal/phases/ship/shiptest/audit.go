package shiptest

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/acsverdict"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/statemap"
	"github.com/mickeyyaya/evolve-loop/go/internal/treefence"
)

type auditSeed struct {
	repo, verdict string
	overrides     map[string]string
	state         map[string]any
	cycle         int
	runID         string
	reportPath    string
}

func SeedAudit(tb testing.TB, repo, verdict string, optOverrides ...map[string]string) {
	tb.Helper()
	s := auditSeed{repo: repo, verdict: verdict, overrides: map[string]string{}}
	if len(optOverrides) > 0 {
		s.overrides = optOverrides[0]
	}
	s.readCycleState(tb)
	boundTree := s.writeSealedReport(tb)
	s.state["cycle_id"], s.state["run_id"], s.state["audit_dispatches"] = s.cycle, s.runID, 1
	s.state["workspace_path"] = filepath.Dir(s.reportPath)
	check(tb, statemap.WriteStateMap(s.cycleStatePath(), s.state), "write %s", s.cycleStatePath())
	s.writeLedgerEntry(tb, boundTree)
}

func SeedAuditWithBoundTree(tb testing.TB, repo, verdict, boundTreeSHA string) {
	tb.Helper()
	SeedAudit(tb, repo, verdict, map[string]string{"bound_tree": boundTreeSHA})
}

func (s *auditSeed) cycleStatePath() string {
	return filepath.Join(s.repo, ".evolve", "cycle-state.json")
}

func (s *auditSeed) readCycleState(tb testing.TB) {
	tb.Helper()
	s.state, _ = statemap.ReadStateMap(s.cycleStatePath())
	if s.state == nil {
		s.state = map[string]any{}
	}
	s.cycle = 1
	if n, ok := s.state["cycle_id"].(float64); ok && n > 0 {
		s.cycle = int(n)
	}
	s.runID = "test-run"
	if id, ok := s.state["run_id"].(string); ok && id != "" {
		s.runID = id
	}
	if s.overrides["cycle"] != "" {
		s.cycle = s.intOverride(tb, "cycle")
	}
}

func (s *auditSeed) intOverride(tb testing.TB, key string) int {
	tb.Helper()
	n, err := strconv.Atoi(s.overrides[key])
	check(tb, err, "override %s", key)
	return n
}

func (s *auditSeed) writeSealedReport(tb testing.TB) string {
	tb.Helper()
	s.reportPath = filepath.Join(s.repo, ".evolve", "runs", fmt.Sprintf("cycle-%d", s.cycle), "audit-report.md")
	MustWrite(tb, s.reportPath, fmt.Sprintf("<!-- challenge-token: testtoken123 -->\n# Audit Report — Cycle %d\n\nVerdict: %s\n\nAll criteria met (test fixture).\n", s.cycle, s.verdict))
	testedRoot, _ := s.state["active_worktree"].(string)
	if testedRoot == "" {
		testedRoot = s.repo
	}
	RunGit(tb, testedRoot, "add", "-A")
	snap, err := treefence.TakeStaged(context.Background(), testedRoot, nil)
	check(tb, err, "snapshot the staged tree of %s", testedRoot)
	if s.overrides["bound_tree"] != "" {
		snap.Tree = s.overrides["bound_tree"]
	}
	SealTestPredicateEvidence(tb, acssuite.EvidenceIdentity{Cycle: s.cycle, RunID: s.runID, Round: 1, TreeSHA: snap.Tree}, s.reportPath)
	return snap.Tree
}

func (s *auditSeed) writeLedgerEntry(tb testing.TB, boundTree string) {
	tb.Helper()
	head := s.overrides["head"]
	if head == "" {
		head = strings.TrimSpace(RunGitOut(tb, s.repo, "rev-parse", "HEAD"))
	}
	tree := s.overrides["tree"]
	if tree == "" {
		tree = TreeStateSHA(tb, s.repo)
	}
	exitCode := 0
	if s.overrides["exit_code"] != "" {
		exitCode = s.intOverride(tb, "exit_code")
	}
	line, err := json.Marshal(map[string]any{
		"ts": "2026-04-27T00:00:00Z", "cycle": s.cycle, "run_id": s.runID, "worktree_tree_sha": boundTree,
		"role": "auditor", "kind": "agent_subprocess", "model": "sonnet", "exit_code": exitCode, "duration_s": "30",
		"artifact_path": s.reportPath, "artifact_sha256": MustHashFile(tb, s.reportPath), "challenge_token": "testtoken123",
		"git_head": head, "tree_state_sha": tree,
	})
	check(tb, err, "encode the ledger entry")
	MustWrite(tb, filepath.Join(s.repo, ".evolve", "ledger.jsonl"), string(line)+"\n")
}

func PredicateVerdictFixture(cycle, green, red, skip int) acssuite.Verdict {
	v := acssuite.Verdict{SchemaVersion: "1.0", Cycle: cycle, GreenCount: green, RedCount: red, SkipCount: skip, Verdict: "PASS", ShipEligible: red == 0,
		PredicateSuite: acssuite.PredicateSuite{Total: green + red + skip, ThisCycleCount: green + red + skip, SkippedCount: skip}}
	if red > 0 {
		v.Verdict = "FAIL"
	}
	for i := 0; i < green+red+skip; i++ {
		id := fmt.Sprintf("predicate-%d", i)
		r := acssuite.Result{ACID: id, Predicate: "go/acs/cycle/...:" + id, ResultStr: "green"}
		if i >= green && i < green+red {
			r.ResultStr = "red"
			r.ExitCode = 1
			v.RedIDs = append(v.RedIDs, id)
		}
		if i >= green+red {
			r.ResultStr = "skip"
			r.ExitCode = acssuite.SkipExitCode
			v.SkipIDs = append(v.SkipIDs, id)
		}
		v.Results = append(v.Results, r)
	}
	return v
}

func SealTestPredicateEvidence(tb testing.TB, id acssuite.EvidenceIdentity, reportPath string) {
	tb.Helper()
	raw, err := json.Marshal(PredicateVerdictFixture(id.Cycle, 1, 0, 0))
	check(tb, err, "encode the predicate verdict")
	MustWrite(tb, filepath.Join(filepath.Dir(reportPath), acsverdict.Filename), string(raw))
	check(tb, acssuite.SealEvidence(reportPath, raw, id), "seal the predicate evidence")
}
