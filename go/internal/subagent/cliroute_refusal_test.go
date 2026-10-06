package subagent

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
	"github.com/mickeyyaya/evolve-loop/go/internal/subagent/subagentrun"
)

func refusingResolver(string) (resolvellm.Result, error) {
	return resolvellm.Result{}, fmt.Errorf("%w: agent scout: outside the allowed set", cliroute.ErrRefused)
}

func TestLLMOf_ARefusedRouteReachesTheDispatcherAsRefused(t *testing.T) {
	_, err := llmOf(refusingResolver)("scout")
	if !errors.Is(err, subagentrun.ErrRouteRefused) || !errors.Is(err, cliroute.ErrRefused) {
		t.Fatalf("the dispatcher sees the refusal, not an ordinary resolver miss it falls back from: %v", err)
	}
	if _, err := llmOf(func(string) (resolvellm.Result, error) { return resolvellm.Result{}, errors.New("no profile") })("scout"); errors.Is(err, subagentrun.ErrRouteRefused) {
		t.Fatalf("an ordinary resolver error keeps today's profile fallback: %v", err)
	}
}

func TestValidateProfile_ARefusedRouteFails(t *testing.T) {
	opts := happyOpts(`{"role":"scout","cli":"codex"}`, "codex")
	opts.ResolveLLM = refusingResolver
	_, err := ValidateProfile(context.Background(), ValidateProfileRequest{Agent: "scout", ProfilesDir: "/p", AdaptersDir: "/a", ProjectRoot: "/r"}, opts)
	if !errors.Is(err, cliroute.ErrRefused) {
		t.Fatalf("validate-profile refuses instead of taking the profile's CLI: %v", err)
	}
}
