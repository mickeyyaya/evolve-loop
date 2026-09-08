//go:build integration

package ship

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/treefence"
)

func TestVerifyAuditBinding_PredicateEvidenceCannotChangeAfterAudit(t *testing.T) {
	for _, mutation := range []string{"delete", "corrupt", "empty", "other-cycle", "other-execution", "other-round", "other-run", "report", "source", "untracked-source", "host-fail", "landed-tree"} {
		t.Run(mutation, func(t *testing.T) {
			repo := makeRepo(t)
			ws := filepath.Join(repo, ".evolve", "runs", "cycle-1")
			path := filepath.Join(ws, acssuite.VerdictFilename)
			reportPath := filepath.Join(ws, "audit-report.md")
			v, err := acssuite.Run(acssuite.Options{Root: repo, Cycle: 1, GoExec: func(_ context.Context, _, pattern string, _ []string) (string, error) {
				if pattern != "./acs/cycle1" {
					return "", nil
				}
				return `{"Action":"pass","Package":"fixture/acs/cycle1","Test":"TestReal"}`, nil
			}})
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(v)
			mustWrite(t, path, string(raw))
			mustWrite(t, reportPath, "## Verdict\n**PASS**\n")
			snap, err := treefence.Take(context.Background(), repo)
			if err != nil {
				t.Fatal(err)
			}
			id := acssuite.EvidenceIdentity{Cycle: 1, RunID: "run-1", Round: 1, TreeSHA: snap.Tree}
			if err := acssuite.SealEvidence(reportPath, raw, id); err != nil {
				t.Fatal(err)
			}
			entry := map[string]any{
				"role": "auditor", "kind": "agent_subprocess", "run_id": "run-1", "exit_code": 0,
				"artifact_path": reportPath, "artifact_sha256": mustHashFile(t, reportPath),
				"git_head": runGitOut(t, repo, "rev-parse", "HEAD"), "tree_state_sha": treeStateSHA(t, repo),
				"worktree_tree_sha": strings.TrimSpace(runGitOut(t, repo, "write-tree")),
			}
			// git command output includes a newline; the production recorder trims it.
			entry["git_head"] = strings.TrimSpace(entry["git_head"].(string))
			line, _ := json.Marshal(entry)
			mustWrite(t, filepath.Join(repo, ".evolve", "ledger.jsonl"), string(line)+"\n")
			opts := &Options{ProjectRoot: repo, CycleID: 1, RunID: "run-1", AuditRound: 1, Runner: execRunner, NowFn: defaultNow}
			if err := verifyAuditBinding(context.Background(), opts, &RunResult{}); err != nil {
				t.Fatalf("valid host evidence refused: %v", err)
			}
			commit := headSHA(t, repo)
			if err := verifyPostPushPredicateEvidence(context.Background(), opts, &RunResult{}, commit); err != nil {
				t.Fatalf("valid landed evidence refused: %v", err)
			}
			switch mutation {
			case "host-fail":
				entry["exit_code"] = 2
				line, _ := json.Marshal(entry)
				mustWrite(t, filepath.Join(repo, ".evolve", "ledger.jsonl"), string(line)+"\n")
			case "landed-tree":
				id.TreeSHA = strings.Repeat("b", 40)
				if err := acssuite.SealEvidence(reportPath, raw, id); err != nil {
					t.Fatal(err)
				}
				entry["worktree_tree_sha"] = id.TreeSHA
				entry["artifact_sha256"] = mustHashFile(t, reportPath)
				line, _ := json.Marshal(entry)
				mustWrite(t, filepath.Join(repo, ".evolve", "ledger.jsonl"), string(line)+"\n")
			case "delete":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "corrupt":
				mustWrite(t, path, "{broken")
			case "empty":
				mustWrite(t, path, "{}")
			case "other-cycle":
				v.Cycle++
				changed, _ := json.Marshal(v)
				mustWrite(t, path, string(changed))
			case "other-execution":
				mustWrite(t, path, string(raw)+"\n")
			case "other-round":
				opts.AuditRound++
			case "other-run":
				opts.RunID = "run-2"
			case "source":
				mustWrite(t, filepath.Join(repo, "fixture.txt"), "changed after Audit")
			case "untracked-source":
				mustWrite(t, filepath.Join(repo, "helper.go"), "package helper")
			case "report":
				mustWrite(t, reportPath, "## Verdict\n**PASS**\n")
			}
			if err := verifyAuditBinding(context.Background(), opts, &RunResult{}); err == nil {
				t.Fatalf("%s after audit authorized ship", mutation)
			}
			if mutation != "source" && mutation != "untracked-source" {
				if err := verifyPostPushPredicateEvidence(context.Background(), opts, &RunResult{}, commit); err == nil {
					t.Fatalf("%s authorized report-only completion", mutation)
				}
			}
		})
	}
}

func TestShip_PostAuditBinaryChurnRequiresReaudit(t *testing.T) {
	repo := makeRepo(t)
	trackEvolveBinary(t, repo)
	addRemote(t, repo)
	wt := makeWorktree(t, repo, "binary-churn")
	mustWrite(t, filepath.Join(wt, "feature.txt"), "audited work\n")
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"), `{"cycle_id":1,"phase":"ship","active_worktree":"`+wt+`"}`)
	seedAudit(t, repo, "PASS")
	before := headSHA(t, repo)
	mustWrite(t, filepath.Join(wt, "go", "evolve"), "mutated after host execution\n")
	res, err := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "must re-audit"})
	if err == nil || res.ExitCode == ExitOK {
		t.Fatal("post-audit binary change authorized a ship")
	}
	if headSHA(t, repo) != before {
		t.Fatal("tree mismatch changed main HEAD")
	}
}
