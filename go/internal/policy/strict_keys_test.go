package policy_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestPolicyParse_RefusesAnUnknownKey(t *testing.T) {
	_, err := policy.Parse([]byte(`{"fleet": {"cuont": 3}}`))

	if !errors.Is(err, policy.ErrUnknownKey) {
		t.Fatalf("a misspelled key under fleet must fail the parse with ErrUnknownKey, got %v", err)
	}
	if !strings.Contains(err.Error(), `"fleet.cuont"`) {
		t.Errorf("the error names the full key path: %v", err)
	}
}

func TestPolicyParse_NamesTheFullPathOfANestedUnknownKey(t *testing.T) {
	cases := map[string]struct {
		raw  string
		path string
	}{
		"top level":            {`{"fleeet": {}}`, `"fleeet"`},
		"map value":            {`{"pins": {"scout": {"clii": "claude"}}}`, `"pins.scout.clii"`},
		"array element":        {`{"floor": [{"id": "a"}, {"idd": "b"}]}`, `"floor[1].idd"`},
		"custom unmarshaler":   {`{"retry": {"phase_max_atempts": 2}}`, `"retry.phase_max_atempts"`},
		"routing agent object": {`{"cli_routing": {"agents": {"builder": {"cli": ["claude"], "modle": "fast"}}}}`, `"cli_routing.agents.builder.modle"`},
		"handoff pair":         {`{"handoff": {"build-audit": {"stagee": "shadow"}}}`, `"handoff.build-audit.stagee"`},
		"quality index":        {`{"quality": {"indexes": {"q1": {"stag": "shadow"}}}}`, `"quality.indexes.q1.stag"`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := policy.Parse([]byte(tc.raw))

			if err == nil || !strings.Contains(err.Error(), tc.path) {
				t.Fatalf("want an error naming %s, got %v", tc.path, err)
			}
		})
	}
}

func TestPolicyParse_KeepsTheValueShapesOfKnownKeys(t *testing.T) {
	raw := `{
		"cli_routing": {"agents": {"scout": ["claude"]}, "tiers": {"top": ["claude"]}},
		"handoff": {"build-audit": {"stage": "enforce"}},
		"mandatory_phases": ["scout", null],
		"pins": null
	}`

	pol, err := policy.Parse([]byte(raw))

	if err != nil {
		t.Fatalf("known keys in every accepted shape parse: %v", err)
	}
	if pol.CLIRouting == nil || len(pol.CLIRouting.Agents["scout"].CLI) != 1 {
		t.Errorf("the array form of an agent rule still decodes: %+v", pol.CLIRouting)
	}
}

func TestPolicyParse_RefusesMalformedJSONBeforeTheKeyWalk(t *testing.T) {
	if _, err := policy.Parse([]byte(`{"fleet": `)); err == nil {
		t.Fatal("malformed JSON is an error")
	}
}

func TestPolicyParse_TheTrackedPolicyAndEveryFixtureDecodeStrictly(t *testing.T) {
	files := []string{filepath.Join("..", "..", "..", ".evolve", "policy.json")}
	fixtures, err := filepath.Glob(filepath.Join("testdata", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range append(files, fixtures...) {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if _, err := policy.Parse(raw); err != nil {
			t.Errorf("%s: %v", path, err)
		}
	}
}

func TestLoadOrDefaults_WarnsAndAppliesTheCompiledDefaultsOnARefusedPolicy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(`{"fleet": {"cuont": 3, "count": 9}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var warn strings.Builder

	pol := policy.LoadOrDefaults(path, &warn)

	if pol.Fleet != nil {
		t.Errorf("a refused policy applies no block, got fleet %+v", pol.Fleet)
	}
	if !strings.Contains(warn.String(), `"fleet.cuont"`) || !strings.Contains(warn.String(), "compiled defaults") {
		t.Errorf("the refusal is never silent: %q", warn.String())
	}
}

func TestLoadOrDefaults_IsSilentForAValidPolicy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(`{"fleet": {"count": 3}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var warn strings.Builder

	pol := policy.LoadOrDefaults(path, &warn)

	if pol.Fleet == nil || warn.Len() != 0 {
		t.Errorf("a valid policy loads with no warning: fleet %+v warn %q", pol.Fleet, warn.String())
	}
}

func TestPolicyParse_AValueOfTheWrongShapeFailsInItsOwnDecoderNotAsAnUnknownKey(t *testing.T) {
	cases := map[string]struct {
		raw  string
		want string
	}{
		"an object for a number":    {`{"fleet": {"count": {"n": 4}}}`, "count"},
		"a tier rule field":         {`{"cli_routing": {"tiers": {"deep": {"clis": 5}}}}`, "tier rule"},
		"a convergence field":       {`{"workflow": {"convergence": {"max_fix_rounds": "two"}}}`, "convergence"},
		"an inbox factor":           {`{"inbox_priority": {"factors": {"base": "high"}}}`, "factors"},
		"a code index field":        {`{"code_index": {"backends": "trail"}}`, "code_index"},
		"a practice watch field":    {`{"practice_watch": {"timeout_s": "slow"}}`, "practice_watch"},
		"a quality index list":      {`{"quality": {"indexes": ["PQ1"]}}`, "quality"},
		"a handoff pair that is 5":  {`{"handoff": {"build-audit": 5}}`, "handoff"},
		"an agent rule cli of 5":    {`{"cli_routing": {"agents": {"scout": {"cli": 5}}}}`, "agent rule"},
		"a retry field of the text": {`{"retry": {"phase_max_attempts": "x"}}`, "phase_max_attempts"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := policy.Parse([]byte(tc.raw))

			if err == nil || errors.Is(err, policy.ErrUnknownKey) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want a decode error naming %q that is not ErrUnknownKey, got %v", tc.want, err)
			}
		})
	}
}

func TestPolicyParse_MatchesTheKeyCaseExactly(t *testing.T) {
	_, err := policy.Parse([]byte(`{"Fleet": {"count": 3}}`))

	if !errors.Is(err, policy.ErrUnknownKey) || !strings.Contains(err.Error(), `"Fleet"`) {
		t.Fatalf("encoding/json folds the case of a key; the strict walk does not: %v", err)
	}
}

func TestLoadOrDefaults_IsSilentForAnAbsentFile(t *testing.T) {
	var warn strings.Builder

	policy.LoadOrDefaults(filepath.Join(t.TempDir(), "policy.json"), &warn)

	if warn.Len() != 0 {
		t.Fatalf("an absent policy is the compiled defaults with no warning: %q", warn.String())
	}
}
