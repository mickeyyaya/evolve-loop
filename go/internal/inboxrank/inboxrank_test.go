package inboxrank_test

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

var rankNow = time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)

func defaults() policy.InboxPriorityConfig { return policy.Policy{}.InboxPriorityConfig() }

func item(id string, edits ...func(*inboxbatch.Item)) inboxbatch.Item {
	it := inboxbatch.Item{ID: id, Weight: 0.5, PriorityClass: "stability", CreatedAt: "2026-10-01"}
	for _, edit := range edits {
		edit(&it)
	}
	return it
}

func class(c string) func(*inboxbatch.Item) {
	return func(it *inboxbatch.Item) { it.PriorityClass = c }
}
func filed(date string) func(*inboxbatch.Item) {
	return func(it *inboxbatch.Item) { it.CreatedAt = date }
}
func campaign(c string) func(*inboxbatch.Item)  { return func(it *inboxbatch.Item) { it.Campaign = c } }
func weight(w float64) func(*inboxbatch.Item)   { return func(it *inboxbatch.Item) { it.Weight = w } }
func deps(ids ...string) func(*inboxbatch.Item) { return func(it *inboxbatch.Item) { it.Deps = ids } }
func files(paths ...string) func(*inboxbatch.Item) {
	return func(it *inboxbatch.Item) { it.Files = paths }
}
func ctxFor(queue ...inboxbatch.Item) inboxrank.Context {
	return inboxrank.Context{Now: rankNow, Queue: queue}
}

func ids(ranked []inboxrank.Ranked) []string {
	out := make([]string, len(ranked))
	for i, r := range ranked {
		out[i] = r.Item.ID
	}
	return out
}

func scoreOf(t *testing.T, ranked []inboxrank.Ranked, id string) float64 {
	t.Helper()
	for _, r := range ranked {
		if r.Item.ID == id {
			return r.Breakdown.Score
		}
	}
	t.Fatalf("%s is not ranked", id)
	return 0
}

func assertFirst(t *testing.T, ranked []inboxrank.Ranked, winner, loser string) {
	t.Helper()
	if got := ids(ranked); got[0] != winner {
		t.Fatalf("order = %v, want %s first", got, winner)
	}
	if scoreOf(t, ranked, winner) <= scoreOf(t, ranked, loser) {
		t.Fatalf("%s must outscore %s, not win a tie-break: %v <= %v", winner, loser, scoreOf(t, ranked, winner), scoreOf(t, ranked, loser))
	}
}

func TestOrder_AHygieneItemOutranksAnOtherwiseEqualSecurityItem(t *testing.T) {
	items := []inboxbatch.Item{item("a-security", class("security")), item("b-hygiene", class("hygiene"))}

	ranked := inboxrank.Order(items, defaults(), ctxFor(items...))

	assertFirst(t, ranked, "b-hygiene", "a-security")
}

func TestOrder_ACorrectnessItemOutranksEveryOtherClass(t *testing.T) {
	var items []inboxbatch.Item
	for _, c := range []string{"security", "hygiene", "maintainability", "feature", "debuggability", "performance", "stability", "correctness"} {
		items = append(items, item("item-"+c, class(c)))
	}

	ranked := inboxrank.Order(items, defaults(), ctxFor(items...))

	want := []string{"item-correctness", "item-stability", "item-performance", "item-debuggability", "item-feature", "item-maintainability", "item-hygiene", "item-security"}
	if got := ids(ranked); !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v, want the operator's class order %v", got, want)
	}
}

func TestOrder_TheClassOrderFollowsThePolicy(t *testing.T) {
	items := []inboxbatch.Item{item("a-hygiene", class("hygiene")), item("b-security", class("security"))}
	cfg := defaults()
	cfg.ClassOrder = []string{"security", "hygiene"}

	ranked := inboxrank.Order(items, cfg, ctxFor(items...))

	assertFirst(t, ranked, "b-security", "a-hygiene")
}

func TestOrder_AnItemThatUnblocksTwoOthersRises(t *testing.T) {
	for name, waiting := range map[string][]inboxbatch.Item{
		"directly":     {item("w1", deps("b-blocker")), item("w2", deps("b-blocker"))},
		"transitively": {item("w1", deps("b-blocker")), item("w2", deps("w1"))},
	} {
		t.Run(name, func(t *testing.T) {
			ready := []inboxbatch.Item{item("a-free"), item("b-blocker")}

			ranked := inboxrank.Order(ready, defaults(), ctxFor(append(ready, waiting...)...))

			assertFirst(t, ranked, "b-blocker", "a-free")
			if facts := ranked[0].Breakdown.Facts; facts.Unblocks != 2 {
				t.Errorf("Unblocks = %d, want the 2 queued items that wait on it", facts.Unblocks)
			}
		})
	}
}

func TestOrder_UnblocksCountsEachDependentOnceAndSurvivesADependencyCycle(t *testing.T) {
	queue := []inboxbatch.Item{
		item("a", deps("c")), item("b", deps("a", "a")), item("c", deps("b")), item("d", deps("a", "b")),
	}

	got := inboxrank.Score(queue[0], defaults(), ctxFor(queue...)).Facts.Unblocks

	if got != 3 {
		t.Fatalf("Unblocks(a) = %d, want 3 (b, c, d; never a itself)", got)
	}
}

func TestOrder_AnOldItemRisesOverAnIdenticalNewOne(t *testing.T) {
	items := []inboxbatch.Item{item("a-new", filed("2026-10-05")), item("b-old", filed("2026-07-08"))}

	ranked := inboxrank.Order(items, defaults(), ctxFor(items...))

	assertFirst(t, ranked, "b-old", "a-new")
}

func TestOrder_AnActiveCampaignMemberRises(t *testing.T) {
	items := []inboxbatch.Item{item("a-other", campaign("docs-sweep")), item("b-member", campaign("inbox-priority"))}
	cfg := defaults()
	cfg.ActiveCampaigns = []string{"inbox-priority"}

	ranked := inboxrank.Order(items, cfg, ctxFor(items...))

	assertFirst(t, ranked, "b-member", "a-other")
	if again := inboxrank.Order(items, defaults(), ctxFor(items...)); scoreOf(t, again, "a-other") != scoreOf(t, again, "b-member") {
		t.Error("with no active campaign the two items must score alike")
	}
}

func TestOrder_ARecurrenceCountMovesTheRank(t *testing.T) {
	items := []inboxbatch.Item{item("a-quiet"), item("b-recurring")}
	ctx := ctxFor(items...)
	ctx.Recurrence = func(it inboxbatch.Item) int {
		if it.ID == "b-recurring" {
			return 3
		}
		return 0
	}

	ranked := inboxrank.Order(items, defaults(), ctx)

	assertFirst(t, ranked, "b-recurring", "a-quiet")
	if ranked[0].Item.Weight != 0.5 {
		t.Errorf("weight = %v: recurrence is a rank factor and never rewrites the weight", ranked[0].Item.Weight)
	}
}

func permutations(items []inboxbatch.Item) [][]inboxbatch.Item {
	if len(items) <= 1 {
		return [][]inboxbatch.Item{items}
	}
	var out [][]inboxbatch.Item
	for i := range items {
		rest := append(append([]inboxbatch.Item(nil), items[:i]...), items[i+1:]...)
		for _, tail := range permutations(rest) {
			out = append(out, append([]inboxbatch.Item{items[i]}, tail...))
		}
	}
	return out
}

func TestOrder_TiesBreakOnDeclaredSurfaceThenTheOlderFilingThenTheID(t *testing.T) {
	items := []inboxbatch.Item{
		item("f-undated", filed("")),
		item("e-undated", filed("")),
		item("d-new", filed("2026-10-02")),
		item("c-same-day-later-id", filed("2026-10-01")),
		item("b-same-day", filed("2026-10-01")),
		item("a-declared", files("go/internal/inboxrank/inboxrank.go"), filed("2026-10-03")),
	}
	cfg := defaults()
	cfg.Factors = policy.InboxPriorityFactors{Base: 1}
	want := []string{"a-declared", "b-same-day", "c-same-day-later-id", "d-new", "e-undated", "f-undated"}

	for _, input := range permutations(items) {
		ranked := inboxrank.Order(input, cfg, ctxFor(input...))

		if got := ids(ranked); !reflect.DeepEqual(got, want) {
			t.Fatalf("input %v: order = %v, want %v", ids(inboxrank.Order(input, cfg, inboxrank.Context{})), got, want)
		}
		for i, r := range ranked {
			if r.Rank != i+1 {
				t.Fatalf("%s: Rank = %d, want %d", r.Item.ID, r.Rank, i+1)
			}
		}
	}
}

func TestOrder_ASharedIDFallsBackToThePath(t *testing.T) {
	first, second := item("dup"), item("dup")
	first.Path, second.Path = "2026-10-01T00-00-00Z-dup.json", "2026-10-02T00-00-00Z-dup.json"

	for _, input := range [][]inboxbatch.Item{{first, second}, {second, first}} {
		if got := inboxrank.Order(input, defaults(), ctxFor(input...)); got[0].Item.Path != first.Path {
			t.Errorf("order = %s, %s, want the path order", got[0].Item.Path, got[1].Item.Path)
		}
	}
}

func TestOrder_IsStableAcrossRunsAndInputOrder(t *testing.T) {
	var items []inboxbatch.Item
	for _, c := range []string{"security", "hygiene", "feature", "stability", "unknown"} {
		for _, id := range []string{"x", "y", "z"} {
			items = append(items, item(c+"-"+id, class(c), deps(c+"-blocker")), item(c+"-blocker", class(c)))
		}
	}
	want := ids(inboxrank.Order(items, defaults(), ctxFor(items...)))
	shuffle := rand.New(rand.NewSource(1))

	for run := 0; run < 50; run++ {
		shuffled := append([]inboxbatch.Item(nil), items...)
		shuffle.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })

		if got := ids(inboxrank.Order(shuffled, defaults(), ctxFor(shuffled...))); !reflect.DeepEqual(got, want) {
			t.Fatalf("run %d: order = %v, want %v", run, got, want)
		}
	}
}

func TestOrder_NeverChangesItsInput(t *testing.T) {
	items := []inboxbatch.Item{item("a", class("hygiene")), item("b", class("security"))}

	inboxrank.Order(items, defaults(), ctxFor(items...))

	if items[0].ID != "a" || items[1].ID != "b" {
		t.Errorf("input reordered to %s, %s", items[0].ID, items[1].ID)
	}
}

func TestScore_ContributionsAddUpToTheScore(t *testing.T) {
	queue := []inboxbatch.Item{
		item("a", class("security"), weight(0.37), filed("2026-08-01"), campaign("c")),
		item("b", deps("a")), item("c", deps("b")),
	}
	cfg := defaults()
	cfg.ActiveCampaigns = []string{"c"}
	ctx := ctxFor(queue...)
	ctx.Recurrence = func(inboxbatch.Item) int { return 2 }

	b := inboxrank.Score(queue[0], cfg, ctx)

	filerWeight, baseFactor := 0.37, cfg.Factors.Base
	if base := (inboxrank.Term{Factor: inboxrank.FactorBase, Value: filerWeight, Weight: baseFactor, Contribution: filerWeight * baseFactor}); b.Terms[0] != base {
		t.Errorf("base term = %+v, want the filer's weight times the base factor %+v", b.Terms[0], base)
	}
	sum := 0.0
	for _, term := range b.Terms {
		if term.Contribution != term.Value*term.Weight {
			t.Errorf("%s: contribution %v != value %v x weight %v", term.Factor, term.Contribution, term.Value, term.Weight)
		}
		sum += term.Contribution
	}
	if sum != b.Score {
		t.Errorf("contributions sum to %v, score is %v", sum, b.Score)
	}
	var got []inboxrank.Factor
	for _, term := range b.Terms {
		got = append(got, term.Factor)
	}
	if want := inboxrank.Factors(); !reflect.DeepEqual(got, want) {
		t.Errorf("terms = %v, want every factor in the one order %v", got, want)
	}
}

func TestFactors_ListsEveryFactorOnceInScoreOrder(t *testing.T) {
	want := []inboxrank.Factor{inboxrank.FactorBase, inboxrank.FactorClass, inboxrank.FactorUnblocks, inboxrank.FactorRecurrence, inboxrank.FactorAge, inboxrank.FactorGoal}

	got := inboxrank.Factors()
	got[0] = "mutated"

	if again := inboxrank.Factors(); !reflect.DeepEqual(again, want) {
		t.Errorf("Factors() = %v, want %v, and a caller's edit must not reach the next call", again, want)
	}
}

func decodeFactors(t *testing.T, weights map[string]float64) (policy.InboxPriorityConfig, error) {
	t.Helper()
	raw, err := json.Marshal(map[string]map[string]float64{"factors": weights})
	if err != nil {
		t.Fatal(err)
	}
	var block policy.InboxPriorityPolicy
	if err := json.Unmarshal(raw, &block); err != nil {
		return policy.InboxPriorityConfig{}, err
	}
	return policy.Policy{InboxPriority: &block}.InboxPriorityConfig(), nil
}

func TestFactors_AreThePolicysFactorNamesInItsOwnOrder(t *testing.T) {
	var names []string
	for _, f := range inboxrank.Factors() {
		names = append(names, string(f))
	}

	if _, err := decodeFactors(t, map[string]float64{}); err == nil || !strings.Contains(err.Error(), "name every one of "+strings.Join(names, ", ")) {
		t.Errorf("a factors block naming none = %v, want the policy to list %v in score order", err, names)
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			solo := map[string]float64{}
			for _, other := range names {
				solo[other] = 0
			}
			solo[name] = -1
			if _, err := decodeFactors(t, solo); err == nil || !strings.Contains(err.Error(), fmt.Sprintf("%q must not be negative", name)) {
				t.Errorf("a negative %s = %v, want the policy to name %s: its names and weights must index-match", name, err, name)
			}
			solo[name] = 1
			cfg, err := decodeFactors(t, solo)
			if err != nil {
				t.Fatal(err)
			}
			for _, term := range inboxrank.Score(item("a"), cfg, ctxFor()).Terms {
				want := 0.0
				if string(term.Factor) == name {
					want = 1
				}
				if term.Weight != want {
					t.Errorf("policy key %s set alone: term %s has weight %v, want %v", name, term.Factor, term.Weight, want)
				}
			}
		})
	}
}

func TestScore_OrderReportsTheSameBreakdownScoreDoes(t *testing.T) {
	queue := []inboxbatch.Item{item("a", class("feature")), item("b", deps("a"))}

	ranked := inboxrank.Order(queue[:1], defaults(), ctxFor(queue...))

	if want := inboxrank.Score(queue[0], defaults(), ctxFor(queue...)); !reflect.DeepEqual(ranked[0].Breakdown, want) {
		t.Errorf("Order breakdown = %+v, Score = %+v", ranked[0].Breakdown, want)
	}
}

func termValue(b inboxrank.Breakdown, f inboxrank.Factor) float64 {
	for _, term := range b.Terms {
		if term.Factor == f {
			return term.Value
		}
	}
	return math.NaN()
}

func TestScore_NormalizesEveryFeatureIntoTheUnitInterval(t *testing.T) {
	queue := []inboxbatch.Item{item("a", weight(1.7))}
	for i := 0; i < 9; i++ {
		queue = append(queue, item(strings.Repeat("w", i+1), deps("a")))
	}
	ctx := ctxFor(queue...)
	ctx.Recurrence = func(inboxbatch.Item) int { return 40 }
	cases := []struct {
		name   string
		it     inboxbatch.Item
		factor inboxrank.Factor
		want   float64
	}{
		{"a weight above one clamps", queue[0], inboxrank.FactorBase, 1},
		{"a negative weight clamps", item("neg", weight(-0.2)), inboxrank.FactorBase, 0},
		{"the first class is one", item("c", class("correctness")), inboxrank.FactorClass, 1},
		{"the last class is one eighth", item("s", class("security")), inboxrank.FactorClass, 0.125},
		{"an unknown class is zero", item("u", class("urgent")), inboxrank.FactorClass, 0},
		{"no class is zero", item("n", class("")), inboxrank.FactorClass, 0},
		{"unblocks beyond the cap is one", queue[0], inboxrank.FactorUnblocks, 1},
		{"recurrence beyond the cap is one", queue[0], inboxrank.FactorRecurrence, 1},
		{"one half-life of age is one half", item("o", filed("2026-09-06")), inboxrank.FactorAge, 0.5},
		{"filed today has no age", item("t", filed("2026-10-06")), inboxrank.FactorAge, 0},
		{"a future filing has no age", item("f", filed("2026-11-01")), inboxrank.FactorAge, 0},
		{"an undated item has no age", item("d", filed("")), inboxrank.FactorAge, 0},
		{"no campaign is not a goal", item("g", campaign("")), inboxrank.FactorGoal, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := termValue(inboxrank.Score(tc.it, defaults(), ctx), tc.factor); got != tc.want {
				t.Errorf("%s value = %v, want %v", tc.factor, got, tc.want)
			}
		})
	}
}

func TestScore_WithoutARecurrenceSeamCountsNone(t *testing.T) {
	b := inboxrank.Score(item("a"), defaults(), inboxrank.Context{Now: rankNow})

	if b.Facts.Recurrence != 0 || termValue(b, inboxrank.FactorRecurrence) != 0 || b.Facts.Unblocks != 0 {
		t.Errorf("facts = %+v, want no recurrence and no dependents", b.Facts)
	}
}

func TestScore_RecordsTheFactsBehindEachFeature(t *testing.T) {
	cfg := defaults()
	cfg.ActiveCampaigns = []string{"camp"}

	facts := inboxrank.Score(item("a", class("urgent"), campaign("camp"), filed("2026-09-26")), cfg, ctxFor()).Facts

	want := inboxrank.Facts{Class: "urgent", ClassPosition: -1, AgeDays: 10, Dated: true, Campaign: "camp", CampaignActive: true}
	if facts != want {
		t.Errorf("facts = %+v, want %+v", facts, want)
	}
}

func TestClassWarnings_NameEveryItemWhoseClassTheOrderLacks(t *testing.T) {
	items := []inboxbatch.Item{item("ok", class("security")), item("typo", class("securty")), item("none", class(""))}

	got := inboxrank.ClassWarnings(items, defaults())

	if len(got) != 2 || !strings.HasPrefix(got[0], "typo: ") || !strings.Contains(got[0], `"securty"`) ||
		!strings.HasPrefix(got[1], "none: ") || !strings.Contains(got[1], "below every class") {
		t.Errorf("ClassWarnings = %q, want the two unknown classes named, each ranked below every class", got)
	}
}

func TestScore_TheClassPositionIsTheOneFactBehindTheClassFeature(t *testing.T) {
	cfg := defaults()
	classes := len(cfg.ClassOrder)

	for i, c := range append(slices.Clone(cfg.ClassOrder), "urgent", "", "Security") {
		b := inboxrank.Score(item("a", class(c)), cfg, ctxFor())

		wantPosition, wantValue := i, float64(classes-i)/float64(classes)
		if i >= classes {
			wantPosition, wantValue = -1, 0
		}
		if b.Facts.ClassPosition != wantPosition || b.Facts.ClassKnown() != (wantPosition >= 0) || termValue(b, inboxrank.FactorClass) != wantValue {
			t.Errorf("%q: position %d known %v value %v, want %d, %v, %v",
				c, b.Facts.ClassPosition, b.Facts.ClassKnown(), termValue(b, inboxrank.FactorClass), wantPosition, wantPosition >= 0, wantValue)
		}
	}
}

func TestScore_ANegativeRecurrenceCountIsNoRecurrence(t *testing.T) {
	ctx := ctxFor()
	ctx.Recurrence = func(inboxbatch.Item) int { return -3 }

	got := termValue(inboxrank.Score(item("a"), defaults(), ctx), inboxrank.FactorRecurrence)

	if got != 0 {
		t.Errorf("recurrence value = %v for a count of -3, want 0: every feature stays in [0, 1]", got)
	}
}
