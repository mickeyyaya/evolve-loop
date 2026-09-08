//go:build integration

package audit

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestNewDefault_HostPredicateExecutionBindsCompleteEvidence(t *testing.T) {
	for _, pass := range []bool{true, false} {
		name := "red"
		if pass {
			name = "green"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeGoPredFixture(t, root, 7, pass)
			if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".evolve/\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("git", "init", "-q", root)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git init: %v %s", err, out)
			}
			if out, err := exec.Command("git", "-C", root, "add", "-A").CombinedOutput(); err != nil {
				t.Fatalf("stage Builder fixture: %v %s", err, out)
			}
			ws := filepath.Join(root, ".evolve", "runs", "cycle-7")
			if err := os.MkdirAll(ws, 0o755); err != nil {
				t.Fatal(err)
			}
			// An apparent green from the wrong cycle must not suppress execution.
			if err := os.WriteFile(filepath.Join(ws, acssuite.VerdictFilename), []byte(`{"cycle":42,"red_count":0}`), 0o644); err != nil {
				t.Fatal(err)
			}
			req := core.PhaseRequest{Cycle: 7, RunID: "run-7", AuditRound: 1, ProjectRoot: root, Worktree: root, Workspace: ws}
			phase := NewDefault(&fakeBridge{writeArtifact: "## Verdict\n**PASS**\n"}, fakePromptsFS("body"))
			resp, err := phase.Run(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			want := core.VerdictFAIL
			if pass {
				want = core.VerdictPASS
			}
			if resp.Verdict != want {
				t.Fatalf("verdict %s, want %s; diagnostics=%+v", resp.Verdict, want, resp.Diagnostics)
			}
			raw, err := os.ReadFile(filepath.Join(ws, acssuite.VerdictFilename))
			if err != nil {
				t.Fatal(err)
			}
			report, err := os.ReadFile(filepath.Join(ws, "audit-report.md"))
			if err != nil {
				t.Fatal(err)
			}
			tree, err := predicateTreeFor(req)
			if err != nil {
				t.Fatal(err)
			}
			v, err := acssuite.VerifyEvidence(string(report), raw, acssuite.EvidenceIdentity{Cycle: 7, RunID: "run-7", Round: 1, TreeSHA: tree})
			if err != nil {
				t.Fatal(err)
			}
			if v.ShipEligible != pass || v.PredicateSuite.Total != 1 {
				t.Fatalf("wrong execution evidence: %+v", v)
			}
			candidates, err := filepath.Glob(filepath.Join(ws, "acs-verdict.candidate.*.json"))
			if err != nil || len(candidates) != 1 {
				t.Fatalf("candidate not preserved: %v %v", candidates, err)
			}
		})
	}
}

func TestBeginPredicateEvidence_UnstagedInputsRefuseAudit(t *testing.T) {
	root := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(root, "source.go"), []byte("package source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-C", root, "add", "source.go")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v %s", err, out)
	}
	before, err := os.ReadFile(filepath.Join(root, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "helper.go"), []byte("package source\nfunc helper() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = beginPredicateEvidence(core.PhaseRequest{Cycle: 1, RunID: "run-1", AuditRound: 1, Worktree: root})
	if err == nil {
		t.Fatal("undeclared helper can affect execution but would be absent from the ship tree")
	}
	after, err := os.ReadFile(filepath.Join(root, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("audit adopted undeclared input into the real index")
	}
}
