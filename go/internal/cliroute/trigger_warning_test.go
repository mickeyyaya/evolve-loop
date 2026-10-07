package cliroute_test

import (
	"slices"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func TestCompile_WarnsWhenAProfilesTriggerListOmitsTheModelMismatchExit(t *testing.T) {
	profs := profiles.NewFromFS(fstest.MapFS{
		"narrow.json":  {Data: []byte(`{"name":"narrow","cli":"claude-tmux","cli_fallback_on_exit":[80,81]}`)},
		"covered.json": {Data: []byte(`{"name":"covered","cli":"claude-tmux","cli_fallback_on_exit":[80,87]}`)},
		"default.json": {Data: []byte(`{"name":"default","cli":"claude-tmux"}`)},
		"single.json":  {Data: []byte(`{"name":"single","cli":"claude-tmux","cli_fallback_on_exit":[85]}`)},
	})
	for name, pol := range map[string]policy.Policy{
		"legacy projection": {},
		"declared table":    routingPolicy(policy.CLIRouting{CLIs: []string{"claude"}, Default: []string{"claude"}}),
	} {
		_, findings := cliroute.Compile(pol, nil, profs)

		var warned []string
		for _, f := range findings {
			if f.Severity == cliroute.SeverityWarn && f.Key != "cli_routing.default" {
				warned = append(warned, f.Key)
			}
		}
		slices.Sort(warned)
		if want := []string{"profiles.narrow.cli_fallback_on_exit", "profiles.single.cli_fallback_on_exit"}; !slices.Equal(warned, want) {
			t.Errorf("%s: warnings %v, want %v: every own trigger list without 87 warns, a one-code list too (findings %+v)", name, warned, want, findings)
		}
	}
}
