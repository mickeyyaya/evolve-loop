package reviewclass

import (
	"path"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/derived"
	"github.com/mickeyyaya/evolve-loop/go/internal/gateclass"
)

const TierDeep = "deep"

var tierRank = map[string]int{"": 0, TierDeep: 1}

func Classes() []Class {
	out := make([]Class, len(table))
	for i, c := range table {
		out[i] = c.clone()
	}
	return out
}

func (c Class) clone() Class {
	c.Seats = slices.Clone(c.Seats)
	for i := range c.Seats {
		c.Seats[i].Any = slices.Clone(c.Seats[i].Any)
	}
	c.SecurityWhen = slices.Clone(c.SecurityWhen)
	c.Checks = slices.Clone(c.Checks)
	return c
}

func Lookup(id ID) (Class, bool) {
	for _, c := range table {
		if c.ID == id {
			return c.clone(), true
		}
	}
	return Class{}, false
}

func KnownDefaults() []string { return slices.Clone(knownDefaults) }

func KnownDefault(p string) (string, bool) {
	for _, pattern := range knownDefaults {
		if strings.HasSuffix(pattern, "/") && strings.HasPrefix(p, pattern) {
			return pattern, true
		}
		if ok, _ := path.Match(pattern, p); ok {
			return pattern, true
		}
	}
	return "", false
}

func (s Seat) String() string {
	var notes []string
	if s.Lens != "" {
		notes = append(notes, "lens="+s.Lens)
	}
	if s.Tier != "" {
		notes = append(notes, "tier="+s.Tier)
	}
	if s.OtherFamily {
		notes = append(notes, "other-family")
	}
	name := strings.Join(s.Any, "|")
	if len(notes) == 0 {
		return name
	}
	return name + "[" + strings.Join(notes, ",") + "]"
}

func (s Seat) key() string { return strings.Join(s.Any, "|") + "[" + s.Lens + "]" }

func mergeSeat(seats []Seat, s Seat) []Seat {
	i := slices.IndexFunc(seats, func(have Seat) bool { return have.key() == s.key() })
	if i < 0 {
		return append(seats, s)
	}
	if tierRank[s.Tier] > tierRank[seats[i].Tier] {
		seats[i].Tier = s.Tier
	}
	seats[i].OtherFamily = seats[i].OtherFamily || s.OtherFamily
	return seats
}

func mergeCheck(checks []Check, c Check) []Check {
	if slices.ContainsFunc(checks, func(have Check) bool { return have.ID == c.ID }) {
		return checks
	}
	return append(checks, c)
}

func listOf(items []string) string { return strings.Join(items, ", ") }

func onPath(match func(string) bool) func(Surface, string) bool {
	return func(_ Surface, p string) bool { return match(p) }
}

func underAny(dirs []string) func(string) bool {
	return func(p string) bool { return hasAnyPrefix(p, dirs...) }
}

func (s Surface) isProtected(p string) bool { return s.Protected == nil || s.Protected(p) }

func isWholeGenerated(p string) bool {
	_, o, ok := derived.OutputOf(p)
	return ok && o.Region == ""
}

func isGenerated(p string) bool {
	_, _, ok := derived.OutputOf(p)
	return ok
}

func isGateInput(p string) bool { return slices.Contains(gateInputs, p) }

func hasAnyPrefix(p string, prefixes ...string) bool {
	return slices.ContainsFunc(prefixes, func(prefix string) bool { return strings.HasPrefix(p, prefix) })
}

func isDoc(p string) bool {
	return strings.HasSuffix(p, ".md") && !hasAnyPrefix(p, notDocDirs...) && !isWholeGenerated(p)
}

func inTestdata(p string) bool {
	return strings.HasPrefix(p, "go/") && strings.Contains(p, "/testdata/")
}

func isTestOnly(p string) bool {
	return inTestdata(p) || (strings.HasSuffix(p, "_test.go") && !gateclass.IsGuardTest(p))
}

func isCodeOf(tier gateclass.Tier) func(string) bool {
	return func(p string) bool {
		code := strings.HasSuffix(p, ".go") || hasAnyPrefix(p, packageDirs...)
		return code && !isTestOnly(p) && gateclass.TierOf(p, nil) == tier
	}
}

func isPersona(p string) bool {
	return (hasAnyPrefix(p, promptDirs...) && strings.HasSuffix(p, ".md")) || hasAnyPrefix(p, personaDirs...)
}

func isManifest(p string) bool {
	return slices.ContainsFunc(manifestGlobs, func(glob string) bool {
		ok, _ := path.Match(glob, p)
		return ok
	}) || hasAnyPrefix(p, manifestDirs...)
}

func (d Diff) classesOf(s Surface, p string) []ID {
	if slices.Contains(d.CommentOnly, p) {
		return []ID{C0}
	}
	var ids []ID
	for _, c := range table {
		if c.match != nil && c.match(s, p) {
			ids = append(ids, c.ID)
		}
	}
	if slices.Contains(d.StageFlips, p) {
		ids = append(ids, C12)
	}
	if len(ids) == 0 {
		return []ID{C13}
	}
	return ids
}

func Classify(s Surface, d Diff) Requirements {
	byClass := map[ID][]string{}
	for _, p := range d.Paths {
		for _, id := range d.classesOf(s, p) {
			byClass[id] = append(byClass[id], p)
		}
	}
	req := Requirements{Classes: []Hit{}, Seats: []Seat{}, Checks: []Check{}}
	for _, c := range table {
		hitPaths, ok := byClass[c.ID]
		if !ok {
			continue
		}
		slices.Sort(hitPaths)
		req.Classes = append(req.Classes, Hit{Class: c.ID, Name: c.Name, Paths: hitPaths})
		for _, seat := range c.seatsFor(s, hitPaths) {
			req.Seats = mergeSeat(req.Seats, seat)
		}
		for _, check := range c.checksFor(hitPaths) {
			req.Checks = mergeCheck(req.Checks, check)
		}
	}
	return req
}

func (c Class) seatsFor(s Surface, hitPaths []string) []Seat {
	seats := c.clone().Seats
	if len(c.SecurityWhen) > 0 && slices.ContainsFunc(hitPaths, func(p string) bool { return c.needsSecurity(s, p) }) {
		seats = append(seats, securitySeat)
	}
	return seats
}

func (c Class) needsSecurity(s Surface, p string) bool {
	if s.Guarantees == nil {
		return true
	}
	return slices.ContainsFunc(s.Guarantees(p), func(g string) bool { return slices.Contains(c.SecurityWhen, g) })
}

func (c Class) checksFor(hitPaths []string) []Check {
	if c.ID != C3 {
		return c.Checks
	}
	var checks []Check
	for _, p := range hitPaths {
		e, _, _ := derived.OutputOf(p)
		checks = mergeCheck(checks, Check{ID: derivedCheck.ID + ":" + e.Name, Rule: "evolve " + strings.Join(e.Check, " ")})
	}
	return checks
}
