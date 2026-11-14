//go:build acs

package cycle2388

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	primary       = "claude-tmux"
	healthyNext   = "codex-tmux"
	benchedFamily = "agy"
	benchedNext   = "agy-tmux"
	freshAnswer   = "answered-by-the-fresh-session"
	docPath       = "docs/architecture/packages/internal-bridgechain.md"
)

type reply struct {
	exit  int
	cause string
	body  string
}

type scriptedBridge struct {
	replies map[string][]reply
	calls   []core.BridgeRequest
}

func (s *scriptedBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	nth := 0
	for _, c := range s.calls {
		if c.CLI == req.CLI {
			nth++
		}
	}
	s.calls = append(s.calls, req)
	queue := s.replies[req.CLI]
	r := reply{}
	if nth < len(queue) {
		r = queue[nth]
	} else if len(queue) > 0 {
		r = queue[len(queue)-1]
	}
	resp := core.BridgeResponse{ExitCode: r.exit, CauseCode: r.cause, Stderr: r.body}
	if r.exit != 0 {
		return resp, fmt.Errorf("bridge: exit %d", r.exit)
	}
	return resp, nil
}

func (s *scriptedBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{Version: "scripted"}, nil
}

func (s *scriptedBridge) sequence() string {
	out := make([]string, len(s.calls))
	for i, c := range s.calls {
		out[i] = c.CLI
		if c.FreshSession {
			out[i] += "(fresh)"
		}
	}
	return strings.Join(out, " ")
}

func benchedProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if _, err := clihealth.NewStore(root, time.Now).BenchWall(benchedFamily, clihealth.QuotaExhaustedPattern, "Individual quota reached"); err != nil {
		t.Fatal(err)
	}
	if _, ok := clihealth.NewStore(root, time.Now).Active()[benchedFamily]; !ok {
		t.Fatal("fixture: the agy family bench is not active")
	}
	return root
}

func fixedPlan(clis ...string) bridgechain.PlanResolver {
	return func(core.BridgeRequest) (llmroute.Plan, error) {
		return llmroute.Plan{Candidates: clis, Triggers: llmroute.DefaultTriggers(), Tiers: []string{"deep"}}, nil
	}
}

func launch(t *testing.T, inner core.Bridge, resolve bridgechain.PlanResolver, root string) (core.BridgeResponse, error) {
	t.Helper()
	w := bridgechain.New(inner, resolve)
	return w.Launch(context.Background(), core.BridgeRequest{CLI: primary, Model: "deep", Agent: "retrospective", ProjectRoot: root})
}

func TestC2388_001_ASubmitWedgedPrimaryWithOnlyBenchedFallbacksIsRetriedOnceInAFreshSessionAndTheRetryAnswerIsReturned(t *testing.T) {
	inner := &scriptedBridge{replies: map[string][]reply{
		primary:     {{exit: 81, cause: "submit_wedged"}, {exit: 0, body: freshAnswer}},
		benchedNext: {{exit: 0, body: "answered-by-the-benched-family"}},
	}}

	resp, err := launch(t, inner, fixedPlan(primary, benchedNext), benchedProject(t))

	if err != nil || resp.Stderr != freshAnswer {
		t.Fatalf("RED: resp=%+v err=%v, want the answer of the fresh session of the primary", resp, err)
	}
	if got := inner.sequence(); got != primary+" "+primary+"(fresh)" {
		t.Fatalf("RED: attempts = %q, want the primary then one fresh session of the primary and no walk to the benched family", got)
	}
	for _, c := range inner.calls {
		if !c.ChainAttempt {
			t.Errorf("RED: every attempt carries ChainAttempt: %+v", c)
		}
	}
}

func TestC2388_002_AFailingRetryIsNotRepeatedAndTheWalkContinuesToTheBenchedFamily(t *testing.T) {
	inner := &scriptedBridge{replies: map[string][]reply{
		primary:     {{exit: 81, cause: "submit_wedged"}},
		benchedNext: {{exit: 0, body: "answered-by-the-benched-family"}},
	}}

	resp, err := launch(t, inner, fixedPlan(primary, benchedNext), benchedProject(t))

	if err != nil || resp.Stderr != "answered-by-the-benched-family" {
		t.Fatalf("RED: resp=%+v err=%v, want the walk to end on the benched family after the retry failed", resp, err)
	}
	if got := inner.sequence(); got != primary+" "+primary+"(fresh) "+benchedNext {
		t.Fatalf("RED: attempts = %q, want exactly one retry of the primary before the walk moves on", got)
	}
}

func TestC2388_003_ANonTransientPrimaryFailureWalksTheChainInBenchOrderWithNoRetry(t *testing.T) {
	root := benchedProject(t)
	benchOrdered := func(core.BridgeRequest) (llmroute.Plan, error) {
		plan := llmroute.Plan{Candidates: []string{primary, benchedNext, healthyNext}, Triggers: llmroute.DefaultTriggers(), Tiers: []string{"deep"}}
		return bridgechain.ApplyCLIHealthBench(root, "retrospective", plan, time.Now, nil), nil
	}
	inner := &scriptedBridge{replies: map[string][]reply{
		primary:     {{exit: 127, cause: "missing_binary"}},
		healthyNext: {{exit: 127, cause: "missing_binary"}},
		benchedNext: {{exit: 0, body: "answered-by-the-benched-family"}},
	}}

	resp, err := launch(t, inner, benchOrdered, root)

	if err != nil || resp.Stderr != "answered-by-the-benched-family" {
		t.Fatalf("RED: resp=%+v err=%v", resp, err)
	}
	if got := inner.sequence(); got != primary+" "+healthyNext+" "+benchedNext {
		t.Fatalf("RED: attempts = %q, want the chain in bench order (healthy before benched) and no fresh session for a missing binary", got)
	}
}

func TestC2388_004_ASubmitWedgedPrimaryWithAHealthyFallbackStillWalksAtOnce(t *testing.T) {
	inner := &scriptedBridge{replies: map[string][]reply{
		primary:     {{exit: 81, cause: "submit_wedged"}},
		healthyNext: {{exit: 0, body: "answered-by-the-healthy-family"}},
	}}

	resp, err := launch(t, inner, fixedPlan(primary, healthyNext), t.TempDir())

	if err != nil || resp.Stderr != "answered-by-the-healthy-family" {
		t.Fatalf("RED: resp=%+v err=%v", resp, err)
	}
	if got := inner.sequence(); got != primary+" "+healthyNext {
		t.Fatalf("RED: attempts = %q, want the healthy fallback right after the primary, with no retry", got)
	}
}

func TestC2388_005_AnArtifactTimeoutWithNoTransientCauseWithOnlyBenchedFallbacksWalksAtOnce(t *testing.T) {
	inner := &scriptedBridge{replies: map[string][]reply{
		primary:     {{exit: 81, cause: "incomplete"}},
		benchedNext: {{exit: 0, body: "answered-by-the-benched-family"}},
	}}

	_, err := launch(t, inner, fixedPlan(primary, benchedNext), benchedProject(t))

	if err != nil {
		t.Fatal(err)
	}
	if got := inner.sequence(); got != primary+" "+benchedNext {
		t.Fatalf("RED: attempts = %q, want a cause that is not transient to walk with no retry", got)
	}
}

func TestC2388_006_ThePackageNoteRecordsTheTransientRetryRuleInSteWithNoLintFinding(t *testing.T) {
	root := acsassert.RepoRoot(t)
	words, err := stelint.LoadStandard(root)
	if err != nil {
		t.Fatal(err)
	}
	findings, err := stelint.LintFile(filepath.Join(root, docPath), stelint.Options{Words: words})
	if err != nil {
		t.Fatal(err)
	}
	var real []string
	for _, f := range findings {
		if !f.IsSkipNotice() {
			real = append(real, fmt.Sprintf("line %d %s: %s", f.Line, f.Rule, f.Excerpt))
		}
	}
	if len(real) > 0 {
		t.Errorf("RED: %d ste-lint finding(s) in %s, first: %v", len(real), docPath, real[:min(5, len(real))])
	}
}

// acs-predicate: config-check
func TestC2388_007_ThePackageNoteStatesTheRetryRuleInOneParagraph(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, docPath))
	if err != nil {
		t.Fatal(err)
	}
	needs := []string{"submit_wedged", "fresh session", "benched", "once", "bench order"}
	for _, block := range strings.Split(string(raw), "\n\n") {
		text := strings.ToLower(block)
		missing := false
		for _, need := range needs {
			if !strings.Contains(text, need) {
				missing = true
			}
		}
		if !missing {
			return
		}
	}
	t.Errorf("RED: no paragraph of %s holds all of %v; the rule (a transient primary with only benched fallbacks is retried once in a fresh session, a non-transient failure walks in bench order) is not recorded", docPath, needs)
}
