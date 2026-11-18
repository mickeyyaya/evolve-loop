package bridgechain_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

func refuseFamily(family string) bridgechain.CandidateSpec {
	return func(cli, _ string) bool { return llmroute.Family(cli) != family }
}

func admitAll(string, string) bool { return true }

func TestCandidateFilterExports_AreNamed(t *testing.T) {
	var spec bridgechain.CandidateSpec = admitAll
	var opt bridgechain.Option = bridgechain.WithCandidateFilter(spec)
	w := bridgechain.New(&scripted{}, fixedPlan([]string{"codex-tmux"}, []string{"deep"}), opt)
	seat, _, err := w.LaunchSeat(context.Background(), core.BridgeRequest{CLI: "codex-tmux", Model: "deep"})
	if err != nil {
		t.Fatalf("LaunchSeat: %v", err)
	}
	var named bridgechain.Seat = seat
	if named.CLI != "codex-tmux" || named.Tier != "deep" {
		t.Fatalf("Seat = %+v", named)
	}
}

func TestWalking_CandidateFilterSkipsARefusedSeatAndLaunchesTheNext(t *testing.T) {
	inner := &scripted{}
	w := bridgechain.New(inner, fixedPlan([]string{"claude-tmux", "codex-tmux"}, []string{"deep"}),
		bridgechain.WithCandidateFilter(refuseFamily("claude")))
	seat, resp, err := w.LaunchSeat(context.Background(), core.BridgeRequest{CLI: "claude-tmux", Model: "deep", Agent: "handoff-judge"})
	if err != nil || resp.ExitCode != 0 {
		t.Fatalf("resp=%+v err=%v", resp, err)
	}
	if got := strings.Join(attempts(inner.calls), " "); got != "codex-tmux@deep" {
		t.Fatalf("a refused seat must never launch; attempts = %q", got)
	}
	if seat != (bridgechain.Seat{CLI: "codex-tmux", Tier: "deep"}) {
		t.Fatalf("seat = %+v", seat)
	}
}

func TestWalking_CandidateFilterThatRefusesEverySeatLaunchesNothingAndErrors(t *testing.T) {
	inner := &scripted{}
	w := bridgechain.New(inner, fixedPlan([]string{"claude-tmux", "codex-tmux"}, []string{"deep"}),
		bridgechain.WithCandidateFilter(func(string, string) bool { return false }))
	_, _, err := w.LaunchSeat(context.Background(), core.BridgeRequest{CLI: "claude-tmux", Model: "deep", Agent: "handoff-judge"})
	if !errors.Is(err, llmroute.ErrNoPermittedAttempt) {
		t.Fatalf("err = %v, want it to wrap llmroute.ErrNoPermittedAttempt", err)
	}
	if len(inner.calls) != 0 {
		t.Fatalf("a filter that refuses every seat must launch nothing, not fall back to the caller's CLI: %v", attempts(inner.calls))
	}
}

func TestWalking_CandidateFilterAlsoRefusesTheCallersOwnCLIWhenThePlanIsEmpty(t *testing.T) {
	inner := &scripted{}
	w := bridgechain.New(inner, fixedPlan(nil, nil), bridgechain.WithCandidateFilter(refuseFamily("claude")))
	_, _, err := w.LaunchSeat(context.Background(), core.BridgeRequest{CLI: "claude-tmux", Model: "deep", Agent: "handoff-judge"})
	if err == nil {
		t.Fatal("the caller's own refused CLI is the default seat of an empty plan and must be refused")
	}
	if len(inner.calls) != 0 {
		t.Fatalf("launched %v", attempts(inner.calls))
	}
}

func TestWalking_FilteredWalkStillBenchesEachQuotaWall(t *testing.T) {
	inner := &scripted{exits: map[string]int{"codex-tmux": 85, "agy-tmux": 85}}
	var benched []string
	w := bridgechain.New(inner, fixedPlan([]string{"claude-tmux", "codex-tmux", "agy-tmux"}, []string{"deep"}),
		bridgechain.WithCandidateFilter(refuseFamily("claude")),
		bridgechain.WithBench(func(e bridgechain.Escalation) { benched = append(benched, e.CLI) }))
	_, _, _ = w.LaunchSeat(context.Background(), core.BridgeRequest{CLI: "claude-tmux", Model: "deep", Agent: "handoff-judge"})
	if got := strings.Join(benched, " "); got != "codex-tmux agy-tmux" {
		t.Fatalf("benched = %q, want each admitted seat that hit the wall and no refused seat", got)
	}
}

func TestWalking_FilteredWalkHandsTheBridgeEachAttemptAsAChainAttempt(t *testing.T) {
	inner := &scripted{}
	w := bridgechain.New(inner, fixedPlan([]string{"claude-tmux", "codex-tmux"}, []string{"deep"}),
		bridgechain.WithCandidateFilter(refuseFamily("claude")))
	if _, _, err := w.LaunchSeat(context.Background(), core.BridgeRequest{CLI: "claude-tmux", Model: "deep", Agent: "handoff-judge"}); err != nil {
		t.Fatal(err)
	}
	for _, c := range inner.calls {
		if !c.ChainAttempt {
			t.Fatalf("a walked attempt carries ChainAttempt so the inner bridge does not walk again: %+v", c)
		}
	}
}

func TestWalking_LaunchSeatReportsTheServedFallbackSeatNotThePlanHead(t *testing.T) {
	cases := []struct {
		name  string
		opts  []bridgechain.Option
		exits map[string]int
		tiers []string
		want  bridgechain.Seat
	}{
		{"cli fallback without a filter", nil, map[string]int{"codex-tmux": 81}, []string{"deep"}, bridgechain.Seat{CLI: "claude-tmux", Tier: "deep"}},
		{"cli fallback under a permissive filter", []bridgechain.Option{bridgechain.WithCandidateFilter(admitAll)}, map[string]int{"codex-tmux": 81}, []string{"deep"}, bridgechain.Seat{CLI: "claude-tmux", Tier: "deep"}},
		{"tier step-down after every cli walls", nil, map[string]int{"codex-tmux@deep": 85, "claude-tmux@deep": 85}, []string{"deep", "balanced"}, bridgechain.Seat{CLI: "codex-tmux", Tier: "balanced"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			inner := &scripted{exits: c.exits}
			w := bridgechain.New(inner, fixedPlan([]string{"codex-tmux", "claude-tmux"}, c.tiers), c.opts...)
			seat, _, err := w.LaunchSeat(context.Background(), core.BridgeRequest{CLI: "codex-tmux", Model: c.tiers[0], Agent: "handoff-judge"})
			if err != nil {
				t.Fatal(err)
			}
			if seat != c.want {
				t.Fatalf("seat = %+v, want %+v (attempts %v)", seat, c.want, attempts(inner.calls))
			}
		})
	}
}

func TestWalking_CandidateFilterIsEvaluatedPerTier(t *testing.T) {
	inner := &scripted{exits: map[string]int{"codex-tmux@deep": 85}}
	atBalancedOnly := func(cli, tier string) bool { return llmroute.Family(cli) != "claude" || tier == "balanced" }
	w := bridgechain.New(inner, fixedPlan([]string{"claude-tmux", "codex-tmux"}, []string{"deep", "balanced"}),
		bridgechain.WithCandidateFilter(atBalancedOnly))
	seat, _, err := w.LaunchSeat(context.Background(), core.BridgeRequest{CLI: "claude-tmux", Model: "deep", Agent: "handoff-judge"})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(attempts(inner.calls), " "); got != "codex-tmux@deep claude-tmux@balanced" {
		t.Fatalf("attempts = %q", got)
	}
	if seat != (bridgechain.Seat{CLI: "claude-tmux", Tier: "balanced"}) {
		t.Fatalf("seat = %+v", seat)
	}
}

func TestWalking_CandidateFilterReadsATierAliasTheWayThePlanCeilingDoes(t *testing.T) {
	inner := &scripted{}
	w := bridgechain.New(inner, fixedPlan([]string{"claude-tmux", "codex-tmux"}, []string{"opus"}),
		bridgechain.WithCandidateFilter(refuseFamily("claude")))
	if _, _, err := w.LaunchSeat(context.Background(), core.BridgeRequest{CLI: "claude-tmux", Model: "opus", Agent: "handoff-judge"}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(attempts(inner.calls), " "); got != "codex-tmux@opus" {
		t.Fatalf("a plan tier written as an alias must be filtered like its canonical tier; attempts = %q", got)
	}
}

func TestWalking_CandidateFilterDoesNotMutateTheResolvedPlansCeiling(t *testing.T) {
	shared := map[string][]string{"deep": {"claude", "codex"}}
	snapshot := map[string][]string{"deep": {"claude", "codex"}}
	resolve := func(core.BridgeRequest) (llmroute.Plan, error) {
		return llmroute.Plan{Candidates: []string{"claude-tmux", "codex-tmux"}, Triggers: llmroute.DefaultTriggers(), Tiers: []string{"deep"}, TierCeiling: shared}, nil
	}
	filtered := bridgechain.New(&scripted{}, resolve, bridgechain.WithCandidateFilter(refuseFamily("claude")))
	if _, _, err := filtered.LaunchSeat(context.Background(), core.BridgeRequest{CLI: "claude-tmux", Model: "deep", Agent: "handoff-judge"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(shared, snapshot) {
		t.Fatalf("the resolved plan's ceiling map was mutated: %v", shared)
	}
	inner := &scripted{}
	plain := bridgechain.New(inner, resolve)
	if _, err := plain.Launch(context.Background(), core.BridgeRequest{CLI: "claude-tmux", Model: "deep", Agent: "builder"}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(attempts(inner.calls), " "); got != "claude-tmux@deep" {
		t.Fatalf("an unfiltered walk of the same plan must still reach claude first; attempts = %q", got)
	}
}

func TestWalking_CandidateFilterNeverWidensThePlansOwnCeiling(t *testing.T) {
	resolve := func(core.BridgeRequest) (llmroute.Plan, error) {
		return llmroute.Plan{Candidates: []string{"claude-tmux", "codex-tmux"}, Triggers: llmroute.DefaultTriggers(), Tiers: []string{"deep"},
			TierCeiling: map[string][]string{"deep": {"codex"}}}, nil
	}
	inner := &scripted{}
	w := bridgechain.New(inner, resolve, bridgechain.WithCandidateFilter(admitAll))
	seat, _, err := w.LaunchSeat(context.Background(), core.BridgeRequest{CLI: "claude-tmux", Model: "deep", Agent: "handoff-judge"})
	if err != nil {
		t.Fatal(err)
	}
	if seat.CLI != "codex-tmux" || len(inner.calls) != 1 {
		t.Fatalf("seat=%+v attempts=%v: the filter may only narrow what the plan's ceiling permits", seat, attempts(inner.calls))
	}
}

func TestWalking_ChainAttemptCannotBypassTheCandidateFilter(t *testing.T) {
	inner := &scripted{}
	w := bridgechain.New(inner, fixedPlan([]string{"codex-tmux"}, []string{"deep"}), bridgechain.WithCandidateFilter(refuseFamily("claude")))
	req := core.BridgeRequest{CLI: "claude-tmux", Model: "deep", Agent: "handoff-judge", ChainAttempt: true}
	if _, _, err := w.LaunchSeat(context.Background(), req); err == nil {
		t.Fatal("a ChainAttempt request on a refused seat must be refused")
	}
	if _, err := w.Launch(context.Background(), req); err == nil {
		t.Fatal("Launch must apply the same refusal")
	}
	if len(inner.calls) != 0 {
		t.Fatalf("launched %v", attempts(inner.calls))
	}
}

func TestWalking_ChainAttemptOnAnAdmittedSeatPassesThroughAsThatSeat(t *testing.T) {
	inner := &scripted{}
	w := bridgechain.New(inner, fixedPlan([]string{"claude-tmux"}, []string{"deep"}), bridgechain.WithCandidateFilter(refuseFamily("claude")))
	seat, _, err := w.LaunchSeat(context.Background(), core.BridgeRequest{CLI: "codex-tmux", Model: "deep", ChainAttempt: true})
	if err != nil {
		t.Fatal(err)
	}
	if seat != (bridgechain.Seat{CLI: "codex-tmux", Tier: "deep"}) || len(inner.calls) != 1 {
		t.Fatalf("seat=%+v attempts=%v", seat, attempts(inner.calls))
	}
}

func TestWalking_LaunchHonoursTheCandidateFilterOnTheCoreBridgeEntry(t *testing.T) {
	inner := &scripted{}
	var bridge core.Bridge = bridgechain.New(inner, fixedPlan([]string{"claude-tmux", "codex-tmux"}, []string{"deep"}),
		bridgechain.WithCandidateFilter(refuseFamily("claude")))
	if _, err := bridge.Launch(context.Background(), core.BridgeRequest{CLI: "claude-tmux", Model: "deep", Agent: "handoff-judge"}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(attempts(inner.calls), " "); got != "codex-tmux@deep" {
		t.Fatalf("attempts = %q", got)
	}
}

func TestWalking_IndependentsFilterLeavesTheJudgeOnlyAnotherFamilyOrAStrongerTier(t *testing.T) {
	author := handoff.Author{CLI: "claude-tmux", Model: "opus", Tier: "deep"}
	cases := []struct {
		name  string
		clis  []string
		tiers []string
		want  string
	}{
		{"another family serves", []string{"claude-tmux", "codex-tmux"}, []string{"deep"}, "codex-tmux@deep"},
		{"same family at a stronger tier serves", []string{"claude-tmux"}, []string{"top"}, "claude-tmux@top"},
		{"same family steps past its own tier to a stronger one", []string{"claude-tmux"}, []string{"deep", "top"}, "claude-tmux@top"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			inner := &scripted{}
			w := bridgechain.New(inner, fixedPlan(c.clis, c.tiers), bridgechain.WithCandidateFilter(handoff.Independents(author)))
			if _, _, err := w.LaunchSeat(context.Background(), core.BridgeRequest{CLI: "claude-tmux", Model: c.tiers[0], Agent: "handoff-judge"}); err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(attempts(inner.calls), " "); got != c.want {
				t.Fatalf("attempts = %q, want %q", got, c.want)
			}
		})
	}
}

func TestWalking_IndependentsFilterRefusesTheAuthorsOwnModelAndLaunchesNothing(t *testing.T) {
	inner := &scripted{}
	w := bridgechain.New(inner, fixedPlan([]string{"claude-tmux", "claude-p"}, []string{"deep"}),
		bridgechain.WithCandidateFilter(handoff.Independents(handoff.Author{CLI: "claude-tmux", Tier: "deep"})))
	_, _, err := w.LaunchSeat(context.Background(), core.BridgeRequest{CLI: "claude-tmux", Model: "deep", Agent: "handoff-judge"})
	if !errors.Is(err, llmroute.ErrNoPermittedAttempt) {
		t.Fatalf("err = %v", err)
	}
	if len(inner.calls) != 0 {
		t.Fatalf("a judge must never be seated on the model of its author: %v", attempts(inner.calls))
	}
}
