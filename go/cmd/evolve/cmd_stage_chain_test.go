package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

const (
	ladderStageKey = "workflow.retry_ladder.stage"
	heldStageKey   = "held_proposals.stage"
	stageE2EPolicy = `{"workflow":{"retry_ladder":{"stage":"shadow"}},"held_proposals":{"stage":"shadow"}}`
	rejectedBuild  = "build deliverable failed contract: [cover_changed] 11 of 26 changed functions are below 100%"
)

type rejectingBuild struct{}

func (rejectingBuild) Name() string { return "build" }

func (rejectingBuild) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	return core.PhaseResponse{Phase: "build", Verdict: core.VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

func (rejectingBuild) Review(_ context.Context, in core.ReviewInput) core.ReviewResult {
	if in.Phase != "build" {
		return core.ReviewResult{Approve: true}
	}
	return core.ReviewResult{Reason: rejectedBuild}
}

func stageE2ERoot(t *testing.T) string {
	t.Helper()
	repo := gittest.Fixture(t)
	for rel, body := range map[string]string{
		".evolve/policy.json":           stageE2EPolicy,
		".evolve/profiles/builder.json": `{"name":"builder","role":"builder","cli":"agy-tmux","model_tier_default":"balanced"}`,
		"go/go.mod":                     "module fixture\n\ngo 1.23\n",
		"go/internal/retryladder/r.go":  "package retryladder\n",
		"go/internal/heldproposal/h.go": "package heldproposal\n",
		".gitignore":                    ".evolve/runs/\n.evolve/worktrees/\n.evolve/stage-evidence/\n.evolve/ledger*\n.evolve/state.json\n",
	} {
		fixtures.MustWrite(t, filepath.Join(repo.Dir, filepath.FromSlash(rel)), body)
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "fixture")
	return repo.Dir
}

func runStageE2ECycle(t *testing.T, root string) (*ledger.FileLedger, []signalcenter.Event) {
	t.Helper()
	evolveDir := filepath.Join(root, ".evolve")
	ld := ledger.New(evolveDir)
	center := signalcenter.New()
	var events []signalcenter.Event
	center.Subscribe(func(e signalcenter.Event) { events = append(events, e) })
	site := newStageEvidenceSite(root, ld, center)
	runners := map[core.Phase]core.PhaseRunner{}
	for _, p := range []core.Phase{core.PhaseIntent, core.PhaseScout, core.PhaseTriage, core.PhaseTDD, core.PhaseBuildPlanner, core.PhaseAudit, core.PhaseRetro, core.PhaseDebugger} {
		runners[p] = &fixtures.FakeRunner{PhaseName: string(p)}
	}
	runners[core.PhaseBuild] = rejectingBuild{}
	wf := policy.Policy{}.WorkflowConfig()
	wf.RetryLadder.Stage = policy.RetryLadderShadow
	o := core.NewOrchestrator(storage.New(evolveDir), ld, runners, core.WithReviewer(rejectingBuild{}), core.WithStageEvidence(site.coreEvidence()),
		core.WithWorkflowConfig(wf), core.WithRetryConfig(policy.RetryConfig{ContractCorrectionRetries: 1}), core.WithSignalCenter(center))
	_, _ = o.RunCycle(context.Background(), core.CycleRequest{ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true})
	return ld, events
}

func reportByKey(t *testing.T, root string) map[string]stageevidence.Result {
	t.Helper()
	out, errOut, code := runStageVerb(t, "report", "--json", "--project-root", root)
	if code != cliExitOK {
		t.Fatalf("stage report exited %d: %s", code, errOut)
	}
	var results []stageevidence.Result
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatal(err)
	}
	byKey := map[string]stageevidence.Result{}
	for _, r := range results {
		byKey[r.Key] = r
	}
	return byKey
}

func noStageFault(t *testing.T, events []signalcenter.Event) {
	t.Helper()
	for _, e := range events {
		if e.Code == core.CodeStageEvidenceFault {
			t.Fatalf("the cycle raised a stage evidence fault: %+v", e)
		}
	}
}

func TestStageLedgerReaderFollowsSymlinkedLedgerToItsStore(t *testing.T) {
	t.Parallel()
	realRoot := stageE2ERoot(t)
	runStageE2ECycle(t, realRoot)

	worktreeRoot := stageE2ERoot(t)
	link := filepath.Join(worktreeRoot, ".evolve", "ledger.jsonl")
	if err := os.Symlink(filepath.Join(realRoot, ".evolve", "ledger.jsonl"), link); err != nil {
		t.Fatal(err)
	}

	byKey := reportByKey(t, worktreeRoot)
	if r := byKey[ladderStageKey]; r.Decisions != 1 || r.Error != "" || len(r.IncompleteScopes) != 0 {
		t.Errorf("symlinked ladder line = %+v, want one counted decision from the target store and no error", r)
	}
}

func TestStageReportWithoutChainReadsLedger(t *testing.T) {
	t.Parallel()
	root := stageE2ERoot(t)
	_, events := runStageE2ECycle(t, root)
	noStageFault(t, events)
	byKey := reportByKey(t, root)
	if r := byKey[ladderStageKey]; r.Decisions != 1 || r.Error != "" || len(r.IncompleteScopes) != 0 {
		t.Errorf("ladder line = %+v, want one counted decision from the ledger and nothing incomplete", r)
	}
	if r := byKey["inbox.curator.stage"]; r.Error != "empty-chain" {
		t.Errorf("a key with no emitter reads %q, want empty-chain", r.Error)
	}
}

func TestShipChildSessionIsCoveredBySeal(t *testing.T) {
	t.Parallel()
	root := stageE2ERoot(t)
	ld := ledger.New(filepath.Join(root, ".evolve"))
	center := signalcenter.New()
	var events []signalcenter.Event
	center.Subscribe(func(e signalcenter.Event) { events = append(events, e) })
	site := newStageEvidenceSite(root, ld, center)
	evidence := site.coreEvidence()

	parent := evidence.Open(1)
	child := site.shipOpener()(1)
	child.Record(heldStageKey, map[string]string{"would_hold": "true", "cycle": "1", "tree": "t", "reasons": "protected"})
	child.Close()
	parent.Record(ladderStageKey, map[string]string{"would_stop": "false"})
	parent.Close()
	if _, err := evidence.Sealer.SealStageEvidence(context.Background(), stageevidence.CycleScope(1), stageevidence.SealInput{Exit: "completed"}); err != nil {
		t.Fatal(err)
	}
	noStageFault(t, events)

	byKey := reportByKey(t, root)
	for _, key := range []string{heldStageKey, ladderStageKey} {
		if r := byKey[key]; r.Decisions != 1 || r.Error != "" || len(r.IncompleteScopes) != 0 {
			t.Errorf("%s line = %+v, want the decision of one session counted in the one seal and nothing incomplete", key, r)
		}
	}
}

func TestAShipRecordAfterTheSealMakesTheScopeReadIncompleteAndCountsNothing(t *testing.T) {
	t.Parallel()
	root := stageE2ERoot(t)
	ld := ledger.New(filepath.Join(root, ".evolve"))
	center := signalcenter.New()
	site := newStageEvidenceSite(root, ld, center)
	evidence := site.coreEvidence()

	parent := evidence.Open(1)
	parent.Record(ladderStageKey, map[string]string{"would_stop": "false"})
	parent.Close()
	if _, err := evidence.Sealer.SealStageEvidence(context.Background(), stageevidence.CycleScope(1), stageevidence.SealInput{Exit: "completed"}); err != nil {
		t.Fatal(err)
	}
	child := site.shipOpener()(1)
	child.Record(heldStageKey, map[string]string{"would_hold": "true", "cycle": "1", "tree": "t", "reasons": "protected"})
	child.Close()

	byKey := reportByKey(t, root)
	for _, key := range []string{heldStageKey, ladderStageKey} {
		if r := byKey[key]; r.Decisions != 0 {
			t.Errorf("%s line = %+v, want no counted decision: the unsealed session fails closed", key, r)
		}
	}
	if ladder := byKey[ladderStageKey]; !slices.Equal(ladder.IncompleteScopes, []string{"cycle-1"}) {
		t.Errorf("ladder line = %+v, want cycle-1 listed as incomplete", ladder)
	}
}

func TestStageChainFlagStillReadsFixture(t *testing.T) {
	t.Parallel()
	root := stageE2ERoot(t)
	runStageE2ECycle(t, root)
	out, _, _ := runStageVerb(t, "verify", ladderStageKey, "--json", "--project-root", root)
	version := decodeResult(t, out).Version
	chain := stageevidence.Chain{}
	for i := 1; i <= 2; i++ {
		rec := stageevidence.Link("", stageevidence.Record{Key: ladderStageKey, Version: version, Kind: stageevidence.KindDecision, ID: 40 + i, Seq: i, Fields: map[string]string{"would_stop": "false"}})
		chain.Records = append(chain.Records, rec)
		chain.Outcomes = append(chain.Outcomes, stageevidence.Outcome{ID: 40 + i, Shipped: true})
	}
	out, _, _ = runStageVerb(t, "verify", ladderStageKey, "--json", "--project-root", root, "--chain", writeStageChain(t, chain))
	if res := decodeResult(t, out); res.Decisions != 2 || res.FirstID != 41 {
		t.Errorf("verify with --chain: decisions %d first id %d, want the 2 fixture decisions of cycles 41 and 42 and not the one ledger decision", res.Decisions, res.FirstID)
	}
}

func TestStageVerbsTypeTheLedgerAndStoreFaultsAndNeverScorePartially(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		damage func(t *testing.T, root string)
		code   string
	}{
		"corrupt store": {code: "corrupt-store", damage: func(t *testing.T, root string) {
			matches, err := filepath.Glob(filepath.Join(root, ".evolve", "ledger-artifacts", "sha256", "*", "*"))
			if err != nil || len(matches) == 0 {
				t.Fatalf("no stored copy to damage: %v", err)
			}
			if err := os.Chmod(matches[0], 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(matches[0], []byte("rewritten\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		"broken ledger": {code: "ledger-broken", damage: func(t *testing.T, root string) {
			f, err := os.OpenFile(filepath.Join(root, ".evolve", "ledger.jsonl"), os.O_APPEND|os.O_WRONLY, 0o644)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			if _, err := f.WriteString("{\"kind\":\"stage-evidence-seal\" not json\n"); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		root := stageE2ERoot(t)
		runStageE2ECycle(t, root)
		tc.damage(t, root)

		out, errOut, rc := runStageVerb(t, "verify", ladderStageKey, "--json", "--project-root", root)
		res := decodeResult(t, out)
		if rc != cliExitFailed || res.Ready || res.Error != tc.code || res.Decisions != 0 {
			t.Errorf("%s: verify exit %d ready %v error %q decisions %d (stderr %s), want not ready, typed %q and no partial verdict", name, rc, res.Ready, res.Error, res.Decisions, errOut, tc.code)
		}
		if byKey := reportByKey(t, root); byKey[ladderStageKey].Error != tc.code || byKey[heldStageKey].Error != tc.code {
			t.Errorf("%s: report errors = %q and %q, want %q on every key", name, byKey[ladderStageKey].Error, byKey[heldStageKey].Error, tc.code)
		}
	}
}

func TestStageEvidenceSiteFaultReachesTheSignalCenterAsTheStageFaultCode(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".evolve"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	center := signalcenter.New()
	rec := newStageEvidenceSite(root, ledger.New(filepath.Join(root, "ledger")), center).open(7)

	rec.Record(ladderStageKey, map[string]string{"would_stop": "true"})
	rec.Close()
	center.Flush()

	var faults []signalcenter.Event
	for _, ev := range center.Recent() {
		if ev.Code == core.CodeStageEvidenceFault {
			faults = append(faults, ev)
		}
	}
	if len(faults) == 0 || faults[0].Cycle != 7 || faults[0].Fields["error"] == "" {
		t.Fatalf("signals = %+v, want %s for cycle 7 naming the cause", center.Recent(), core.CodeStageEvidenceFault)
	}
}

func TestStageEvidenceSiteVersionNamesWhyADecisionVersionIsMissing(t *testing.T) {
	t.Parallel()
	missing := t.TempDir()
	if _, err := newStageEvidenceSite(missing, nil, nil).versionOf()(ladderStageKey); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("no policy.json: err = %v, want read policy.json", err)
	}
	withPolicy := t.TempDir()
	if err := os.MkdirAll(filepath.Join(withPolicy, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(withPolicy, ".evolve", "policy.json"), []byte(stageE2EPolicy), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := newStageEvidenceSite(withPolicy, nil, nil).versionOf()("no.such.stage"); !errors.Is(err, errUnknownStageKey) {
		t.Errorf("unknown key: err = %v, want unknown stage key", err)
	}
}

func TestStageBuildIDNamesTheVCSRevisionAndFallsBackToUnknown(t *testing.T) {
	t.Parallel()
	withRevision := &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "GOOS", Value: "darwin"}, {Key: "vcs.revision", Value: "abc123"}}}
	emptyRevision := &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: ""}}}
	cases := []struct {
		name string
		info *debug.BuildInfo
		ok   bool
		want string
	}{
		{"revision", withRevision, true, "abc123"},
		{"empty revision", emptyRevision, true, "unknown"},
		{"no revision setting", &debug.BuildInfo{}, true, "unknown"},
		{"no build info", nil, false, "unknown"},
	}
	for _, tc := range cases {
		if got := buildIDOf(tc.info, tc.ok); got != tc.want {
			t.Errorf("%s: buildIDOf = %q, want %q", tc.name, got, tc.want)
		}
	}
	if stageBuildID() == "" {
		t.Error("stageBuildID is empty, want a revision or unknown")
	}
}

func TestWireOrchestratorDepsWiresTheStageEvidenceOfTheCycleAndOfItsShip(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)

	deps := wireOrchestratorDeps(root, evolveDir, io.Discard, routingRun{})

	if deps.RoutingErr != nil {
		t.Fatalf("wireOrchestratorDeps: %v", deps.RoutingErr)
	}
	if !deps.Orchestrator.StageEvidenceWired() {
		t.Error("the orchestrator has no stage evidence: no cycle records or seals")
	}
	shipPhase, ok := deps.Runners[core.PhaseShip].(*ship.Phase)
	if !ok || !shipPhase.StageEvidenceWired() {
		t.Errorf("ship runner = %T, want a ship phase with a stage evidence port", deps.Runners[core.PhaseShip])
	}
}

func TestRunShipCmdHandsShipTheStageEvidencePort(t *testing.T) {
	t.Setenv("EVOLVE_PLUGIN_ROOT", "")
	calls := stubShipRun(t, ship.RunResult{}, nil)

	rc := runShipCmd([]string{"--project-root", t.TempDir(), "msg"}, nil, io.Discard, io.Discard)

	if rc != 0 || len(*calls) != 1 || (*calls)[0].StageEvidence == nil {
		t.Fatalf("rc = %d, calls = %d, want one ship run that carries the stage evidence port", rc, len(*calls))
	}
}
