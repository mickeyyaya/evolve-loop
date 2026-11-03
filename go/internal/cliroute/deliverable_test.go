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

func cloudHostsOfTheTest(driver string) ([]string, bool) {
	if driver == "claude-cloud" {
		return []string{"github.com"}, true
	}
	return nil, false
}

func reportRouter(t *testing.T, pol policy.Policy, prof profiles.Profile, judged bool) *cliroute.Router {
	t.Helper()
	table, findings := cliroute.Compile(pol, nil, cliroute.SingleProfile{Agent: prof.Name, Profile: &prof})
	if errs := errorFindings(findings); len(errs) > 0 {
		t.Fatalf("Compile reported errors: %+v", errs)
	}
	host := cliroute.Host{LookPath: everyBinary()}
	if judged {
		host.CloudHosts = cloudHostsOfTheTest
	}
	return mustRouter(t, table, host)
}

func reportRequest(mutate func(*cliroute.Request)) cliroute.Request {
	req := worktreeRequest(func(r *cliroute.Request) { r.NeedsTools = false })
	if mutate != nil {
		mutate(&req)
	}
	return req
}

func TestResolve_AReportPhaseDropsACloudSeatInBothPaths(t *testing.T) {
	multi := policy.CLIRouting{CLIs: []string{"claude"}, AfterChain: "stop"}
	for name, pol := range map[string]policy.Policy{"legacy": {}, "declared": routingPolicy(multi)} {
		t.Run(name, func(t *testing.T) {
			r := reportRouter(t, pol, builderProfile("claude-cloud", "claude-tmux"), true)

			d := mustResolve(t, r, reportRequest(nil))

			if want := []string{"claude-tmux"}; !reflect.DeepEqual(d.Plan.Candidates, want) {
				t.Fatalf("chain = %v, want %v", d.Plan.Candidates, want)
			}
			if !traceMentions(d.Trace, "[claude-cloud]", "pull request") {
				t.Fatalf("trace = %q, want a line naming the dropped cloud seat", d.Trace)
			}
		})
	}
}

func TestResolve_AWorktreePhaseKeepsACloudSeat(t *testing.T) {
	r := reportRouter(t, policy.Policy{}, builderProfile("claude-cloud", "claude-tmux"), true)

	d := mustResolve(t, r, worktreeRequest(nil))

	if want := []string{"claude-cloud", "claude-tmux"}; !reflect.DeepEqual(d.Plan.Candidates, want) {
		t.Fatalf("chain = %v, want %v", d.Plan.Candidates, want)
	}
}

func TestResolve_AReportPhaseWithOnlyACloudSeatIsRefusedWithTheHostsOfTheData(t *testing.T) {
	multi := policy.CLIRouting{CLIs: []string{"claude"}, AfterChain: "stop"}
	for name, pol := range map[string]policy.Policy{"legacy": {}, "declared": routingPolicy(multi)} {
		t.Run(name, func(t *testing.T) {
			r := reportRouter(t, pol, builderProfile("claude-cloud"), true)

			_, err := r.Resolve(reportRequest(nil))

			var dErr *cliroute.DeliverableError
			if !errors.Is(err, cliroute.ErrRefused) || !errors.As(err, &dErr) {
				t.Fatalf("err = %v, want a refused route carrying a DeliverableError", err)
			}
			if !reflect.DeepEqual(dErr.Dropped, []string{"claude-cloud"}) || !reflect.DeepEqual(dErr.Hosts, []string{"github.com"}) {
				t.Fatalf("DeliverableError = %+v, want claude-cloud dropped and the hosts of the data", dErr)
			}
		})
	}
}

func TestResolve_AHostThatJudgesNoCloudSeatLeavesAReportChainWhole(t *testing.T) {
	r := reportRouter(t, policy.Policy{}, builderProfile("claude-cloud", "claude-tmux"), false)

	d := mustResolve(t, r, reportRequest(nil))

	if want := []string{"claude-cloud", "claude-tmux"}; !reflect.DeepEqual(d.Plan.Candidates, want) {
		t.Fatalf("chain = %v, want %v", d.Plan.Candidates, want)
	}
}

func TestResolve_AReportChainWithNoCloudSeatNeverTracesTheDeliverableFilter(t *testing.T) {
	r := reportRouter(t, policy.Policy{}, builderProfile("claude-tmux"), true)

	d := mustResolve(t, r, reportRequest(nil))

	if len(d.Trace) != 0 {
		t.Fatalf("trace = %q, want none", d.Trace)
	}
}

func TestDeliverableError_NamesTheRuleTheDroppedSeatsAndTheHosts(t *testing.T) {
	err := &cliroute.DeliverableError{Rule: "agents:auditor", Dropped: []string{"claude-cloud"}, Hosts: []string{"github.com", "example.org"}}

	got := err.Error()
	for _, want := range []string{"agents:auditor", "claude-cloud", "github.com", "example.org", "pull request"} {
		if !strings.Contains(got, want) {
			t.Fatalf("Error() = %q, want it to hold %q", got, want)
		}
	}
	if llmroute.Family("claude-cloud") == "" {
		t.Fatal("the cloud seat has no family")
	}
}
