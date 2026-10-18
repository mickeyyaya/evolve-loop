package cliroute_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func ollamaIsToolless(driver string) bool { return driver == "ollama-tmux" }

func toolUseRouter(t *testing.T, pol policy.Policy, prof profiles.Profile) *cliroute.Router {
	t.Helper()
	table, findings := cliroute.Compile(pol, nil, cliroute.SingleProfile{Agent: prof.Name, Profile: &prof})
	if errs := errorFindings(findings); len(errs) > 0 {
		t.Fatalf("Compile reported errors: %+v", errs)
	}
	return mustRouter(t, table, cliroute.Host{LookPath: everyBinary(), Toolless: ollamaIsToolless})
}

func builderProfile(cli string, fallback ...string) profiles.Profile {
	return profiles.Profile{Name: "builder", CLI: cli, CLIFallback: fallback, ModelTierDefault: "balanced"}
}

func worktreeRequest(mutate func(*cliroute.Request)) cliroute.Request {
	req := cliroute.Request{Agent: "builder", Phase: "build", DefaultModel: "balanced", NeedsTools: true}
	if mutate != nil {
		mutate(&req)
	}
	return req
}

type toolUseCase struct {
	name string
	pol  policy.Policy
	prof profiles.Profile
	req  cliroute.Request
	want []string
}

func runToolUseCases(t *testing.T, cases []toolUseCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := mustResolve(t, toolUseRouter(t, tc.pol, tc.prof), tc.req)
			if !reflect.DeepEqual(d.Plan.Candidates, tc.want) {
				t.Fatalf("chain = %v, want %v (trace %v)", d.Plan.Candidates, tc.want, d.Trace)
			}
		})
	}
}

func TestResolve_ALegacyWorktreePhaseDropsAToollessCLIWhoeverNamedIt(t *testing.T) {
	runToolUseCases(t, []toolUseCase{
		{"the profile cli", policy.Policy{}, builderProfile("ollama-tmux", "claude-tmux"), worktreeRequest(nil), []string{"claude-tmux"}},
		{"the profile cli_fallback", policy.Policy{}, builderProfile("claude-tmux", "ollama-tmux", "agy-tmux"), worktreeRequest(nil), []string{"claude-tmux", "agy-tmux"}},
		{"an EVOLVE_BUILDER_CLI override", policy.Policy{}, builderProfile("claude-tmux", "agy-tmux"),
			worktreeRequest(func(r *cliroute.Request) { r.Env = map[string]string{"EVOLVE_BUILDER_CLI": "ollama-tmux"} }), []string{"agy-tmux"}},
		{"an EVOLVE_CLI override", policy.Policy{}, builderProfile("claude-tmux", "agy-tmux"),
			worktreeRequest(func(r *cliroute.Request) { r.Env = map[string]string{"EVOLVE_CLI": "ollama-tmux"} }), []string{"agy-tmux"}},
		{"the advisor overlay", policy.Policy{}, builderProfile("claude-tmux", "agy-tmux"),
			worktreeRequest(func(r *cliroute.Request) { r.Overlay = llmroute.Overlay{CLI: "ollama"} }), []string{"claude-tmux", "agy-tmux"}},
		{"a policy pin", policy.Policy{Pins: map[string]policy.Pin{"build": {CLI: "ollama"}}}, builderProfile("claude-tmux", "agy-tmux"), worktreeRequest(nil), []string{"agy-tmux"}},
		{"the caller cli", policy.Policy{}, builderProfile("claude-tmux", "agy-tmux"),
			worktreeRequest(func(r *cliroute.Request) { r.Phase, r.CallerCLI = "", "ollama-tmux" }), []string{"claude-tmux", "agy-tmux"}},
	})
}

func TestResolve_APhaseThatNeedsNoToolsKeepsAToollessCLI(t *testing.T) {
	runToolUseCases(t, []toolUseCase{
		{"a phase with no worktree", policy.Policy{}, builderProfile("ollama-tmux", "claude-tmux"),
			worktreeRequest(func(r *cliroute.Request) { r.NeedsTools = false }), []string{"ollama-tmux", "claude-tmux"}},
	})
}

func TestResolve_AHostThatFlagsNothingLeavesTheChainWholeAndSaysSoInTheTrace(t *testing.T) {
	multi := policy.CLIRouting{CLIs: []string{"claude", "ollama"}, AfterChain: "stop"}
	for name, pol := range map[string]policy.Policy{"legacy": {}, "declared": routingPolicy(multi)} {
		t.Run(name, func(t *testing.T) {
			prof := builderProfile("ollama-tmux", "claude-tmux")
			table, _ := cliroute.Compile(pol, nil, cliroute.SingleProfile{Agent: "builder", Profile: &prof})
			r := mustRouter(t, table, cliroute.Host{LookPath: everyBinary()})

			d := mustResolve(t, r, worktreeRequest(nil))

			if want := []string{"ollama-tmux", "claude-tmux"}; !reflect.DeepEqual(d.Plan.Candidates, want) {
				t.Fatalf("a host that flags nothing drops nothing: %v, want %v", d.Plan.Candidates, want)
			}
			if !traceMentions(d.Trace, "tool use", "not filtered") {
				t.Fatalf("trace = %q, want a line that says the worktree chain is not filtered", d.Trace)
			}
		})
	}
}

func TestResolve_APhaseThatNeedsNoToolsNeverTracesTheToolUseFilter(t *testing.T) {
	prof := builderProfile("ollama-tmux", "claude-tmux")
	table, _ := cliroute.Compile(policy.Policy{}, nil, cliroute.SingleProfile{Agent: "builder", Profile: &prof})
	r := mustRouter(t, table, cliroute.Host{LookPath: everyBinary()})

	d := mustResolve(t, r, worktreeRequest(func(req *cliroute.Request) { req.NeedsTools = false }))

	if len(d.Trace) != 0 {
		t.Fatalf("trace = %q, want none for a phase that runs no tools", d.Trace)
	}
}

func TestResolve_ADeclaredWorktreePhaseDropsAToollessCLIInAMultiFamilyTable(t *testing.T) {
	multi := policy.CLIRouting{CLIs: []string{"agy", "claude", "ollama"}, AfterChain: "stop"}
	withDefault := multi
	withDefault.Default = []string{"ollama", "agy", "claude"}
	runToolUseCases(t, []toolUseCase{
		{"the profile rule", routingPolicy(multi), builderProfile("ollama-tmux", "agy-tmux", "claude-tmux"), worktreeRequest(nil), []string{"agy-tmux", "claude-tmux"}},
		{"the default rule", routingPolicy(withDefault), builderProfile("claude-tmux"), worktreeRequest(nil), []string{"agy-tmux", "claude-tmux"}},
		{"an env override", routingPolicy(multi), builderProfile("claude-tmux", "agy-tmux"),
			worktreeRequest(func(r *cliroute.Request) { r.Env = map[string]string{"EVOLVE_BUILDER_CLI": "ollama-tmux"} }), []string{"claude-tmux", "agy-tmux"}},
		{"the advisor overlay", routingPolicy(multi), builderProfile("claude-tmux", "agy-tmux"),
			worktreeRequest(func(r *cliroute.Request) { r.Overlay = llmroute.Overlay{CLI: "ollama"} }), []string{"claude-tmux", "agy-tmux"}},
		{"the caller cli", routingPolicy(multi), builderProfile("claude-tmux", "agy-tmux"),
			worktreeRequest(func(r *cliroute.Request) { r.Phase, r.CallerCLI = "", "ollama-tmux" }), []string{"claude-tmux", "agy-tmux"}},
		{"the after_chain tail", routingPolicy(policy.CLIRouting{CLIs: []string{"claude", "ollama"}, Default: []string{"claude"}}), builderProfile("claude-tmux"), worktreeRequest(nil), []string{"claude-tmux"}},
	})
}

func TestResolve_TheAllowedSetOfTheClaudeOnlyTableKeepsAToollessCLIOutOfAWorktreeChain(t *testing.T) {
	profileRule := policy.CLIRouting{CLIs: []string{"claude"}}
	runToolUseCases(t, []toolUseCase{
		{"the profile rule", routingPolicy(profileRule), builderProfile("ollama-tmux", "claude-tmux"), worktreeRequest(nil), []string{"claude-tmux"}},
		{"the advisor overlay", routingPolicy(profileRule), builderProfile("claude-tmux"),
			worktreeRequest(func(r *cliroute.Request) { r.Overlay = llmroute.Overlay{CLI: "ollama"} }), []string{"claude-tmux"}},
	})
}

func TestResolve_AWorktreeChainLeftWithNoToolCapableCLIFailsLoudlyNamingTheDroppedOnes(t *testing.T) {
	declared := policy.CLIRouting{CLIs: []string{"claude", "ollama"}, Default: []string{"ollama"}, AfterChain: "stop"}
	cases := []struct {
		name string
		pol  policy.Policy
		prof profiles.Profile
		rule string
	}{
		{"legacy", policy.Policy{}, builderProfile("ollama-tmux"), "legacy:profile"},
		{"declared", routingPolicy(declared), builderProfile("claude-tmux"), "default"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := toolUseRouter(t, tc.pol, tc.prof).Resolve(worktreeRequest(nil))

			var toolErr *cliroute.ToolUseError
			if !errors.Is(err, cliroute.ErrRefused) || !errors.As(err, &toolErr) {
				t.Fatalf("err = %v, want a refused route carrying a ToolUseError", err)
			}
			if toolErr.Rule != tc.rule || !reflect.DeepEqual(toolErr.Dropped, []string{"ollama-tmux"}) {
				t.Fatalf("ToolUseError = %+v, want rule %s dropping [ollama-tmux]", toolErr, tc.rule)
			}
			for _, part := range []string{"builder", "ollama-tmux", "tool use"} {
				if !strings.Contains(err.Error(), part) {
					t.Fatalf("err = %q, want it to mention %q", err, part)
				}
			}
		})
	}
}

func TestToolUseError_NamesTheRuleAndEveryDroppedCLI(t *testing.T) {
	err := &cliroute.ToolUseError{Rule: "agents:builder", Dropped: []string{"ollama-tmux", "ollama"}}

	if got := err.Error(); !strings.Contains(got, "agents:builder") || !strings.Contains(got, "ollama-tmux") || !strings.Contains(got, "ollama]") {
		t.Fatalf("Error() = %q, want the rule and each dropped CLI", got)
	}
}

func TestResolve_ATraceNamesTheCLIsTheToolUseFilterDropped(t *testing.T) {
	multi := policy.CLIRouting{CLIs: []string{"agy", "claude", "ollama"}, AfterChain: "stop"}
	for name, pol := range map[string]policy.Policy{"legacy": {}, "declared": routingPolicy(multi)} {
		t.Run(name, func(t *testing.T) {
			d := mustResolve(t, toolUseRouter(t, pol, builderProfile("ollama-tmux", "claude-tmux")), worktreeRequest(nil))

			if !traceMentions(d.Trace, "[ollama-tmux]", "no tool use") {
				t.Fatalf("trace = %q, want a line naming the dropped CLI", d.Trace)
			}
		})
	}
}

func traceMentions(trace []string, parts ...string) bool {
	for _, line := range trace {
		if mentionsAll(line, parts) {
			return true
		}
	}
	return false
}
