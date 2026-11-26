package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type judgeBridge struct {
	mu    sync.Mutex
	calls []core.BridgeRequest
	exits map[string]int
	cause map[string]string
	reply string
	quiet map[string]int
	echo  bool
	touch func(path string)
}

func (b *judgeBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.mu.Lock()
	b.calls = append(b.calls, req)
	n := len(b.calls)
	b.mu.Unlock()
	if code := b.quiet[req.CLI]; code != 0 {
		return core.BridgeResponse{ExitCode: code}, nil
	}
	if code := b.exits[req.CLI]; code != 0 {
		cause := ""
		if n == 1 || code == 85 {
			cause = b.cause[req.CLI]
		}
		return core.BridgeResponse{ExitCode: code, CauseCode: cause}, errors.New("bridge: exit")
	}
	if b.reply != "" {
		if err := os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755); err != nil {
			return core.BridgeResponse{}, err
		}
		if err := os.WriteFile(req.ArtifactPath, []byte(b.reply), 0o644); err != nil {
			return core.BridgeResponse{}, err
		}
		if b.touch != nil {
			b.touch(req.ArtifactPath)
		}
	}
	resp := core.BridgeResponse{ExitCode: 0}
	if b.echo {
		resp.Stdout = b.reply
	}
	return resp, nil
}

func (b *judgeBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func (b *judgeBridge) launched() []string {
	out := make([]string, len(b.calls))
	for i, c := range b.calls {
		out[i] = c.CLI + "@" + c.Model
	}
	return out
}

type transientOnce struct {
	*judgeBridge
	failed bool
}

func (b *transientOnce) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	if !b.failed && req.CLI == "claude-tmux" {
		b.failed = true
		b.mu.Lock()
		b.calls = append(b.calls, req)
		b.mu.Unlock()
		return core.BridgeResponse{ExitCode: 81, CauseCode: "transient_upstream"}, errors.New("bridge: exit 81")
	}
	return b.judgeBridge.Launch(ctx, req)
}

const judgeTestReplyTemplate = `{"verdicts":[{"ask":"%s","agrees":true,"note":"ok"}]}`

func judgeReply() string { return strings.Replace(judgeTestReplyTemplate, "%s", hcCoverAsk, 1) }

func emptyModelCatalog(t *testing.T) {
	t.Helper()
	empty := t.TempDir()
	bridge.SetModelCatalogDirFn(func() string { return empty })
}

func shippedJudgeRouter(t *testing.T) *cliroute.Router {
	t.Helper()
	router, _, err := loadCLIRouter(checkedInRoot(t), routingHost(io.Discard, time.Now))
	if err != nil {
		t.Fatal(err)
	}
	return router
}

func shippedVendors(t *testing.T) map[string][]string {
	t.Helper()
	pol, err := policy.Load(filepath.Join(checkedInRoot(t), ".evolve", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	return pol.CatalogConfig().AllowedFamilies
}

type judgeRun struct {
	bridge  *judgeBridge
	benched []string
	project hcProject
	call    core.JudgeCall
	runner  core.HandoffJudgeRunner
}

func newJudgeRun(t *testing.T, author handoff.Author, tier string, inner func(*judgeBridge) core.Bridge) *judgeRun {
	t.Helper()
	emptyModelCatalog(t)
	run := &judgeRun{bridge: &judgeBridge{exits: map[string]int{}, reply: judgeReply()}, project: newHCProject(t)}
	run.project.cycle1950()
	var base core.Bridge = run.bridge
	if inner != nil {
		base = inner(run.bridge)
	}
	walked := bridgechain.New(base, nil, bridgechain.WithBench(func(e bridgechain.Escalation) { run.benched = append(run.benched, e.CLI) }))
	run.call = core.JudgeCall{
		Input:  core.ReviewInput{Cycle: 1950, RunID: "run-1", Workspace: run.project.runDir(1950), Worktree: run.project.root, ProjectRoot: run.project.root},
		Pair:   "tdd-build",
		Author: author,
		Tier:   tier,
		Asks:   []string{hcCoverAsk},
	}
	run.runner = handoffJudgeRunner(judgeDeps{walked: walked, router: shippedJudgeRouter(t), vendors: shippedVendors(t)})
	return run
}

func (r *judgeRun) do(t *testing.T) (core.JudgeResult, error) {
	t.Helper()
	return r.runner(context.Background(), r.call)
}

var codexAuthor = handoff.Author{CLI: "codex-tmux", Tier: "balanced"}

func TestHandoffJudge_EnforceNeverLaunchesTheAuthorModelSeat(t *testing.T) {
	run := newJudgeRun(t, handoff.Author{CLI: "claude-tmux", Tier: "balanced"}, "balanced", nil)
	run.bridge.exits["agy-tmux"] = 85
	_, err := run.do(t)
	if err == nil || errors.Is(err, llmroute.ErrNoPermittedAttempt) {
		t.Fatalf("err = %v; a walled admitted seat is a fault, not a refusal", err)
	}
	if got := strings.Join(run.bridge.launched(), " "); got != "agy-tmux@balanced" {
		t.Fatalf("launched %q; the seat on the author model (claude sonnet) must never launch", got)
	}
}

func TestHandoffJudge_ProductionAuthorsAreAllResolved(t *testing.T) {
	emptyModelCatalog(t)
	vendors := shippedVendors(t)
	cases := []struct {
		author handoff.Author
		admits string
	}{
		{handoff.Author{CLI: "claude-tmux", Tier: "balanced"}, "agy-tmux"},
		{handoff.Author{CLI: "agy-tmux", Tier: "balanced"}, "claude-tmux"},
		{handoff.Author{CLI: "agy-claude-tmux", Tier: "balanced"}, "agy-tmux"},
	}
	for _, c := range cases {
		t.Run(c.author.CLI, func(t *testing.T) {
			if _, ok := bridgechain.ServedModel(c.author.CLI, c.author.Tier); !ok {
				t.Fatalf("author %+v does not resolve through the manifest", c.author)
			}
			if !independentSeat(c.author, vendors)(c.admits, "balanced") {
				t.Fatalf("a seat of %s must be independent of %+v", c.admits, c.author)
			}
			if independentSeat(c.author, vendors)(c.author.CLI, c.author.Tier) {
				t.Fatalf("the author seat itself must be refused")
			}
		})
	}
}

func TestHandoffJudge_ASeatThatIsADriverNameIsFiltered(t *testing.T) {
	emptyModelCatalog(t)
	vendors := shippedVendors(t)
	spec := independentSeat(handoff.Author{CLI: "agy-tmux", Tier: "fast"}, vendors)
	if !spec("claude-tmux", "balanced") {
		t.Fatal("claude-tmux is a driver name of the claude family; its model is another vendor")
	}
	if spec("agy-tmux", "balanced") {
		t.Fatal("agy-tmux at another effort of the same model must be refused")
	}
	if !spec("agy-tmux", "deep") {
		t.Fatal("agy-tmux at a stronger tier that serves another model is independent")
	}
	if spec("agy-claude-tmux", "balanced") != spec("claude-tmux", "balanced") {
		t.Fatal("the vendor decision must not depend on the driver spelling")
	}
}

func TestHandoffJudge_NoIndependentSeatLaunchesNothingAndIsRefused(t *testing.T) {
	run := newJudgeRun(t, handoff.Author{CLI: handoff.Unattributed}, "deep", nil)
	_, err := run.do(t)
	if !errors.Is(err, llmroute.ErrNoPermittedAttempt) {
		t.Fatalf("err = %v, want a refusal", err)
	}
	if len(run.bridge.calls) != 0 {
		t.Fatalf("launched %v", run.bridge.launched())
	}
}

func TestHandoffJudge_Exit85BenchesTheSeatAndWalksToTheNextIndependentSeat(t *testing.T) {
	run := newJudgeRun(t, codexAuthor, "balanced", nil)
	run.bridge.exits["agy-tmux"] = 85
	result, err := run.do(t)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(run.bridge.launched(), " "); got != "agy-tmux@balanced claude-tmux@balanced" {
		t.Fatalf("launched %q", got)
	}
	if strings.Join(run.benched, " ") != "agy-tmux" {
		t.Fatalf("benched = %v; the walker of the cycle benches the walled seat once", run.benched)
	}
	if result.Seat != (handoff.Seat{CLI: "claude-tmux", Tier: "balanced"}) {
		t.Fatalf("seat = %+v; the record names the seat that launched", result.Seat)
	}
	if len(result.Verdicts) != 1 || !result.Verdicts[0].Agrees {
		t.Fatalf("verdicts = %+v", result.Verdicts)
	}
}

func TestHandoffJudge_TransientFaultRelaunchesInAFreshSession(t *testing.T) {
	run := newJudgeRun(t, codexAuthor, "balanced", func(b *judgeBridge) core.Bridge {
		return &transientOnce{judgeBridge: b}
	})
	if _, err := clihealth.NewStore(run.project.root, time.Now).BenchWall("agy", clihealth.QuotaExhaustedPattern, "quota"); err != nil {
		t.Fatal(err)
	}
	result, err := run.runner(context.Background(), run.call)
	if err != nil {
		t.Fatal(err)
	}
	calls := run.bridge.calls
	if len(calls) != 2 || calls[0].CLI != "claude-tmux" || calls[1].CLI != "claude-tmux" || !calls[1].FreshSession {
		t.Fatalf("calls = %+v; want one fresh-session relaunch of the primary seat", calls)
	}
	if result.Seat.CLI != "claude-tmux" {
		t.Fatalf("seat = %+v", result.Seat)
	}
}

func TestHandoffJudge_JudgeTierReachesTheLaunchedSeat(t *testing.T) {
	run := newJudgeRun(t, codexAuthor, "balanced", nil)
	if _, err := run.do(t); err != nil {
		t.Fatal(err)
	}
	if len(run.bridge.calls) == 0 || run.bridge.calls[0].Model != "balanced" {
		t.Fatalf("launched %v; the profile default is deep and judge_tier balanced must win", run.bridge.launched())
	}
	run.call.Tier = "top"
	run.bridge.calls = nil
	if _, err := run.do(t); err != nil || len(run.bridge.calls) == 0 || run.bridge.calls[0].Model != "top" {
		t.Fatalf("a judge_tier of top must reach the seat: %v %v", run.bridge.launched(), err)
	}
}

func TestHandoffJudge_TheWalkerNeverStepsBelowBalanced(t *testing.T) {
	run := newJudgeRun(t, codexAuthor, "balanced", nil)
	run.bridge.exits["agy-tmux"], run.bridge.exits["claude-tmux"] = 85, 85
	if _, err := run.do(t); err == nil {
		t.Fatal("every seat walled must be a fault")
	}
	for _, call := range run.bridge.calls {
		if call.Model == "fast" {
			t.Fatalf("launched %v; the envelope floor is balanced", run.bridge.launched())
		}
	}
	if len(run.bridge.calls) != 2 {
		t.Fatalf("launched %v; want each admitted seat once", run.bridge.launched())
	}
}

func TestHandoffJudge_EmptyWorktreeFallsBackToTheProjectRoot(t *testing.T) {
	run := newJudgeRun(t, codexAuthor, "balanced", nil)
	run.call.Input.Worktree = ""
	if _, err := run.do(t); err != nil {
		t.Fatalf("with no worktree the request is built on the project root: %v", err)
	}
	if !strings.Contains(run.bridge.calls[0].Prompt, hcCoverAsk) {
		t.Fatalf("prompt lacks the ask of the request built on the project root:\n%s", run.bridge.calls[0].Prompt)
	}
}

func TestHandoffJudgePrompt_MultiLineAskTextIsOneLine(t *testing.T) {
	t.Parallel()
	call := core.JudgeCall{Pair: "tdd-build", Asks: []string{"a1"}, Input: core.ReviewInput{Workspace: "/ws"}}
	asks := []handoff.Ask{{ID: "a1", Text: "first line\nsecond   line\r\nthird", EvidenceKind: "predicate"}}
	prompt := handoffJudgePrompt(call, asks, "/ws/handoff-judge.json")
	if !strings.Contains(prompt, "- a1: first line second line third (evidence: predicate)\n") {
		t.Fatalf("prompt:\n%s", prompt)
	}
}

func TestHandoffJudgePrompt_CarriesTheRulesOfTheJudge(t *testing.T) {
	t.Parallel()
	call := core.JudgeCall{Pair: "tdd-build", Asks: []string{"a1"}, Input: core.ReviewInput{Workspace: "/ws"}}
	prompt := handoffJudgePrompt(call, []handoff.Ask{{ID: "a1", Text: "t", EvidenceKind: "predicate"}}, "/ws/handoff-judge.json")
	for _, want := range []string{"does this evidence meet the meaning of this ask", "/ws/handoff-judge.json", "/ws/handoff-tdd-build.json", `{"verdicts":[`, "Do not edit"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt lacks %q:\n%s", want, prompt)
		}
	}
}

func TestHandoffJudgePrompt_IsBuiltFromALedgerWithNoJudgeKey(t *testing.T) {
	t.Parallel()
	call := core.JudgeCall{Pair: "tdd-build", Asks: []string{"a1"}, Input: core.ReviewInput{Workspace: "/ws"},
		Ledger: handoff.Ledger{
			Entries: []handoff.Entry{
				{Ask: "a1", Status: handoff.StatusMet, Evidence: []handoff.Evidence{{Kind: "predicate", Ref: "TestX"}}, Note: "real note"},
				{Ask: "a2", Status: handoff.StatusMet, Evidence: []handoff.Evidence{{Kind: "predicate", Ref: "OTHER-ASK-EVIDENCE"}}},
			},
			Judge: &handoff.Judgement{Error: "FORGED-JUDGE-MARKER"},
		}}
	prompt := handoffJudgePrompt(call, []handoff.Ask{{ID: "a1", Text: "t", EvidenceKind: "predicate"}}, "/ws/j.json")
	if strings.Contains(prompt, "FORGED-JUDGE-MARKER") {
		t.Fatalf("a judge key of the ledger must never reach the judge:\n%s", prompt)
	}
	if strings.Contains(prompt, "OTHER-ASK-EVIDENCE") {
		t.Fatalf("the evidence of an ask that is not judged must not reach the prompt:\n%s", prompt)
	}
	if !strings.Contains(prompt, "TestX") || !strings.Contains(prompt, "real note") {
		t.Fatalf("the entries of the ledger are the evidence to judge:\n%s", prompt)
	}
}

func TestHandoffJudgeRunner_APlantedStaleFileIsNotReused(t *testing.T) {
	t.Run("a planted file is cleared before the launch", func(t *testing.T) {
		run := newJudgeRun(t, codexAuthor, "balanced", nil)
		run.bridge.reply = ""
		planted := filepath.Join(run.call.Input.Workspace, "handoff-judge.json")
		if err := os.MkdirAll(filepath.Dir(planted), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(planted, []byte(judgeReply()), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := run.do(t); !errors.Is(err, errJudgeNoVerdictFile) {
			t.Fatalf("err = %v; a seat that wrote no file is a fault, and a planted file is never the verdict", err)
		}
	})
	t.Run("a file older than the launch is refused", func(t *testing.T) {
		run := newJudgeRun(t, codexAuthor, "balanced", nil)
		old := time.Now().Add(-time.Hour)
		run.bridge.touch = func(path string) {
			if err := os.Chtimes(path, old, old); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := run.do(t); !errors.Is(err, errJudgeNoVerdictFile) {
			t.Fatalf("err = %v; a stale file must be refused", err)
		}
	})
}

func TestHandoffJudgeRunner_FaultsAreErrorsWithTheBasisKept(t *testing.T) {
	for name, reply := range map[string]string{
		"prose":       "agree",
		"missing ask": `{"verdicts":[]}`,
		"unknown key": `{"verdicts":[{"ask":"` + hcCoverAsk + `","agrees":true,"note":"","x":1}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			run := newJudgeRun(t, codexAuthor, "balanced", nil)
			run.bridge.reply = reply
			result, err := run.do(t)
			if err == nil {
				t.Fatal("a bad reply is an error")
			}
			if errors.Is(err, llmroute.ErrNoPermittedAttempt) {
				t.Fatalf("a bad reply is no refusal: %v", err)
			}
			if result.Basis.AuthorModel == "" || result.Basis.TierCeiling != "balanced" {
				t.Fatalf("basis = %+v", result.Basis)
			}
		})
	}
}

func TestHandoffJudgeRunner_TheResultRecordsTheBasisOfTheFilter(t *testing.T) {
	run := newJudgeRun(t, codexAuthor, "balanced", nil)
	result, err := run.do(t)
	if err != nil {
		t.Fatal(err)
	}
	b := result.Basis
	if b.AuthorModel == "" || b.SeatModel == "" || b.AuthorModel == b.SeatModel || len(b.AuthorVendors) == 0 || len(b.SeatVendors) == 0 || b.TierCeiling != "balanced" {
		t.Fatalf("basis = %+v", b)
	}
	if strings.Join(b.AuthorVendors, ",") != "gpt" || strings.Join(b.SeatVendors, ",") != "gemini" {
		t.Fatalf("vendors = %v / %v", b.AuthorVendors, b.SeatVendors)
	}
}

func TestHandoffJudgeRunner_ARequestThatCannotBuildIsAnError(t *testing.T) {
	run := newJudgeRun(t, codexAuthor, "balanced", nil)
	run.call.Pair = "no-such-pair"
	if _, err := run.do(t); err == nil || len(run.bridge.calls) != 0 {
		t.Fatalf("an unknown pair has no request and launches nothing: %v", err)
	}
}

func TestHandoffJudgeRunner_AtI8ShadowAValidFileRaisesNoDisagreement(t *testing.T) {
	emptyModelCatalog(t)
	center := signalcenter.New()
	run := newJudgeRun(t, codexAuthor, "balanced", nil)
	run.bridge.echo = true
	run.project.write(".evolve/policy.json", `{"isolation":{"i8":{"stage":"shadow"}}}`)
	walked := bridgechain.New(run.bridge, nil)
	run.runner = handoffJudgeRunner(judgeDeps{walked: walked, router: shippedJudgeRouter(t), vendors: shippedVendors(t), signals: center})
	if _, err := run.do(t); err != nil {
		t.Fatal(err)
	}
	if events := center.Recent(); len(events) != 0 {
		t.Fatalf("events = %+v; a valid file that the stdout repeats must not raise flow.shadow.disagree", events)
	}
}

func TestHandoffJudge_ProfileAndRoutingAgree(t *testing.T) {
	t.Parallel()
	root := checkedInRoot(t)
	pol, err := policy.Load(filepath.Join(root, ".evolve", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	route, ok := pol.CLIRouting.Agents[handoffJudgeAgent]
	if !ok || route.Model != "" || strings.Join(route.CLI, ",") != "agy,claude" {
		t.Fatalf("routing entry = %+v (present=%v); want cli agy,claude and no model", route, ok)
	}
	prof, err := profiles.NewFromDir(routingProfilesDir(root)).Get(handoffJudgeAgent)
	if err != nil {
		t.Fatal(err)
	}
	var drivers []string
	for _, family := range route.CLI {
		drivers = append(drivers, llmroute.DefaultDriverForFamily(family))
	}
	if prof.CLI != drivers[0] || strings.Join(prof.CLIFallback, ",") != strings.Join(drivers[1:], ",") || strings.Join(prof.AllowedCLIs, ",") != strings.Join(route.CLI, ",") {
		t.Fatalf("profile cli=%q fallback=%v allowed=%v disagree with routing %v (drivers %v)", prof.CLI, prof.CLIFallback, prof.AllowedCLIs, route.CLI, drivers)
	}
	if prof.ModelTierEnvelope == nil || prof.ModelTierEnvelope.Min != "balanced" || prof.ModelTierEnvelope.Max != "top" {
		t.Fatalf("envelope = %+v", prof.ModelTierEnvelope)
	}
	decision, err := shippedJudgeRouter(t).Resolve(cliroute.Request{Agent: handoffJudgeAgent, ProjectRoot: root, DefaultModel: "deep"})
	if err != nil {
		t.Fatal(err)
	}
	for _, cli := range decision.Plan.Candidates {
		if llmroute.Family(cli) == "agy-claude" {
			t.Fatalf("plan %v holds agy-claude; the profile must keep the after-chain tail out", decision.Plan.Candidates)
		}
	}
	if len(decision.Plan.Candidates) != 2 {
		t.Fatalf("plan = %v, want agy then claude", decision.Plan.Candidates)
	}
}

func TestHandoffJudge_IsWiredIntoTheCycleReviewer(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	if !wireOrchestratorDeps(root, evolveDir, io.Discard, routingRun{}).Orchestrator.HandoffJudgeWired() {
		t.Fatal("the cycle reviewer chain must hold the handoff judge")
	}
}

func TestWireHandoffJudge_ReturnsAWiredCopy(t *testing.T) {
	t.Parallel()
	base := core.NewHandoffReviewer(policy.Policy{}.HandoffConfig(), nil)
	wired := wireHandoffJudge(base, judgeDeps{walked: bridgechain.New(&judgeBridge{}, nil)})
	if !wired.JudgeWired() || base.JudgeWired() {
		t.Fatal("wireHandoffJudge returns a reviewer that holds the runner")
	}
}

func TestHandoffShow_PrintsTheJudgeSeatOutcomeAndBasis(t *testing.T) {
	t.Parallel()
	p := newHCProject(t)
	req := p.cycle1950()
	p.ledger(1950, req, hcMetPredicate(hcCoverAsk), hcMetPredicate(hcGuardAsk))
	ledgerPath := filepath.Join(p.runDir(1950), "handoff-tdd-build.json")
	raw, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	judge := `"judge":{"seat":{"cli":"agy-tmux","tier":"balanced"},"verdicts":[{"ask":"` + hcCoverAsk + `","agrees":false,"note":"n"}],` +
		`"basis":{"author_model":"sonnet","seat_model":"Gemini 4 Hydrogen","author_vendors":["claude"],"seat_vendors":["gemini"],"tier_ceiling":"balanced"}},`
	body := strings.Replace(string(raw), `{"schema_version"`, `{`+judge+`"schema_version"`, 1)
	if err := os.WriteFile(ledgerPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, stdout, stderr := runHC(t, "show", "--cycle", "1950", "--project-root", p.root)
	if rc != 0 {
		t.Fatalf("exit = %d\n%s\n%s", rc, stdout, stderr)
	}
	for _, want := range []string{"tdd-build:", "judge: agy-tmux@balanced outcome=disagree", "author_model=sonnet", "seat_model=Gemini 4 Hydrogen", "author_vendors=claude", "seat_vendors=gemini", "tier_ceiling=balanced"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout)
		}
	}
	rc, stdout, _ = runHC(t, "show", "--cycle", "1950", "--project-root", p.root, "--json")
	if rc != 0 || !strings.Contains(stdout, `"judge":{"seat":{"cli":"agy-tmux","tier":"balanced"}`) || !strings.Contains(stdout, `"outcome":"disagree"`) {
		t.Errorf("json:\n%s", stdout)
	}
}

func TestHandoffShow_AMissingJudgeKeyPrintsNoJudgeLine(t *testing.T) {
	t.Parallel()
	p := blockedProject1950(t)
	_, stdout, _ := runHC(t, "show", "--cycle", "1950", "--project-root", p.root)
	if strings.Contains(stdout, "judge") {
		t.Fatalf("stdout names a judge that did not run:\n%s", stdout)
	}
}

func TestHandoffJudgeRunner_AFileThatCannotBeClearedIsAnErrorAndLaunchesNothing(t *testing.T) {
	run := newJudgeRun(t, codexAuthor, "balanced", nil)
	blocker := filepath.Join(run.call.Input.Workspace, "handoff-judge.json", "keep")
	if err := os.MkdirAll(filepath.Dir(blocker), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := run.do(t); err == nil || len(run.bridge.calls) != 0 {
		t.Fatalf("err = %v launched %v; a decision file that cannot be cleared stops the judge before the launch", err, run.bridge.launched())
	}
}

func TestHandoffJudgeRunner_ASeatThatExitsNonZeroWithNoErrorIsAFault(t *testing.T) {
	run := newJudgeRun(t, codexAuthor, "balanced", nil)
	run.bridge.quiet = map[string]int{"agy-tmux": 7}
	result, err := run.do(t)
	if !errors.Is(err, errJudgeSeatExit) {
		t.Fatalf("err = %v", err)
	}
	if result.Seat.CLI != "agy-tmux" || result.Verdicts != nil {
		t.Fatalf("result = %+v; the seat that ran is recorded and no verdict is kept", result)
	}
}
