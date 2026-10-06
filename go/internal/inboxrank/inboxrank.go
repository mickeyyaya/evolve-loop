// Package inboxrank ranks pending inbox items by one computed, explainable priority score.
// See docs/architecture/packages/internal-inboxrank.md.
package inboxrank

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type Factor string

const (
	FactorBase       Factor = "base"
	FactorClass      Factor = "class"
	FactorUnblocks   Factor = "unblocks"
	FactorRecurrence Factor = "recurrence"
	FactorAge        Factor = "age"
	FactorGoal       Factor = "goal"
)

const hoursPerDay = 24

func Factors() []Factor {
	return []Factor{FactorBase, FactorClass, FactorUnblocks, FactorRecurrence, FactorAge, FactorGoal}
}

type Context struct {
	Now        time.Time
	Queue      []inboxbatch.Item
	Recurrence func(inboxbatch.Item) int
}

type Term struct {
	Factor       Factor  `json:"factor"`
	Value        float64 `json:"value"`
	Weight       float64 `json:"weight"`
	Contribution float64 `json:"contribution"`
}

type Facts struct {
	Class          string  `json:"class"`
	ClassPosition  int     `json:"class_position"`
	Unblocks       int     `json:"unblocks"`
	Recurrence     int     `json:"recurrence"`
	AgeDays        float64 `json:"age_days"`
	Dated          bool    `json:"dated"`
	Campaign       string  `json:"campaign,omitempty"`
	CampaignActive bool    `json:"campaign_active"`
}

func (f Facts) ClassKnown() bool {
	return f.ClassPosition >= 0
}

type Breakdown struct {
	Score float64 `json:"score"`
	Terms []Term  `json:"terms"`
	Facts Facts   `json:"facts"`
}

type Ranked struct {
	Rank      int
	Item      inboxbatch.Item
	Breakdown Breakdown
}

func Score(it inboxbatch.Item, cfg policy.InboxPriorityConfig, ctx Context) Breakdown {
	return newScorer(cfg, ctx).score(it)
}

func Order(items []inboxbatch.Item, cfg policy.InboxPriorityConfig, ctx Context) []Ranked {
	s := newScorer(cfg, ctx)
	entries := make([]rankEntry, len(items))
	for i, it := range items {
		entries[i] = rankEntry{
			ranked:   Ranked{Item: it, Breakdown: s.score(it)},
			declared: it.DeclaredSurface(),
			filed:    it.FiledAt(),
		}
	}
	slices.SortStableFunc(entries, compareEntries)
	ranked := make([]Ranked, len(entries))
	for i, e := range entries {
		ranked[i] = e.ranked
		ranked[i].Rank = i + 1
	}
	return ranked
}

func ClassWarnings(items []inboxbatch.Item, cfg policy.InboxPriorityConfig) []string {
	var warnings []string
	for _, it := range items {
		if err := inboxbatch.CheckPriorityClass(it.PriorityClass, cfg.ClassOrder); err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v; ranked below every class", it.ID, err))
		}
	}
	return warnings
}

type rankEntry struct {
	ranked   Ranked
	declared bool
	filed    time.Time
}

func compareEntries(a, b rankEntry) int {
	if c := cmp.Compare(b.ranked.Breakdown.Score, a.ranked.Breakdown.Score); c != 0 {
		return c
	}
	if a.declared != b.declared {
		return boolFirst(a.declared)
	}
	if c := compareFiled(a.filed, b.filed); c != 0 {
		return c
	}
	if c := strings.Compare(a.ranked.Item.ID, b.ranked.Item.ID); c != 0 {
		return c
	}
	return strings.Compare(a.ranked.Item.Path, b.ranked.Item.Path)
}

func boolFirst(aIsTrue bool) int {
	if aIsTrue {
		return -1
	}
	return 1
}

func compareFiled(a, b time.Time) int {
	switch {
	case a.IsZero() && b.IsZero():
		return 0
	case a.IsZero():
		return 1
	case b.IsZero():
		return -1
	}
	return a.Compare(b)
}

type scorer struct {
	cfg        policy.InboxPriorityConfig
	now        time.Time
	recurrence func(inboxbatch.Item) int
	dependents map[string][]string
}

func newScorer(cfg policy.InboxPriorityConfig, ctx Context) scorer {
	dependents := map[string][]string{}
	for _, queued := range ctx.Queue {
		for _, dep := range queued.Deps {
			dependents[dep] = append(dependents[dep], queued.ID)
		}
	}
	countRecurrence := ctx.Recurrence
	if countRecurrence == nil {
		countRecurrence = func(inboxbatch.Item) int { return 0 }
	}
	return scorer{cfg: cfg, now: ctx.Now, recurrence: countRecurrence, dependents: dependents}
}

func (s scorer) score(it inboxbatch.Item) Breakdown {
	facts := s.facts(it)
	f := s.cfg.Factors
	features := []Term{
		{Factor: FactorBase, Value: unitClamp(it.Weight), Weight: f.Base},
		{Factor: FactorClass, Value: s.classValue(facts.ClassPosition), Weight: f.Class},
		{Factor: FactorUnblocks, Value: capped(facts.Unblocks, s.cfg.UnblocksCap), Weight: f.Unblocks},
		{Factor: FactorRecurrence, Value: capped(facts.Recurrence, s.cfg.RecurrenceCap), Weight: f.Recurrence},
		{Factor: FactorAge, Value: s.ageValue(facts.AgeDays), Weight: f.Age},
		{Factor: FactorGoal, Value: indicator(facts.CampaignActive), Weight: f.Goal},
	}
	b := Breakdown{Terms: features, Facts: facts}
	for i := range b.Terms {
		b.Terms[i].Contribution = float64(b.Terms[i].Value * b.Terms[i].Weight)
		b.Score += b.Terms[i].Contribution
	}
	return b
}

func (s scorer) facts(it inboxbatch.Item) Facts {
	ageDays, dated := s.ageDays(it)
	campaign := strings.TrimSpace(it.Campaign)
	return Facts{
		Class:          it.PriorityClass,
		ClassPosition:  inboxbatch.PriorityClassPosition(it.PriorityClass, s.cfg.ClassOrder),
		Unblocks:       s.unblocks(it.ID),
		Recurrence:     s.recurrence(it),
		AgeDays:        ageDays,
		Dated:          dated,
		Campaign:       campaign,
		CampaignActive: campaign != "" && slices.Contains(s.cfg.ActiveCampaigns, campaign),
	}
}

func (s scorer) unblocks(id string) int {
	seen := map[string]bool{id: true}
	frontier := []string{id}
	for len(frontier) > 0 {
		next := frontier[0]
		frontier = frontier[1:]
		for _, dependent := range s.dependents[next] {
			if !seen[dependent] {
				seen[dependent] = true
				frontier = append(frontier, dependent)
			}
		}
	}
	return len(seen) - 1
}

func (s scorer) ageDays(it inboxbatch.Item) (float64, bool) {
	filed := it.FiledAt()
	if filed.IsZero() {
		return 0, false
	}
	return max(0, s.now.Sub(filed).Hours()/hoursPerDay), true
}

func (s scorer) classValue(position int) float64 {
	if position < 0 {
		return 0
	}
	classes := len(s.cfg.ClassOrder)
	return float64(classes-position) / float64(classes)
}

func (s scorer) ageValue(days float64) float64 {
	if days <= 0 {
		return 0
	}
	return 1 - math.Pow(0.5, days/s.cfg.AgeHalflifeDays)
}

func capped(count, ceiling int) float64 {
	return float64(min(max(count, 0), ceiling)) / float64(ceiling)
}

func unitClamp(v float64) float64 {
	return min(1, max(0, v))
}

func indicator(on bool) float64 {
	if on {
		return 1
	}
	return 0
}
