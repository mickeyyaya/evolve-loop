package convergence

import (
	"path"
	"slices"
	"strconv"
	"strings"
)

const unknownComponent = "unknown"

var areaRoots = []string{"skills", "docs", ".evolve", "agents"}

func componentOf(f Finding) string {
	if f.Component != "" {
		return f.Component
	}
	file, _ := splitLocation(f.Location)
	segments := strings.Split(file, "/")
	switch {
	case file == "":
		return unknownComponent
	case len(segments) == 1:
		return file
	case len(segments) > 3 && segments[0] == "go" && segments[1] == "acs":
		return strings.Join(segments[:3], "/")
	case slices.Contains(areaRoots, segments[0]):
		return strings.Join(segments[:2], "/")
	}
	return path.Dir(file)
}

func splitLocation(location string) (string, int) {
	i := strings.LastIndex(location, ":")
	if i < 0 {
		return location, 0
	}
	line, err := strconv.Atoi(location[i+1:])
	if err != nil {
		return location, 0
	}
	return location[:i], line
}

func insideHunks(location string, hunks []Hunk) bool {
	file, line := splitLocation(location)
	return line > 0 && slices.ContainsFunc(hunks, func(h Hunk) bool {
		return h.File == file && h.From <= line && line <= h.To
	})
}

type view struct {
	in    Input
	rules loopRules
}

func (v view) filedByRule(f Finding) bool {
	if !v.rules.filesByRule || f.Status != StatusOpen {
		return false
	}
	switch {
	case f.Severity == SeverityInfo:
		return true
	case f.Kind == KindCapability && f.Severity != SeverityCritical:
		return true
	case f.Late:
		return !f.Severity.atLeast(SeverityHigh) || f.Falsification == FalsificationRefuted
	}
	return false
}

func (v view) unresolved(f Finding) bool { return f.Status == StatusOpen && !v.filedByRule(f) }

func (v view) strict(k int, bar Severity, exempt []string) []Finding {
	var out []Finding
	for _, f := range v.in.Rounds[k].Findings {
		if v.unresolved(f) && f.Severity.atLeast(bar) && !exempted(f, exempt) {
			out = append(out, f)
		}
	}
	return out
}

func exempted(f Finding, exempt []string) bool {
	return f.Severity != SeverityCritical && slices.Contains(exempt, componentOf(f))
}

func (v view) hasOpenCritical(k int) bool { return len(v.strict(k, SeverityCritical, nil)) > 0 }

func (v view) mass(k int, bar Severity) float64 {
	atBar := v.strict(k, bar, nil)
	if v.in.Mass != nil {
		return v.in.Mass(k, atBar)
	}
	return defaultMass(atBar)
}

func defaultMass(findings []Finding) float64 {
	total := 0.0
	for _, f := range findings {
		total += defaultWeights[f.Severity]
	}
	return total
}

type repairEffect struct{ repairs, damage int }

func (v view) effectOf(k int) repairEffect {
	last := latestStatus(v.in.Rounds[:k])
	var e repairEffect
	for _, f := range v.in.Rounds[k].Findings {
		prev, seen := last[f.ID]
		switch {
		case !f.Severity.atLeast(SeverityLow):
		case seen && prev == StatusOpen && f.Status == StatusFixed:
			e.repairs++
		case seen && prev == StatusFixed && f.Status == StatusOpen:
			e.damage++
		case !seen && f.Status == StatusOpen && insideHunks(f.Location, v.in.Rounds[k].FixHunks):
			e.damage++
		}
	}
	return e
}

func latestStatus(js []Judgment) map[string]Status {
	last := map[string]Status{}
	for _, j := range js {
		for _, f := range j.Findings {
			last[f.ID] = f.Status
		}
	}
	return last
}

type roundRank struct {
	strict int
	mass   float64
}

func (r roundRank) better(than roundRank) bool {
	return r.strict < than.strict || (r.strict == than.strict && r.mass < than.mass)
}

func (v view) rankOf(k int, bar Severity, exempt []string) roundRank {
	return roundRank{strict: len(v.strict(k, bar, exempt)), mass: defaultMass(v.strict(k, SeverityLow, exempt))}
}

func (v view) keepBest(bar Severity, exempt []string) int {
	best, bestRank := 0, v.rankOf(0, bar, exempt)
	for k := 1; k <= v.in.Round; k++ {
		if rank := v.rankOf(k, bar, exempt); rank.better(bestRank) {
			best, bestRank = k, rank
		}
	}
	return best
}

func (v view) openIDs(k int) []string {
	var ids []string
	for _, f := range v.in.Rounds[k].Findings {
		if f.Status == StatusOpen {
			ids = append(ids, f.ID)
		}
	}
	return ids
}
