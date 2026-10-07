package cliroute_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestCompile_ATableWithNoDefaultWarnsThatEveryProfileLessLaunchIsUnreachable(t *testing.T) {
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Work: map[string][]string{"build": {"agy", "claude"}}}

	_, findings := cliroute.Compile(routingPolicy(block), syntheticCatalog(), syntheticProfiles(t))

	requireFinding(t, findings, "cli_routing.default", cliroute.SeverityWarn, "no profile", cliroute.ClassifierAgent)
}

func TestCompile_ADefaultReachesEveryProfileLessLaunch(t *testing.T) {
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}}

	_, findings := cliroute.Compile(routingPolicy(block), syntheticCatalog(), syntheticProfiles(t))

	if f, found := findingFor(findings, "cli_routing.default", cliroute.SeverityWarn); found {
		t.Errorf("a declared default reaches every profile-less launch; got %+v", f)
	}
}

func TestCompile_NoBlockNeedsNoDefault(t *testing.T) {
	_, findings := cliroute.Compile(policy.Policy{}, syntheticCatalog(), syntheticProfiles(t))

	if f, found := findingFor(findings, "cli_routing.default", cliroute.SeverityWarn); found {
		t.Errorf("the legacy projection keeps its own profile-less routing; got %+v", f)
	}
}
