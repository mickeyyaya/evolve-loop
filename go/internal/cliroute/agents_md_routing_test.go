package cliroute_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

var (
	errAgentsMdSectionMissing = errors.New("AGENTS.md has no routing bullet that starts with `- Since 2026-10-07 the `cli_routing` table decides every dispatch:`")
	errAgentsMdListMissing    = errors.New("the routing bullets of AGENTS.md do not state a chain list")
	errAgentsMdListDrift      = errors.New("a chain list in AGENTS.md differs from the policy")
	errAgentsMdStaleFirst     = errors.New("AGENTS.md says that agy runs first, but the policy default does not start with agy")
	errAgentsMdUnknownModel   = errors.New("AGENTS.md names a Gemini model that the agy manifest and the policy do not define")
	errAgentsMdShareMissing   = errors.New("AGENTS.md does not state the share_pct of the deep tier")
)

const agentsMdRoutingHeading = "- Since 2026-10-07 the `cli_routing` table decides every dispatch:"

var (
	geminiModelPattern = regexp.MustCompile(`Gemini \d+(?:\.\d+)? [A-Z][a-z]+`)
	agyFirstPattern    = regexp.MustCompile(`(?i)agy[- ]first`)
)

func agentsMdRoutingBlock(doc string) (string, error) {
	_, rest, found := strings.Cut(doc, agentsMdRoutingHeading+"\n")
	if !found {
		return "", errAgentsMdSectionMissing
	}
	var block []string
	for _, line := range strings.Split(rest, "\n") {
		if !strings.HasPrefix(line, " ") {
			break
		}
		block = append(block, line)
	}
	return strings.Join(block, "\n"), nil
}

func statedList(block, key string) ([]string, bool) {
	pattern := regexp.MustCompile("`" + regexp.QuoteMeta(key) + "` (?:is|are) `\\[([^\\]]*)\\]`")
	m := pattern.FindStringSubmatch(block)
	if m == nil {
		return nil, false
	}
	var out []string
	for _, item := range strings.Split(m[1], ",") {
		out = append(out, strings.TrimSpace(item))
	}
	return out, true
}

func policyChainsByKey(rt policy.CLIRouting) map[string][]string {
	return map[string][]string{
		"clis":           rt.CLIs,
		"default":        rt.Default,
		"tiers.deep":     rt.Tiers["deep"].CLIs,
		"tiers.top":      rt.Tiers["top"].CLIs,
		"agents.builder": rt.Agents["builder"].CLI,
	}
}

func checkAgentsMdChains(block string, rt policy.CLIRouting) []error {
	var errs []error
	chains := policyChainsByKey(rt)
	for _, key := range slices.Sorted(maps.Keys(chains)) {
		got, stated := statedList(block, key)
		if !stated {
			errs = append(errs, fmt.Errorf("%w: no `%s` is `[...]` statement", errAgentsMdListMissing, key))
			continue
		}
		if want := chains[key]; !slices.Equal(got, want) {
			errs = append(errs, fmt.Errorf("%w: %s: AGENTS.md %v, policy %v", errAgentsMdListDrift, key, got, want))
		}
	}
	return errs
}

func checkAgentsMdFirst(block string, rt policy.CLIRouting) []error {
	if len(rt.Default) > 0 && rt.Default[0] != "agy" && agyFirstPattern.MatchString(block) {
		return []error{fmt.Errorf("%w: default starts with %s", errAgentsMdStaleFirst, rt.Default[0])}
	}
	return nil
}

func checkAgentsMdModels(block string, known map[string]bool) []error {
	var errs []error
	for _, name := range geminiModelPattern.FindAllString(block, -1) {
		if !known[name] {
			errs = append(errs, fmt.Errorf("%w: %q", errAgentsMdUnknownModel, name))
		}
	}
	return errs
}

func checkAgentsMdShare(block string, rt policy.CLIRouting) []error {
	share := rt.Tiers["deep"].SharePct
	if share == 0 {
		return nil
	}
	if !regexp.MustCompile(fmt.Sprintf("`share_pct`[^\n]*\\b%d\\b", share)).MatchString(block) {
		return []error{fmt.Errorf("%w: policy share_pct of deep is %d", errAgentsMdShareMissing, share)}
	}
	return nil
}

func checkAgentsMdRouting(doc string, rt policy.CLIRouting, knownModels map[string]bool) []error {
	block, err := agentsMdRoutingBlock(doc)
	if err != nil {
		return []error{err}
	}
	var errs []error
	errs = append(errs, checkAgentsMdChains(block, rt)...)
	errs = append(errs, checkAgentsMdFirst(block, rt)...)
	errs = append(errs, checkAgentsMdModels(block, knownModels)...)
	errs = append(errs, checkAgentsMdShare(block, rt)...)
	return errs
}

func modelFamilyName(model string) string {
	name, _, _ := strings.Cut(model, " (")
	return name
}

func agyModelNames(t *testing.T) map[string]bool {
	t.Helper()
	var manifest struct {
		ModelTierMap map[string]string `json:"model_tier_map"`
	}
	if err := json.Unmarshal([]byte(repoFile(t, "go", "internal", "bridge", "manifests", "agy-tmux.json")), &manifest); err != nil {
		t.Fatal(err)
	}
	p, err := policy.Load(filepath.Join("..", "..", "..", ".evolve", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, model := range manifest.ModelTierMap {
		names[modelFamilyName(model)] = true
	}
	if p.Catalog != nil {
		for _, model := range p.Catalog.TierPins["agy"] {
			names[modelFamilyName(model)] = true
		}
	}
	if len(names) == 0 {
		t.Fatal("the agy manifest and the policy define no model")
	}
	return names
}

func TestAgentsMdRoutingMatchesPolicy(t *testing.T) {
	doc := repoFile(t, "AGENTS.md")
	for _, err := range checkAgentsMdRouting(doc, checkedInRouting(t), agyModelNames(t)) {
		t.Error(err)
	}
}

func TestAgentsMdRoutingDriftIsDetected(t *testing.T) {
	rt := policy.CLIRouting{
		CLIs:    []string{"agy", "agy-claude", "claude"},
		Default: []string{"claude", "agy", "agy-claude"},
		Agents:  map[string]policy.AgentRule{"builder": {CLI: []string{"agy", "agy-claude", "claude"}}},
		Tiers: map[string]policy.TierRule{
			"deep": {CLIs: []string{"claude", "agy", "agy-claude"}, SharePct: 50},
			"top":  {CLIs: []string{"agy-claude", "claude"}},
		},
	}
	known := map[string]bool{"Gemini 4 Hydrogen": true, "Gemini 4 Argon": true}
	block := func(clis, def, deep, top, builder, extra string) string {
		return "# T\n\n" + agentsMdRoutingHeading + "\n" +
			"  - `clis` is `[" + clis + "]`, `default` is `[" + def + "]`, and `agents.builder` is `[" + builder + "]`.\n" +
			"  - `tiers.deep` is `[" + deep + "]` with `share_pct` 50. `tiers.top` is `[" + top + "]`.\n" +
			"  - Gemini 4 Hydrogen runs at balanced.\n" + extra + "- Next top-level bullet.\n"
	}
	good := block("agy, agy-claude, claude", "claude, agy, agy-claude", "claude, agy, agy-claude", "agy-claude, claude", "agy, agy-claude, claude", "")
	cases := []struct {
		name string
		doc  string
		want error
	}{
		{"a document in step with the policy", good, nil},
		{"the stale default order", strings.Replace(good, "`default` is `[claude, agy, agy-claude]`", "`default` is `[agy, agy-claude, claude]`", 1), errAgentsMdListDrift},
		{"the stale deep order", strings.Replace(good, "`tiers.deep` is `[claude, agy, agy-claude]`", "`tiers.deep` is `[agy, agy-claude, claude]`", 1), errAgentsMdListDrift},
		{"a builder chain that the policy does not have", strings.Replace(good, "`agents.builder` is `[agy, agy-claude, claude]`", "`agents.builder` is `[claude]`", 1), errAgentsMdListDrift},
		{"a merged statement of clis and default", strings.Replace(good, "`clis` is `[agy, agy-claude, claude]`, `default` is", "`clis` and `default` are `[agy, agy-claude, claude]`, `default` is", 1), errAgentsMdListMissing},
		{"no routing bullet", "# T\n\nnothing here\n", errAgentsMdSectionMissing},
		{"the agy runs first statement", block("agy, agy-claude, claude", "claude, agy, agy-claude", "claude, agy, agy-claude", "agy-claude, claude", "agy, agy-claude, claude", "  - Every phase runs agy first.\n"), errAgentsMdStaleFirst},
		{"a Gemini model that no manifest defines", block("agy, agy-claude, claude", "claude, agy, agy-claude", "claude, agy, agy-claude", "agy-claude, claude", "agy, agy-claude, claude", "  - Gemini 3.8 Flash High runs at balanced.\n"), errAgentsMdUnknownModel},
		{"a document with no share statement", strings.Replace(good, " with `share_pct` 50", "", 1), errAgentsMdShareMissing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := checkAgentsMdRouting(tc.doc, rt, known)
			if tc.want == nil {
				if len(errs) != 0 {
					t.Fatalf("want no drift, got %v", errs)
				}
				return
			}
			if !slices.ContainsFunc(errs, func(err error) bool { return errors.Is(err, tc.want) }) {
				t.Fatalf("want %v, got %v", tc.want, errs)
			}
		})
	}
}
