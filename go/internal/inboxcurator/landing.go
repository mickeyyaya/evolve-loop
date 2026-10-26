package inboxcurator

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

type ProposalKind string

const (
	ProposeConsume ProposalKind = "consume"
	ProposeReroute ProposalKind = "reroute"
	ProposeMerge   ProposalKind = "merge"
)

type Trigger string

const (
	TriggerLanding Trigger = "landing"
	TriggerSweep   Trigger = "sweep"
	TriggerRouting Trigger = "routing"
)

type Proposal struct {
	Kind          ProposalKind `json:"kind"`
	ID            string       `json:"id"`
	Path          string       `json:"path"`
	Target        string       `json:"target,omitempty"`
	Reason        string       `json:"reason"`
	Evidence      []string     `json:"evidence"`
	Trigger       Trigger      `json:"trigger"`
	Deterministic bool         `json:"deterministic"`
}

type Change struct {
	Commit string
	Paths  []string
	Text   string
	Tests  []string
	When   time.Time
}

var testNameRE = regexp.MustCompile(`\b(?:Test|Benchmark|Fuzz)[A-Z0-9_][A-Za-z0-9_]*`)

func NamedTests(it inboxbatch.Item) ([]string, bool) {
	var names []string
	every := len(it.Acceptance) > 0
	for _, criterion := range it.Acceptance {
		found := testNameRE.FindAllString(criterion, -1)
		every = every && len(found) > 0
		for _, name := range found {
			if !slices.Contains(names, name) {
				names = append(names, name)
			}
		}
	}
	return names, every
}

type testEvidence struct {
	found, missing []string
	every          bool
}

func judgeTests(it inboxbatch.Item, hasTest func(string) bool) testEvidence {
	names, every := NamedTests(it)
	ev := testEvidence{every: every}
	for _, name := range names {
		if hasTest(name) {
			ev.found = append(ev.found, name)
		} else {
			ev.missing = append(ev.missing, name)
		}
	}
	return ev
}

func (ev testEvidence) share() float64 {
	return float64(len(ev.found)) / float64(len(ev.found)+len(ev.missing))
}

func (ev testEvidence) lines() []string {
	var out []string
	if len(ev.found) > 0 {
		out = append(out, "tests found: "+strings.Join(ev.found, ", "))
	}
	if len(ev.missing) > 0 {
		out = append(out, "tests missing: "+strings.Join(ev.missing, ", "))
	}
	return out
}

func MatchLanding(c Change, open []inboxbatch.Item, hasTest func(string) bool, minShare float64) []Proposal {
	var out []Proposal
	for _, it := range open {
		if p, ok := matchOne(c, it, hasTest, minShare); ok {
			out = append(out, p)
		}
	}
	return out
}

func matchOne(c Change, it inboxbatch.Item, hasTest func(string) bool, minShare float64) (Proposal, bool) {
	p := Proposal{Kind: ProposeConsume, ID: it.ID, Path: it.Path, Trigger: TriggerLanding, Evidence: []string{"commit " + c.Commit}}
	if consumedBy(c, it) {
		p.Reason, p.Deterministic = "the change consumed this item; the root copy is stale", true
		return p, true
	}
	touched := touchedSurface(it, c.Paths)
	ev := judgeTests(it, hasTest)
	named := namesItem(c.Text, it.ID)
	if len(touched) > 0 {
		p.Evidence = append(p.Evidence, "changed "+strings.Join(touched, ", "))
	}
	p.Evidence = append(p.Evidence, ev.lines()...)
	switch {
	case len(touched) > 0 && len(ev.found) > 0 && ev.share() >= minShare:
		p.Deterministic = ev.every && len(ev.missing) == 0 && definesOneOf(c, ev.found) && !c.When.Before(it.FiledAt())
		p.Reason = fmt.Sprintf("the change touched the fix surface and %d of %d named tests exist", len(ev.found), len(ev.found)+len(ev.missing))
	case named:
		p.Reason = "the change text names the item"
	default:
		return Proposal{}, false
	}
	return p, true
}

func consumedBy(c Change, it inboxbatch.Item) bool {
	for _, p := range c.Paths {
		dir, base := path.Split(p)
		if path.Clean(dir) == ".evolve/inbox/consumed" && base == it.Path {
			return true
		}
	}
	return false
}

func definesOneOf(c Change, names []string) bool {
	return slices.ContainsFunc(names, func(name string) bool { return slices.Contains(c.Tests, name) })
}

func touchedSurface(it inboxbatch.Item, paths []string) []string {
	var touched []string
	for _, declared := range it.DeclaredPaths() {
		for _, changed := range paths {
			if place, ok := samePlace(declared, changed); ok && !slices.Contains(touched, place) {
				touched = append(touched, place)
			}
		}
	}
	slices.Sort(touched)
	return touched
}

var idTokenRE = regexp.MustCompile(`[a-z0-9]+(?:-[a-z0-9]+)+`)

func namesItem(text, id string) bool {
	return slices.Contains(idTokenRE.FindAllString(strings.ToLower(text), -1), id)
}

func Sweep(open []inboxbatch.Item, hasTest func(string) bool, minShare float64) []Proposal {
	var out []Proposal
	for _, it := range open {
		ev := judgeTests(it, hasTest)
		if len(ev.found) == 0 || ev.share() < minShare {
			continue
		}
		out = append(out, Proposal{
			Kind: ProposeConsume, ID: it.ID, Path: it.Path, Trigger: TriggerSweep, Evidence: ev.lines(),
			Reason: fmt.Sprintf("%d of %d named tests exist on the tree; the operator judges the rest of the acceptance", len(ev.found), len(ev.found)+len(ev.missing)),
		})
	}
	return out
}
