package cliroute_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute/cliroutetest"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func TestMain(m *testing.M) {
	os.Unsetenv("EVOLVE_CLI")
	os.Unsetenv("EVOLVE_CLI_HEALTH")
	os.Exit(m.Run())
}

type fakeCatalog map[string]phasespec.PhaseSpec

func (c fakeCatalog) Get(name string) (phasespec.PhaseSpec, bool) {
	s, ok := c[name]
	return s, ok
}

func (c fakeCatalog) Names() []string {
	names := make([]string, 0, len(c))
	for n := range c {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func syntheticCatalog() fakeCatalog {
	return fakeCatalog{
		"scout": {Name: "scout"},
		"tdd":   {Name: "tdd"},
		"build": {Name: "build"},
		"audit": {Name: "audit"},
		"memo":  {Name: "memo"},
	}
}

const syntheticProfilesJSON = `{
 "auditor": {"name":"auditor","cli":"claude-tmux","cli_fallback":["claude-p"],"allowed_clis":["claude"],"model_tier_default":"deep","model_tier_envelope":{"min":"deep","max":"deep"},"cross_family_with":"builder"},
 "builder": {"name":"builder","cli":"codex-tmux","cli_fallback":["claude-tmux"],"allowed_clis":["claude","codex","agy"],"model_tier_default":"balanced","model_tier_envelope":{"min":"balanced","max":"deep"},"model_tier_overrides":{"m_complex_5plus_files":"deep"},"cross_family_with":"auditor"},
 "tdd-engineer": {"name":"tdd-engineer","cli":"claude-tmux","cli_fallback":["claude-p"],"allowed_clis":["claude"],"model_tier_default":"balanced","cross_family_with":"builder"},
 "scout": {"name":"scout","cli":"codex-tmux","cli_fallback":["claude-tmux"],"allowed_clis":["all"],"model_tier_default":"balanced"},
 "memo": {"name":"memo","cli":"agy-tmux","cli_fallback":["claude-tmux"],"model_tier_default":"fast"},
 "spec-verify": {"name":"spec-verify","cli":"claude-tmux","model_tier_overrides":{"hard_case":"top"}},
 "scanner": {"name":"scanner","cli":"agy-tmux","cli_fallback":["claude-tmux"],"model_tier_default":"fast","model_tier_overrides":{"hard_case":"deep"}}
}`

func syntheticProfiles(t *testing.T) *profiles.Loader {
	t.Helper()
	var docs map[string]map[string]any
	if err := json.Unmarshal([]byte(syntheticProfilesJSON), &docs); err != nil {
		t.Fatal(err)
	}
	fsys := fstest.MapFS{}
	for name, doc := range docs {
		raw, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		fsys[name+".json"] = &fstest.MapFile{Data: raw}
	}
	return profiles.NewFromFS(fsys)
}

func routingPolicy(block policy.CLIRouting) policy.Policy {
	return policy.Policy{CLIRouting: &block}
}

func installed(bins ...string) func(string) (string, error) {
	return func(bin string) (string, error) {
		if slices.Contains(bins, bin) {
			return "/fake/bin/" + bin, nil
		}
		return "", errors.New("not installed")
	}
}

func everyBinary() func(string) (string, error) {
	return installed("claude", "codex", "agy", "ollama")
}

func mustRouter(t *testing.T, table cliroute.Table, host cliroute.Host) *cliroute.Router {
	t.Helper()
	r, err := cliroute.New(table, host)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return r
}

func mustCompile(t *testing.T, p policy.Policy, cat cliroute.Catalog, profs cliroute.ProfileSource) cliroute.Table {
	t.Helper()
	table, findings := cliroute.Compile(p, cat, profs)
	if errs := errorFindings(findings); len(errs) > 0 {
		t.Fatalf("Compile reported errors: %+v", errs)
	}
	return table
}

func errorFindings(findings []cliroute.Finding) []cliroute.Finding {
	var out []cliroute.Finding
	for _, f := range findings {
		if f.Severity == cliroute.SeverityError {
			out = append(out, f)
		}
	}
	return out
}

func findingFor(findings []cliroute.Finding, key string, severity cliroute.Severity, mentions ...string) (cliroute.Finding, bool) {
	for _, f := range findings {
		if f.Key == key && f.Severity == severity && mentionsAll(f.Message, mentions) {
			return f, true
		}
	}
	return cliroute.Finding{}, false
}

func mentionsAll(message string, mentions []string) bool {
	for _, m := range mentions {
		if !strings.Contains(message, m) {
			return false
		}
	}
	return true
}

func requireFinding(t *testing.T, findings []cliroute.Finding, key string, severity cliroute.Severity, mentions ...string) {
	t.Helper()
	if _, ok := findingFor(findings, key, severity, mentions...); !ok {
		t.Fatalf("no %s finding for %s mentioning %q in %+v", severity, key, mentions, findings)
	}
}

func families(chain []string) []string {
	out := make([]string, 0, len(chain))
	for _, c := range chain {
		out = append(out, llmroute.Family(c))
	}
	return out
}

func walkWith(plan llmroute.Plan, exits map[string]int) llmroute.TieredDispatchResult {
	return llmroute.DispatchTiered(plan, func(cli, tier string) (int, error) {
		code, scripted := exits[cli+"@"+tier]
		if !scripted || code == 0 {
			return 0, nil
		}
		return code, fmt.Errorf("exit %d", code)
	}, nil)
}

func walkEveryAttemptWalled(plan llmroute.Plan) []string {
	return llmroute.DispatchTiered(plan, func(cli, tier string) (int, error) {
		return 85, errors.New("walled")
	}, nil).Attempts
}

func realBench(root, phase string, plan llmroute.Plan, env map[string]string) llmroute.Plan {
	return bridgechain.ApplyCLIHealthBench(root, phase, plan, env, cliroutetest.Now, nil)
}

func realCatalog(t *testing.T) phasespec.Catalog {
	t.Helper()
	cat, _, _, err := phasespec.MergedCatalog(cliroutetest.RepoRoot(t))
	if err != nil {
		t.Fatalf("merged catalog: %v", err)
	}
	return cat
}

func operatorTable() policy.Policy {
	return routingPolicy(policy.CLIRouting{
		CLIs:    []string{"agy", "agy-claude", "claude"},
		Default: []string{"agy", "agy-claude", "claude"},
		Tiers:   map[string]policy.TierRule{"deep": {CLIs: []string{"agy-claude", "claude"}}, "top": {CLIs: []string{"agy-claude", "claude"}}},
	})
}
