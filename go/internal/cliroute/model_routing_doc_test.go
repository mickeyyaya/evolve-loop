package cliroute_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

var (
	errNoTierTable       = errors.New("the tier table has no rows")
	errTierChainDrift    = errors.New("a tier chain in the document differs from the policy")
	errTierBulletDrift   = errors.New("a tier bullet in the document differs from the policy")
	errUnknownRoutingKey = errors.New("the document names a routing key that no struct tag defines")
	errStaleStatement    = errors.New("the document keeps a statement that the policy contradicts")
	errShareMissing      = errors.New("the document does not state the share of the tier")
)

var (
	tierRowPattern    = regexp.MustCompile("(?m)^\\| `(fast|balanced|deep|top)` \\|[^|]*\\|([^|]*)\\|")
	tierBulletPattern = regexp.MustCompile("(?m)^- A phase at the `?(deep|top)`? tier[^\n]*$")
	backtickPattern   = regexp.MustCompile("`([^`]+)`")
	snakeKeyPattern   = regexp.MustCompile(`^[a-z]+(_[a-z]+)+$`)
	staleDeepPattern  = regexp.MustCompile("not yet in the `deep` and `top` chains")
	routingFamilies   = []string{"claude", "agy", "agy-claude"}
)

func backtickTokens(text string) []string {
	var out []string
	for _, m := range backtickPattern.FindAllStringSubmatch(text, -1) {
		out = append(out, m[1])
	}
	return out
}

func familiesIn(text string) []string {
	var out []string
	for _, tok := range backtickTokens(text) {
		if slices.Contains(routingFamilies, tok) {
			out = append(out, tok)
		}
	}
	return out
}

func sectionOf(doc, heading string) string {
	_, rest, found := strings.Cut(doc, "\n## "+heading+"\n")
	if !found {
		return ""
	}
	body, _, _ := strings.Cut(rest, "\n## ")
	return body
}

func docTierChains(doc string) (map[string][]string, error) {
	chains := map[string][]string{}
	for _, m := range tierRowPattern.FindAllStringSubmatch(sectionOf(doc, "The tier policy"), -1) {
		chains[m[1]] = familiesIn(m[2])
	}
	if len(chains) == 0 {
		return nil, errNoTierTable
	}
	return chains, nil
}

func collectJSONTags(t reflect.Type, into map[string]bool, seen map[reflect.Type]bool) {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Map {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct || seen[t] {
		return
	}
	seen[t] = true
	for i := range t.NumField() {
		f := t.Field(i)
		if name, _, _ := strings.Cut(f.Tag.Get("json"), ","); name != "" && name != "-" {
			into[name] = true
		}
		collectJSONTags(f.Type, into, seen)
	}
}

func policyKeyTags() map[string]bool {
	tags := map[string]bool{}
	seen := map[reflect.Type]bool{}
	collectJSONTags(reflect.TypeOf(policy.Policy{}), tags, seen)
	collectJSONTags(reflect.TypeOf(profiles.Profile{}), tags, seen)
	return tags
}

func checkTierChains(doc string, rt policy.CLIRouting) []error {
	chains, err := docTierChains(doc)
	if err != nil {
		return []error{err}
	}
	var errs []error
	for _, tier := range []string{"deep", "top"} {
		want := rt.Tiers[tier].CLIs
		if got := chains[tier]; !slices.Equal(got, want) {
			errs = append(errs, fmt.Errorf("%w: tier %s: document %v, policy %v", errTierChainDrift, tier, got, want))
		}
	}
	return errs
}

func checkTierBullets(doc string, rt policy.CLIRouting) []error {
	seen := map[string]bool{}
	var errs []error
	for _, m := range tierBulletPattern.FindAllStringSubmatch(sectionOf(doc, "The CLI routing table"), -1) {
		tier := m[1]
		seen[tier] = true
		if got, want := familiesIn(m[0]), rt.Tiers[tier].CLIs; !slices.Equal(got, want) {
			errs = append(errs, fmt.Errorf("%w: tier %s: bullet names %v, policy %v", errTierBulletDrift, tier, got, want))
		}
	}
	for _, tier := range []string{"deep", "top"} {
		if !seen[tier] {
			errs = append(errs, fmt.Errorf("%w: no bullet that starts with `- A phase at the `%s` tier`", errTierBulletDrift, tier))
		}
	}
	return errs
}

func checkStaleStatements(doc string, rt policy.CLIRouting) []error {
	if slices.Contains(rt.Tiers["deep"].CLIs, "agy") && staleDeepPattern.MatchString(doc) {
		return []error{fmt.Errorf("%w: agy is in the deep chain, but the document says it is not yet there", errStaleStatement)}
	}
	return nil
}

func checkShare(doc string, rt policy.CLIRouting) []error {
	share := rt.Tiers["deep"].SharePct
	if share == 0 {
		return nil
	}
	pattern := regexp.MustCompile(fmt.Sprintf("`share_pct`[^\n]*\\b%d\\b", share))
	if !pattern.MatchString(sectionOf(doc, "The tier policy")) && !pattern.MatchString(sectionOf(doc, "The CLI routing table")) {
		return []error{fmt.Errorf("%w: policy share_pct of deep is %d", errShareMissing, share)}
	}
	return nil
}

func checkRoutingKeys(doc string, known map[string]bool) []error {
	var errs []error
	for _, heading := range []string{"The tier policy", "The CLI routing table"} {
		for _, tok := range backtickTokens(sectionOf(doc, heading)) {
			if snakeKeyPattern.MatchString(tok) && !known[tok] {
				errs = append(errs, fmt.Errorf("%w: `%s`", errUnknownRoutingKey, tok))
			}
		}
	}
	return errs
}

func checkModelRoutingDoc(doc string, rt policy.CLIRouting, known map[string]bool) []error {
	var errs []error
	errs = append(errs, checkTierChains(doc, rt)...)
	errs = append(errs, checkTierBullets(doc, rt)...)
	errs = append(errs, checkStaleStatements(doc, rt)...)
	errs = append(errs, checkShare(doc, rt)...)
	errs = append(errs, checkRoutingKeys(doc, known)...)
	return errs
}

func repoFile(t *testing.T, rel ...string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(append([]string{"..", "..", ".."}, rel...)...))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func checkedInRouting(t *testing.T) policy.CLIRouting {
	t.Helper()
	p, err := policy.Load(filepath.Join("..", "..", "..", ".evolve", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	if p.CLIRouting == nil {
		t.Fatal("the checked-in policy has no cli_routing table")
	}
	return *p.CLIRouting
}

func adrFile(t *testing.T, number string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join("..", "..", "..", "docs", "architecture", "adr", number+"-*.md"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("want exactly one ADR %s, got %v (err %v)", number, matches, err)
	}
	raw, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func lineWith(doc, prefix string) string {
	for _, line := range strings.Split(doc, "\n") {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	return ""
}

func TestModelRoutingDocMatchesPolicy(t *testing.T) {
	doc := repoFile(t, "docs", "reference", "model-routing.md")
	for _, err := range checkModelRoutingDoc(doc, checkedInRouting(t), policyKeyTags()) {
		t.Error(err)
	}
}

func TestModelRoutingDocDriftIsDetected(t *testing.T) {
	rt := policy.CLIRouting{Tiers: map[string]policy.TierRule{
		"deep": {CLIs: []string{"claude", "agy", "agy-claude"}, SharePct: 50},
		"top":  {CLIs: []string{"agy-claude", "claude"}},
	}}
	known := map[string]bool{"share_pct": true, "headroom_pct": true}
	table := func(deep, top string) string {
		return "# T\n\n## The tier policy\n\n| Tier | Use | Chain | Model |\n|---|---|---|---|\n" +
			"| `deep` | x | " + deep + " | m |\n| `top` | x | " + top + " | m |\n\n" +
			"`share_pct` of deep is 50.\n\n## The CLI routing table\n\n" +
			"- A phase at the `deep` tier runs `claude`, then `agy`, then `agy-claude`.\n" +
			"- A phase at the `top` tier runs `agy-claude`, then `claude`.\n"
	}
	good := table("`claude` → `agy` → `agy-claude`", "`agy-claude` → `claude`")
	cases := []struct {
		name string
		doc  string
		want error
	}{
		{"a document in step with the policy", good, nil},
		{"the stale deep chain order", table("`agy-claude` → `agy` → `claude`", "`agy-claude` → `claude`"), errTierChainDrift},
		{"the stale top chain with agy", table("`claude` → `agy` → `agy-claude`", "`agy-claude` → `agy` → `claude`"), errTierChainDrift},
		{"an empty tier table", "# T\n\n## The tier policy\n\nno table here\n", errNoTierTable},
		{"a key that no struct tag defines", good + "\nSet `fake_routing_key` to tune it.\n", errUnknownRoutingKey},
		{"a stale deep bullet", strings.Replace(good, "runs `claude`, then `agy`, then `agy-claude`", "runs `agy-claude`, then `claude`", 1), errTierBulletDrift},
		{"a merged deep and top bullet", strings.Replace(good, "- A phase at the `deep` tier", "- A phase at the deep or the top tier", 1), errTierBulletDrift},
		{"the agy not yet in deep statement", good + "\n`agy` is not yet in the `deep` and `top` chains.\n", errStaleStatement},
		{"a document with no share statement", strings.Replace(good, "`share_pct` of deep is 50.", "", 1), errShareMissing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := checkModelRoutingDoc(tc.doc, rt, known)
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

func TestADR0150AmendsADR0136(t *testing.T) {
	amends := lineWith(adrFile(t, "0150"), "- **Amends:**")
	if !strings.Contains(amends, "(0136-") {
		t.Errorf("ADR-0150 must have an `- **Amends:**` line that links ADR-0136, got %q", amends)
	}
	amendedBy := lineWith(adrFile(t, "0136"), "- **Amended by:**")
	if !strings.Contains(amendedBy, "(0150-") {
		t.Errorf("ADR-0136 must have an `- **Amended by:**` line that links ADR-0150, got %q", amendedBy)
	}
}
