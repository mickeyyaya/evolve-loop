package triage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

type journeyBridge struct {
	report, decision string
	launches         int
}

func (b *journeyBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.launches++
	dir := filepath.Dir(req.ArtifactPath)
	report := b.report
	if tok, err := os.ReadFile(filepath.Join(dir, "challenge-token.txt")); err == nil {
		report = "<!-- challenge-token: " + strings.TrimSpace(string(tok)) + " -->\n" + report
	}
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(req.ArtifactPath, []byte(report), 0o644)
	if b.decision != "" {
		_ = os.WriteFile(filepath.Join(dir, "triage-decision.json"), []byte(b.decision), 0o644)
	}
	return core.BridgeResponse{Stdout: report}, nil
}

func (b *journeyBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

type triageReviewOnly struct{ inner core.DeliverableReviewer }

func (r triageReviewOnly) Review(ctx context.Context, in core.ReviewInput) core.ReviewResult {
	if in.Phase != string(core.PhaseTriage) {
		return core.ReviewResult{Approve: true}
	}
	return r.inner.Review(ctx, in)
}

func journeyCatalog(t *testing.T) phasespec.Catalog {
	t.Helper()
	cat, err := phasespec.Load(filepath.Join("..", "..", "..", "..", "docs", "architecture", "phase-registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func journeyRepo(t *testing.T) string {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write(".gitignore", ".evolve/\ngo/bin/\n")
	repo.Git("add", ".gitignore")
	repo.Git("commit", "-q", "-m", "base")
	return repo.Dir
}

func runRouteJourney(t *testing.T, bridge *journeyBridge, failClaims int32) (core.CycleResult, error, map[core.Phase]core.PhaseRunner, string) {
	t.Helper()
	root := journeyRepo(t)
	writeInboxItem(t, root, "2026-01-01T00-00-00Z-x.json", `{"id":"x","title":"routed","weight":0.9,"route":"console-only"}`)
	writeInboxItem(t, root, "2026-01-01T00-00-01Z-y.json", `{"id":"y","title":"lane work","weight":0.8}`)
	if err := committedset.WriteRecord(root, committedset.Record{Cycle: 1, RunID: "r", ItemIDs: []string{"x", "y"}, GoalHash: "g"}); err != nil {
		t.Fatal(err)
	}
	cat := journeyCatalog(t)
	var calls atomic.Int32
	claim := func(dir string, cycle int, ids []string) error {
		if calls.Add(1) <= failClaims {
			return errors.New("disk full")
		}
		return inboxmover.ClaimPending(inboxmover.Options{InboxDir: dir, Stderr: io.Discard, Ledger: ledger.New(filepath.Dir(dir))}, cycle, ids)
	}
	runners := fixtures.BuildRunners(nil)
	evolveDir := filepath.Join(root, ".evolve")
	reviewer := deliverable.NewReviewerWithCatalogStageReportSize(config.StageEnforce, cat, config.StageEnforce, config.StageOff, 0)
	effects := deliverable.NewHostEffects(cat, claim, nil)
	runners[core.PhaseTriage] = New(Config{
		Bridge: bridge, Prompts: fakePromptsFS("body"), PhaseIO: config.StageEnforce,
		ContractVerifier: func() runner.ContractVerifier { return reviewer },
		HostEffects:      func() core.HostEffects { return effects },
	})
	o := core.NewOrchestrator(storage.New(evolveDir), ledger.New(evolveDir), runners,
		core.WithCatalog(cat), core.WithReviewer(triageReviewOnly{reviewer}),
		core.WithHostEffects(effects))
	res, err := o.RunCycle(context.Background(), core.CycleRequest{ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true})
	return res, err, runners, root
}

func TestRouteJourney_APinOfARoutedAndAnUnroutedItemBuildsOnlyTheUnrouted(t *testing.T) {
	bridge := &journeyBridge{
		report:   "# Triage Decision\n\n## top_n\n- x: routed — priority=H, source={inbox}\n- y: lane work — priority=H, source={inbox}\n",
		decision: `{"cycle":1,"top_n":[{"id":"x"},{"id":"y"}],"deferred":[]}`,
	}

	res, err, runners, root := runRouteJourney(t, bridge, 0)

	if err != nil {
		t.Fatalf("RunCycle: %v (verdict=%s termination=%s)", err, res.FinalVerdict, res.TerminationReason)
	}
	if bridge.launches != 1 {
		t.Fatalf("the gate owes no correction for a host route: triage launched %d times", bridge.launches)
	}
	if got := runners[core.PhaseBuild].(*fixtures.FakeRunner).Calls; got != 1 {
		t.Fatalf("y must reach build once, got %d (phases %v)", got, res.PhasesRun)
	}
	ws := filepath.Join(root, ".evolve", "runs", "cycle-1")
	if got, _ := committedset.Committed(ws); len(got) != 1 || got[0] != "y" {
		t.Fatalf("x left the commitment and y stays: %v", got)
	}
	if top, _ := committedset.DecisionTopN(ws); len(top) != 1 || top[0] != "y" {
		t.Fatalf("x must not reach build through top_n: %v", top)
	}
}

func TestRouteJourney_AClaimThatFailsOnceFailsTheCycleWithTheHostTextAndBuildsNothing(t *testing.T) {
	bridge := &journeyBridge{
		report:   "# Triage Decision\n\n## top_n\n- x: routed — priority=H, source={inbox}\n- y: lane work — priority=H, source={inbox}\n",
		decision: `{"cycle":1,"top_n":[{"id":"x"},{"id":"y"}],"deferred":[]}`,
	}

	res, _, runners, _ := runRouteJourney(t, bridge, 1)

	if res.FinalVerdict != core.VerdictFAIL {
		t.Fatalf("a claim that failed once fails the cycle: verdict=%s", res.FinalVerdict)
	}
	if got := runners[core.PhaseBuild].(*fixtures.FakeRunner).Calls; got != 0 {
		t.Fatalf("nothing builds after the failed claim, got %d", got)
	}
	want := "review gate: the host tried to route and claim committed item \"y\", and neither landed."
	joined := strings.Join(res.FailReasons, "\n")
	if !strings.Contains(joined, want) || !strings.Contains(joined, "evolve signals tail --cycle 1 --code RUNNER_HOST_EFFECT_FAILED") {
		t.Fatalf("the failure reason must name the host attempt and the next step:\n%s", joined)
	}
	if strings.Contains(joined, "The host claims it after this phase ends") {
		t.Fatalf("the cell-form text is false after the host tried:\n%s", joined)
	}
}
