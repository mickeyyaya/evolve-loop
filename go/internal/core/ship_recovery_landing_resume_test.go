//go:build integration

package core_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/acsverdict"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/treefence"
)

const (
	resumeCycle  = 1830
	resumeRunID  = "run-1830"
	resumeItemID = "cli-phase-cycle-request"
	resumeItem   = ".evolve/inbox/2026-09-30T10-51-14Z-cli-phase-cycle-request.json"
)

type outageLane struct {
	root, wt, ws, base string
	view               *phaseio.ExplanationView
}

func writeResumeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newOutageLane(t *testing.T) outageLane {
	t.Helper()
	repo, remote := gittest.Fixture(t), gittest.Bare(t)
	l := outageLane{root: repo.Dir}
	writeResumeFile(t, filepath.Join(l.root, ".gitignore"), ".evolve/\n!.evolve/\n.evolve/*\n!.evolve/inbox/\n")
	writeResumeFile(t, filepath.Join(l.root, "fixture.txt"), "fixture line 1\n")
	writeResumeFile(t, filepath.Join(l.root, filepath.FromSlash(resumeItem)), `{"id":"`+resumeItemID+`"}`+"\n")
	writeResumeFile(t, filepath.Join(l.root, ".evolve", "state.json"), "{}\n")
	repo.Git("add", "-A")
	repo.Git("-c", "commit.gpgsign=false", "commit", "-q", "-m", "base with the lane's inbox item")
	repo.Git("remote", "add", "origin", remote.Dir)
	repo.Git("push", "-q", "origin", "main")
	l.base = repo.Git("rev-parse", "HEAD")
	l.wt = filepath.Join(t.TempDir(), "wt")
	repo.Git("worktree", "add", "-q", "-b", "cycle-1830-branch", l.wt, "main")
	l.ws = filepath.Join(l.root, ".evolve", "runs", fmt.Sprintf("cycle-%d", resumeCycle))
	writeResumeFile(t, filepath.Join(l.root, ".evolve", "cycle-state.json"), fmt.Sprintf(
		`{"cycle_id":%d,"run_id":%q,"active_worktree":%q,"workspace_path":%q,"worktree_base_sha":%q}`, resumeCycle, resumeRunID, l.wt, l.ws, l.base))
	writeResumeFile(t, filepath.Join(l.ws, "triage-decision.json"), `{"schema_version":1,"top_n":[{"id":"`+resumeItemID+`"}],"deferred":[],"dropped":[]}`)
	l.view = sealOutageExplanation(t, l)
	runGitT(t, l.wt, "add", "-A")
	sealOutageAudit(t, l, repo.Git("rev-parse", "HEAD"))
	return l
}

func sealOutageExplanation(t *testing.T, l outageLane) *phaseio.ExplanationView {
	t.Helper()
	binding := explanationdocs.CycleBinding{ProjectRoot: l.root, Worktree: l.wt, Workspace: l.ws, BaseSHA: l.base,
		Cycle: resumeCycle, RunID: resumeRunID, ContractVersion: explanationdocs.CurrentContractVersion}
	activation := binding
	activation.Worktree, activation.BaseSHA = "", ""
	if err := explanationdocs.Activate(activation); err != nil {
		t.Fatal(err)
	}
	if err := explanationdocs.SealBuild(binding); err != nil {
		t.Fatal(err)
	}
	document, err := explanationdocs.DocumentPath(resumeCycle, resumeRunID)
	if err != nil {
		t.Fatal(err)
	}
	writeResumeFile(t, filepath.Join(l.wt, "fixture.txt"), "fixture line 1\nthe audited cycle work\n")
	writeResumeFile(t, filepath.Join(l.wt, filepath.FromSlash(document)), "# Build Explanation — Cycle 1830\n\n"+
		"## Build Binding\n- Cycle: 1830\n- Base SHA: "+l.base+"\n\n"+
		"## Summary\nThe fixture records the audited cycle work.\n\n"+
		"## Rationale\nThe tracked fixture is the smallest material change a landing can carry.\n\n"+
		"## Changed Areas\n- `fixture.txt` — adds the audited line the landing publishes.\n\n"+
		"## Design Decisions\nThe lane lands through the real worktree ship.\n\n"+
		"## Verification\nThe recovery resumes the ship at its push.\n\n"+
		"## Compatibility\nNo public interface changes.\n\n"+
		"## Limitations\nCovers the worktree landing only.\n")
	writeResumeFile(t, filepath.Join(l.ws, "build-report.md"), "## Explanation Documentation\n- Status: REQUIRED\n- Document: "+document+"\n")
	if failures := explanationdocs.CheckBuild(context.Background(), binding); len(failures) != 0 {
		t.Fatalf("CheckBuild: %v", failures)
	}
	view, err := explanationdocs.Load(l.ws)
	if err != nil {
		t.Fatal(err)
	}
	if err := explanationdocs.SealResult(context.Background(), binding); err != nil {
		t.Fatal(err)
	}
	return view
}

func sealOutageAudit(t *testing.T, l outageLane, head string) {
	t.Helper()
	snap, err := treefence.TakeStaged(context.Background(), l.wt, nil)
	if err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(l.ws, "audit-report.md")
	writeResumeFile(t, report, "# Audit Report — Cycle 1830\n\nVerdict: PASS\n")
	verdict, err := json.Marshal(acssuite.Verdict{SchemaVersion: "1.0", Cycle: resumeCycle, GreenCount: 1, Verdict: "PASS", ShipEligible: true,
		PredicateSuite: acssuite.PredicateSuite{Total: 1, ThisCycleCount: 1},
		Results:        []acssuite.Result{{ACID: "predicate-0", Predicate: "go/acs/cycle/...:predicate-0", ResultStr: "green"}}})
	if err != nil {
		t.Fatal(err)
	}
	writeResumeFile(t, filepath.Join(l.ws, acsverdict.Filename), string(verdict))
	if err := acssuite.SealEvidence(report, verdict, acssuite.EvidenceIdentity{Cycle: resumeCycle, RunID: resumeRunID, Round: 1, TreeSHA: snap.Tree}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	row, err := json.Marshal(map[string]any{
		"ts": "2026-10-07T15:05:22Z", "cycle": resumeCycle, "run_id": resumeRunID, "worktree_tree_sha": snap.Tree,
		"role": "auditor", "kind": "agent_subprocess", "model": "opus", "exit_code": 0, "duration_s": "377",
		"artifact_path": report, "artifact_sha256": hex.EncodeToString(sum[:]), "challenge_token": "tok",
		"git_head": head, "tree_state_sha": hex.EncodeToString(sum[:]), "worktree_base_sha": l.base,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeResumeFile(t, filepath.Join(l.root, ".evolve", "ledger.jsonl"), string(row)+"\n")
}

type serverOutage struct{ failures, pushes int }

func (o *serverOutage) runner() sysexec.RunFunc {
	return func(ctx context.Context, name, cwd string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name == "git" && len(args) > 0 && args[0] == "push" {
			o.pushes++
			if o.pushes <= o.failures {
				_, _ = io.WriteString(stderr, "! [remote rejected] main -> main (Internal Server Error)\n")
				return 1, nil
			}
		}
		return sysexec.DefaultRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
	}
}

func (l outageLane) shipRequest() core.PhaseRequest {
	return core.PhaseRequest{
		Cycle: resumeCycle, RunID: resumeRunID, AuditRound: 1, ProjectRoot: l.root, Worktree: l.wt, Workspace: l.ws,
		WorktreeBaseSHA: l.base, ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion, BuildExplanation: l.view,
		Context: map[string]string{"commit_message": "evolve-cycle 1830"},
	}
}

func TestRecoverFromShipError_ATransientPushFailureResumesTheShipWithoutAReaudit(t *testing.T) {
	l := newOutageLane(t)
	outage := &serverOutage{failures: 4}
	phase := ship.New(ship.Config{Runner: outage.runner(), Sleep: func(time.Duration) {}})
	o := core.NewOrchestrator(&recStorage{}, &fakeLedger{}, newRunners(nil))
	cs := core.CycleState{CycleID: resumeCycle, RunID: resumeRunID, ActiveWorktree: l.wt, WorkspacePath: l.ws,
		WorktreeBaseSHA: l.base, ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion}

	depth := 0
	for ; ; depth++ {
		resp, err := phase.Run(context.Background(), l.shipRequest())
		if err == nil && resp.Verdict == core.VerdictPASS {
			break
		}
		se, ok := core.AsShipError(err)
		if !ok {
			t.Fatalf("ship %d failed without a ShipError: %v", depth+1, err)
		}
		next, recovered := o.RecoverFromShipErrorForTest(context.Background(), l.root, resumeCycle, &cs, se, depth, 1)
		if !recovered || next != core.PhaseShip {
			t.Fatalf("ship %d failed with %s/%s (%s) and recovery routed to %q (recovered=%v): a transient push failure must resume the ship at its push, never re-dispatch Audit", depth+1, se.Code, se.Class, se.Message, next, recovered)
		}
	}

	if depth != 1 {
		t.Errorf("the ship landed at recovery depth %d, want 1: one transient failure, one resumed ship", depth)
	}
}
