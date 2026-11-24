package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/agentcmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/heldaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/heldproposal"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestSeatOutcomeOf_AFailVerdictIsAnOutcomeAndASeatFailureIsAnError(t *testing.T) {
	t.Parallel()
	down := errors.New("exit 21")
	fail := agentcmd.Envelope{Verdict: core.VerdictFAIL, Report: "findings", CLI: "agy-tmux", Model: "deep", RunLabel: "r"}

	out, err := seatOutcomeOf(fail, down)
	if err != nil || out.Verdict != core.VerdictFAIL || out.Report != "findings" || out.CLI != "agy-tmux" || out.Tier != "deep" {
		t.Errorf("FAIL verdict = %+v, %v, want an outcome that keeps the findings", out, err)
	}
	for name, env := range map[string]agentcmd.Envelope{
		"a dispatch error":   {Verdict: "", Error: "bridge down"},
		"a FAIL with error":  {Verdict: core.VerdictFAIL, Error: "no report"},
		"a non-FAIL failure": {Verdict: core.VerdictPASS},
	} {
		if _, err := seatOutcomeOf(env, down); !errors.Is(err, down) {
			t.Errorf("%s: err = %v, want the seat failure", name, err)
		}
	}
	if out, err := seatOutcomeOf(agentcmd.Envelope{Verdict: core.VerdictPASS}, nil); err != nil || out.Verdict != core.VerdictPASS {
		t.Errorf("clean run = %+v, %v", out, err)
	}
}

func TestHeldAuditPort_BothShipCompositionRootsWireTheSameHelper(t *testing.T) {
	t.Parallel()
	for _, file := range []string{"cmd_cycle.go", "cmd_ship.go"} {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), "HeldAudit:") || !strings.Contains(string(raw), "HeldAuditPlan:") || !strings.Contains(string(raw), "heldAuditPlanPort(") || !strings.Contains(string(raw), "heldAuditPort(") {
			t.Errorf("%s does not set the held-audit port: a ship through it would hold with no audit", file)
		}
	}
}

func TestHeldAuditPort_ARealCompositionWaitsForAHoldWithNoBuilderRecordAndLaunchesNothing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	rec := heldproposal.Record{Cycle: 5, Status: heldproposal.StatusHeld, Tree: "t", DiffSHA256: "d"}
	if err := heldproposal.WriteRecord(evolveDir, rec); err != nil {
		t.Fatal(err)
	}

	err := heldAuditPort(root, io.Discard)(context.Background(), evolveDir, filepath.Join(root, "ws"), rec)

	report, _, found, rerr := heldproposal.ReadAuditReport(evolveDir, 5)
	if err != nil || rerr != nil || !found || report.Verdict != heldproposal.AuditWait || !strings.HasPrefix(report.Reason, heldproposal.WaitBuilderUnattributed) {
		t.Errorf("report = %+v found %v err %v %v, want a WAIT builder_unattributed", report, found, err, rerr)
	}
}

const heldAuditPolicy = `{"cli_routing": {"clis": ["agy", "agy-claude", "claude"], "default": ["claude", "agy", "agy-claude"],
"floors": {"claude": ["adversarial-review"]},
"agents": {"held-proposal-audit": {"cli": ["claude", "agy"], "model": "deep"}}}}` + "\n"

func heldAuditProject(t *testing.T, policyBody string) string {
	t.Helper()
	repo := repoRootForSkills(t)
	root := t.TempDir()
	for _, rel := range []string{
		"docs/architecture/phase-registry.json",
		".evolve/phases/adversarial-review/phase.json",
		".evolve/profiles/adversarial-review.json",
		"agents/evolve-adversarial-review.md",
	} {
		copyFileForTest(t, filepath.Join(repo, filepath.FromSlash(rel)), filepath.Join(root, filepath.FromSlash(rel)))
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(policyBody), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func checkTheHeldAuditSeatRunsAdversarialReviewOnTheHeldRoute(t *testing.T) {
	t.Helper()
	root := heldAuditProject(t, heldAuditPolicy)
	bridge := &reviewerBridge{report: "## Threat Model\nA diff.\n## Findings\nNone.\n## Verdict\nPASS\n"}
	base := func() agentcmd.Deps {
		return agentcmd.Deps{
			InstallRouter: rootRouterInstaller,
			Bridge:        func(string, *signalcenter.Center) core.Bridge { return bridge },
			Signals:       quietSignals, TreeState: fixedTreeState("tree"),
			Getenv: func(string) string { return "" },
			Now:    func() time.Time { return time.Date(2026, 11, 24, 9, 0, 0, 0, time.UTC) },
		}
	}
	notClaude := func(cli, tier string) bool { return !isClaudeFamilyDriver(cli) && tier == "deep" }
	diff := filepath.Join(root, "audit.diff")
	if err := os.WriteFile(diff, []byte("diff --git a/x b/x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	req := heldaudit.SeatRequest{Cycle: 5, Input: "audit this", Files: []string{diff}, Route: cliroute.HeldAuditKey, Admit: notClaude}

	out, err := heldAuditSeatWith(root, io.Discard, base)(context.Background(), req)

	if err != nil || out.Verdict != core.VerdictPASS || isClaudeFamilyDriver(out.CLI) || out.CLI == "" {
		t.Fatalf("outcome %+v err %v, want a PASS on a non-Claude driver: the held route walks past the claude-first chain", out, err)
	}
	if len(bridge.requests) != 1 || isClaudeFamilyDriver(bridge.requests[0].CLI) {
		t.Errorf("launches %v, want one launch on a non-Claude driver", bridge.requests)
	}

	req.Admit = func(string, string) bool { return false }
	if _, err := heldAuditSeatWith(root, io.Discard, base)(context.Background(), req); err == nil || len(bridge.requests) != 1 {
		t.Errorf("err %v launches %d, want a seat failure and no second launch when the admission refuses every route", err, len(bridge.requests))
	}

	decision, err := heldAuditResolver(root, heldAuditRouterOnce(root, io.Discard))(cliroute.HeldAuditKey)
	if err != nil || len(decision.Plan.Candidates) < 2 || heldAuditFloor(heldAuditRouterOnce(root, io.Discard))(cliroute.HeldAuditKey) != "" ||
		heldAuditFloor(heldAuditRouterOnce(root, io.Discard))("adversarial-review") != "claude" {
		t.Errorf("decision %+v err %v, want the held chain with no floor while adversarial-review keeps its claude floor", decision, err)
	}
	first, _ := heldAuditRouterOnce(root, io.Discard)()
	load := heldAuditRouterOnce(root, io.Discard)
	a, _ := load()
	b, _ := load()
	if a == nil || a != b || first == nil {
		t.Error("the routing table must load once for each dispatch, not once for each query")
	}
	broken := heldAuditProject(t, `{"cli_routing": {"clis": ["nope"], "default": ["nope"]}}`+"\n")
	if _, err := heldAuditResolver(broken, heldAuditRouterOnce(broken, io.Discard))(cliroute.HeldAuditKey); err == nil {
		t.Error("a refused routing table must be an error, never an empty route")
	}
	if heldAuditFloor(heldAuditRouterOnce(broken, io.Discard))(cliroute.HeldAuditKey) != "" {
		t.Error("an unreadable table has no floor to report")
	}
}

func TestHeldAuditPlanPort_APlanOverARealCompositionNamesTheWaitAndLaunchesNothing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	rec := heldproposal.Record{Cycle: 5, Status: heldproposal.StatusWouldHold, Tree: "t"}

	plan := heldAuditPlanPort(root, io.Discard)(filepath.Join(root, "ws"), rec)

	if plan.Wait != heldproposal.WaitBuilderUnattributed || plan.CLI != "" {
		t.Errorf("plan = %+v, want a builder_unattributed wait", plan)
	}
}
