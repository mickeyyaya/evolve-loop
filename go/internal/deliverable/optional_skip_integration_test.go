//go:build integration

package deliverable

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

const optionalPhase = core.Phase("widget-scan")

type optionalOnlyReviewer struct{ inner core.DeliverableReviewer }

func (r optionalOnlyReviewer) Review(ctx context.Context, in core.ReviewInput) core.ReviewResult {
	if in.Phase != string(optionalPhase) {
		return core.ReviewResult{Approve: true}
	}
	return r.inner.Review(ctx, in)
}

type optionalPlanner struct{}

func (optionalPlanner) Plan(router.RouteInput) (*router.PhasePlan, error) {
	return &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: "scout", Run: true}, {Phase: "tdd", Run: true}, {Phase: "build", Run: true},
		{Phase: string(optionalPhase), Run: true}, {Phase: "audit", Run: true}, {Phase: "ship", Run: true},
	}}, nil
}

func TestOptionalSkip_RealDeliverableGateFreshAndResume(t *testing.T) {
	for _, resume := range []bool{false, true} {
		mode := "fresh"
		if resume {
			mode = "resume"
		}
		for _, tc := range []struct {
			name      string
			err       error
			mandatory bool
			ships     bool
		}{
			{"missing optional persona", core.ErrAgentDocMissing, false, true},
			{"substantive optional failure", errors.New("invalid input"), false, false},
			{"ordinary WARN still needs artifact", nil, false, false},
			{"mandatory persona missing", core.ErrAgentDocMissing, true, false},
		} {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				ctx := context.Background()
				root := gitRepoWithOneCommit(t)
				evolveDir := filepath.Join(root, ".evolve")
				store, log := storage.New(evolveDir), ledger.New(evolveDir)
				cat, warnings := (phasespec.Catalog{}).Merge([]phasespec.PhaseSpec{{
					Name: string(optionalPhase), Optional: true, After: "build", Role: "evaluate",
					Outputs: phasespec.IO{Files: []string{".evolve/runs/cycle-{cycle}/widget-scan-report.md"}},
				}})
				if len(warnings) != 0 {
					t.Fatal(warnings)
				}
				runners := fixtures.BuildRunners(nil)
				opt := &fixtures.FakeRunner{PhaseName: string(optionalPhase), FailErr: tc.err, FailUntil: 99, Verdict: core.VerdictWARN}
				runners[optionalPhase] = opt
				cfg := config.RoutingConfig{Stage: config.StageAdvisory, Mode: config.ModeDynamicLLM,
					Mandatory: []string{"scout", "build", "audit", "ship"}, MaxInsertions: 4,
					Order: []string{"scout", "tdd", "build", string(optionalPhase), "audit", "ship"}}
				if tc.mandatory {
					cfg.Mandatory = append(cfg.Mandatory, string(optionalPhase))
				}
				reviewer := NewReviewerWithCatalog(config.StageEnforce, cat).(*Reviewer)
				// Keep the independent circuit-breaker policy out of this
				// admission test: every allowed correction remains enforced.
				reviewer.threshold = 99
				o := core.NewOrchestrator(store, log, runners, core.WithCatalog(cat),
					core.WithRouting(cfg, router.StaticPreset{}), core.WithPlanner(optionalPlanner{}),
					core.WithReviewer(optionalOnlyReviewer{reviewer}))
				req := core.CycleRequest{ProjectRoot: root, GoalHash: "optional-skip", DisableWorkspaceGuard: true}
				var runErr error
				if resume {
					ws := core.RunWorkspacePath(root, 7)
					if err := os.MkdirAll(ws, 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(ws, "routing-plan.json"), []byte(`[{"phase":"widget-scan","run":true},{"phase":"audit","run":true},{"phase":"ship","run":true}]`), 0o644); err != nil {
						t.Fatal(err)
					}
					sha, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
					if err != nil {
						t.Fatal(err)
					}
					if err := store.WriteCycleState(ctx, core.CycleState{CycleID: 7, RunID: "resume-7", GoalHash: req.GoalHash,
						WorkspacePath: ws, ActiveWorktree: root, WorktreeBaseSHA: strings.TrimSpace(string(sha)),
						ExplanationDocumentationVersion: 1, CompletedPhases: []string{"scout", "tdd", "build"}}); err != nil {
						t.Fatal(err)
					}
					if err := explanationdocs.Activate(explanationdocs.CycleBinding{
						ProjectRoot: root, Worktree: root, Workspace: ws, BaseSHA: strings.TrimSpace(string(sha)),
						Cycle: 7, RunID: "resume-7", ContractVersion: 1,
					}); err != nil {
						t.Fatal(err)
					}
					_, runErr = o.RunCycleFromPhase(ctx, req, &core.ResumePoint{CycleID: 7, Phase: string(optionalPhase), WorktreePath: root})
				} else {
					_, runErr = o.RunCycle(ctx, req)
				}
				shipCalls := runners[core.PhaseShip].(*fixtures.FakeRunner).Calls
				if tc.ships {
					if runErr != nil || shipCalls != 1 || opt.Calls != 1 {
						t.Fatalf("admitted optional skip entered artifact correction or blocked Ship: err=%v optional=%d ship=%d", runErr, opt.Calls, shipCalls)
					}
					raw, err := os.ReadFile(filepath.Join(evolveDir, "ledger.jsonl"))
					if err != nil || !strings.Contains(string(raw), "optional_missing_persona_skip") {
						t.Fatalf("skip lost its durable reason: %v %s", err, raw)
					}
				} else if runErr == nil || shipCalls != 0 {
					t.Fatalf("non-admitted missing deliverable reached Ship: err=%v ship=%d", runErr, shipCalls)
				}
			})
		}
	}
}
