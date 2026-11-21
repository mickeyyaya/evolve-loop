package handoff_test

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

var servedModels = map[string]map[string]string{
	"claude": {"fast": "haiku", "balanced": "sonnet", "deep": "opus", "top": "opus", "sonnet": "sonnet", "opus": "opus"},
	"codex":  {"fast": "gpt-mini", "balanced": "gpt-5", "deep": "gpt-5", "top": "gpt-5-pro"},
	"agy": {"fast": "Gemini 4 Hydrogen (Medium)", "balanced": "Gemini 4 Hydrogen (High)",
		"deep": "Gemini 4 Hydrogen (High)", "top": "Gemini 4 Hydrogen (XHigh)"},
	"agy-claude": {"balanced": "Claude Sonnet 5.5 (Medium)", "deep": "Claude Opus 5.5 (Medium)", "top": "Claude Opus 5.5 (High)"},
}

var productionVendors = map[string][]string{
	"agy": {"gemini"}, "agy-claude": {"claude"}, "claude": {"claude"}, "codex": {"gpt"},
}

func served(cli, tier string) (string, bool) {
	model, ok := servedModels[llmroute.Family(cli)][tier]
	return model, ok
}

func TestIndependents_AdmitsOnlyAnotherFamilyOrAStrongerTier(t *testing.T) {
	cases := []struct {
		name   string
		author handoff.Author
		cli    string
		tier   string
		want   bool
	}{
		{"another vendor at a weaker tier", handoff.Author{CLI: "claude-tmux", Tier: "deep"}, "codex-tmux", "fast", true},
		{"another vendor at the same tier", handoff.Author{CLI: "claude-tmux", Tier: "deep"}, "codex-tmux", "deep", true},
		{"another vendor with an unresolved tier", handoff.Author{CLI: "claude-tmux", Tier: "deep"}, "agy-tmux", "", false},
		{"same driver at the same tier", handoff.Author{CLI: "claude-tmux", Tier: "deep"}, "claude-tmux", "deep", false},
		{"same family other driver at the same tier", handoff.Author{CLI: "claude-tmux", Tier: "deep"}, "claude-p", "deep", false},
		{"same family at a weaker tier", handoff.Author{CLI: "claude-tmux", Tier: "deep"}, "claude-tmux", "balanced", false},
		{"same family stronger tier of the same model", handoff.Author{CLI: "claude-tmux", Tier: "deep"}, "claude-tmux", "top", false},
		{"same family stronger tier of another model", handoff.Author{CLI: "claude-tmux", Tier: "balanced"}, "claude-tmux", "deep", true},
		{"same family other driver stronger model", handoff.Author{CLI: "claude-tmux", Tier: "balanced"}, "claude-p", "top", true},
		{"alias of the author tier is the same tier", handoff.Author{CLI: "claude-tmux", Tier: "deep"}, "claude-tmux", "opus", false},
		{"author alias against the canonical name", handoff.Author{CLI: "claude-tmux", Tier: "opus"}, "claude-tmux", "deep", false},
		{"alias of a stronger tier is stronger", handoff.Author{CLI: "claude-tmux", Tier: "sonnet"}, "claude-tmux", "opus", true},
		{"unknown candidate tier is never admitted", handoff.Author{CLI: "claude-tmux", Tier: "deep"}, "claude-tmux", "mystery", false},
		{"bare family name matches its driver", handoff.Author{CLI: "claude", Tier: "deep"}, "claude-tmux", "deep", false},
		{"author tier empty refuses a same-family top seat", handoff.Author{CLI: "claude-tmux", Tier: ""}, "claude-tmux", "top", false},
		{"author tier empty refuses another vendor", handoff.Author{CLI: "claude-tmux", Tier: ""}, "codex-tmux", "fast", false},
		{"author tier unknown refuses every seat", handoff.Author{CLI: "claude-tmux", Tier: "mystery"}, "claude-tmux", "top", false},
		{"author cli empty refuses another vendor", handoff.Author{CLI: "", Tier: "deep"}, "codex-tmux", "top", false},
		{"unattributed author refuses another family", handoff.Author{CLI: handoff.Unattributed, Tier: "deep"}, "codex-tmux", "top", false},
		{"unattributed author refuses a stronger tier", handoff.Author{CLI: handoff.Unattributed, Tier: "deep"}, handoff.Unattributed, "top", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var spec func(cli, tier string) bool = handoff.Independents(c.author, served, productionVendors)
			if got := spec(c.cli, c.tier); got != c.want {
				t.Fatalf("Independents(%+v)(%q, %q) = %v, want %v", c.author, c.cli, c.tier, got, c.want)
			}
		})
	}
}

func TestIndependents_RefusesTheAuthorModelByDriverName(t *testing.T) {
	cases := []struct {
		name   string
		author handoff.Author
		cli    string
		tier   string
	}{
		{"claude deep and top are one model", handoff.Author{CLI: "claude-tmux", Tier: "deep"}, "claude-tmux", "top"},
		{"agy balanced and top are one model at two efforts", handoff.Author{CLI: "agy-tmux", Tier: "balanced"}, "agy-tmux", "top"},
		{"agy-claude and claude are one vendor", handoff.Author{CLI: "agy-claude-tmux", Tier: "deep"}, "claude-tmux", "deep"},
		{"claude and agy-claude are one vendor", handoff.Author{CLI: "claude-tmux", Tier: "deep"}, "agy-claude-tmux", "top"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if handoff.Independents(c.author, served, productionVendors)(c.cli, c.tier) {
				t.Fatalf("seat %s@%s must be refused for author %+v", c.cli, c.tier, c.author)
			}
		})
	}
}

func TestIndependents_ASameFamilySeatAtTheAuthorTierOnAnotherModelIsRefused(t *testing.T) {
	byDriver := func(cli, tier string) (string, bool) {
		if cli == "claude-p" {
			return "sonnet", true
		}
		return "opus", true
	}
	spec := handoff.Independents(handoff.Author{CLI: "claude-tmux", Tier: "deep"}, byDriver, productionVendors)
	if spec("claude-p", "deep") {
		t.Fatal("a same-family seat at the author tier was admitted: the seat must rank strictly higher")
	}
}

func TestIndependents_AnAuthorThatDoesNotResolveRefusesEverySeat(t *testing.T) {
	none := func(string, string) (string, bool) { return "", false }
	spec := handoff.Independents(handoff.Author{CLI: "claude-tmux", Tier: "deep"}, none, productionVendors)
	if spec("codex-tmux", "deep") {
		t.Fatal("an author with no served model must refuse each seat")
	}
	onlySeat := func(cli, tier string) (string, bool) {
		if cli == "codex-tmux" {
			return "gpt-5", true
		}
		return "", false
	}
	if handoff.Independents(handoff.Author{CLI: "claude-tmux", Tier: "deep"}, onlySeat, productionVendors)("codex-tmux", "deep") {
		t.Fatal("an author that does not resolve must refuse even a seat that resolves")
	}
}

func TestIndependents_VendorSets(t *testing.T) {
	author := handoff.Author{CLI: "claude-tmux", Tier: "deep"}
	cases := []struct {
		name    string
		vendors map[string][]string
		cli     string
		want    bool
	}{
		{"a nil map refuses", nil, "codex-tmux", false},
		{"a missing seat key refuses", map[string][]string{"claude": {"claude"}}, "codex-tmux", false},
		{"a missing author key refuses", map[string][]string{"codex": {"gpt"}}, "codex-tmux", false},
		{"an empty seat set refuses", map[string][]string{"claude": {"claude"}, "codex": {}}, "codex-tmux", false},
		{"a driver name as the key is not a family", map[string][]string{"claude-tmux": {"claude"}, "codex-tmux": {"gpt"}}, "codex-tmux", false},
		{"disjoint sets admit", map[string][]string{"claude": {"claude"}, "codex": {"gpt"}}, "codex-tmux", true},
		{"sets that intersect are one vendor", map[string][]string{"claude": {"claude", "shared"}, "codex": {"gpt", "shared"}}, "codex-tmux", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := handoff.Independents(author, served, c.vendors)(c.cli, "deep"); got != c.want {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestIndependents_UnattributedIsTheStampedValue(t *testing.T) {
	if handoff.Unattributed != "unattributed" {
		t.Fatalf("handoff.Unattributed = %q; the ledger stamp and the rule share this one value", handoff.Unattributed)
	}
	spec := handoff.Independents(handoff.Author{CLI: handoff.Unattributed, Tier: "deep"}, served, productionVendors)
	if spec("codex-tmux", "top") {
		t.Fatal("a ledger stamped with handoff.Unattributed must be refused")
	}
}

func TestIdentityBasis_RecordsTheDataTheFilterReads(t *testing.T) {
	author := handoff.Author{CLI: "claude-tmux", Tier: "deep"}
	got := handoff.Identity{Served: served, Vendors: productionVendors}.Basis(author, handoff.Seat{CLI: "agy-tmux", Tier: "top"}, "deep")
	want := handoff.Basis{AuthorModel: "opus", SeatModel: "Gemini 4 Hydrogen", AuthorVendors: []string{"claude"}, SeatVendors: []string{"gemini"}, TierCeiling: "deep"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("basis = %+v, want %+v", got, want)
	}
	none := handoff.Identity{Served: served, Vendors: productionVendors}.Basis(author, handoff.Seat{}, "deep")
	if none.SeatModel != "" || none.SeatVendors != nil || none.AuthorModel != "opus" {
		t.Fatalf("a basis with no seat keeps the author side only: %+v", none)
	}
}
