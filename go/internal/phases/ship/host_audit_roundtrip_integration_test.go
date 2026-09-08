//go:build integration

package ship

import (
	"context"
	"errors"
	"go/format"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/audit"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// This joins the real native Audit, the orchestrator's private host recorder,
// and Ship's binding verifier. Only the model and non-audit phases are replaced.
func TestHostAuditRoundTrip_CoreLedgerAuthorizesOnlyHostPass(t *testing.T) {
	for _, pass := range []bool{true, false} {
		name, assertion := "green", ""
		if !pass {
			name, assertion = "red", `t.Fatal("required behavior absent")`
		}
		t.Run(name, func(t *testing.T) {
			repo := makeRepo(t)
			mustWrite(t, filepath.Join(repo, "go", "go.mod"), "module fixture\n\ngo 1.23\n")
			source, err := format.Source([]byte("package cycle7\nimport \"testing\"\nfunc TestRequiredBehavior(t *testing.T) { " + assertion + " }\n"))
			if err != nil {
				t.Fatal(err)
			}
			mustWrite(t, filepath.Join(repo, "go", "acs", "cycle7", "predicate_test.go"), string(source))
			// The committed fixture also reaches the native durable-ACS check.
			mustWrite(t, filepath.Join(repo, "go", "acs", "regression", "doc.go"), "package regression\n")
			runGit(t, repo, "add", "--", "go")
			ws := core.RunWorkspacePath(repo, 7)
			mustMkdir(t, ws)
			st := &fixtures.FakeStorage{State: core.State{LastAllocatedCycleNumber: 7}, CycleState: core.CycleState{
				CycleID: 7, RunID: "host-roundtrip", Phase: "audit", WorkspacePath: ws, ActiveWorktree: repo,
				CompletedPhases: []string{"scout", "triage", "tdd", "build"},
			}}
			runners := fixtures.BuildRunners(nil)
			native := audit.NewDefault(&fixtures.FakeBridge{WriteArtifact: "## Verdict\n**PASS**\n"},
				prompts.NewFromFS(fstest.MapFS{"agents/evolve-auditor.md": {Data: []byte("audit fixture")}}))
			var captured core.PhaseRequest
			var hostVerdict string
			runners[core.PhaseAudit] = &hostBindingProbeRunner{name: "audit", run: func(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
				captured = req
				resp, err := native.Run(ctx, req)
				hostVerdict = resp.Verdict
				if pass && resp.Verdict != core.VerdictPASS {
					t.Logf("native audit diagnostics: %+v", resp.Diagnostics)
				}
				return resp, err
			}}
			checked := false
			check := func(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
				checked = true
				err := verifyAuditBinding(ctx, &Options{ProjectRoot: repo, WorkspacePath: ws, ActiveWorktree: repo, CycleID: 7, RunID: captured.RunID, AuditRound: captured.AuditRound, Runner: execRunner, NowFn: defaultNow}, &RunResult{})
				if pass && err != nil {
					t.Errorf("host PASS receipt rejected after core ledger binding: %v", err)
				}
				if !pass {
					var se *core.ShipError
					if !errors.As(err, &se) || se.Code != core.CodeAuditBindingAuditorExit {
						t.Errorf("host FAIL with narrative PASS accepted or misclassified: %v", err)
					}
				}
				return core.PhaseResponse{Phase: req.PreviousPhase, Verdict: core.VerdictPASS}, nil
			}
			runners[core.PhaseShip] = &hostBindingProbeRunner{name: "ship", run: check}
			runners[core.PhaseRetro] = &hostBindingProbeRunner{name: "retro", run: check}
			o := core.NewOrchestrator(st, ledger.New(filepath.Join(repo, ".evolve")), runners)
			_, err = o.RunCycleFromPhase(context.Background(), core.CycleRequest{ProjectRoot: repo, GoalHash: "verify-native-audit", DisableWorkspaceGuard: true}, &core.ResumePoint{Phase: "audit", CycleID: 7, WorktreePath: repo})
			if err != nil {
				t.Fatal(err)
			}
			want := core.VerdictPASS
			if !pass {
				want = core.VerdictFAIL
			}
			if hostVerdict != want || !checked {
				t.Fatalf("host=%q checked=%v; want %q and consumed binding", hostVerdict, checked, want)
			}
		})
	}
}

type hostBindingProbeRunner struct {
	name string
	run  func(context.Context, core.PhaseRequest) (core.PhaseResponse, error)
}

func (r *hostBindingProbeRunner) Name() string { return r.name }
func (r *hostBindingProbeRunner) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	return r.run(ctx, req)
}
