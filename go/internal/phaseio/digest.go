package phaseio

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const defaultUpstreamDigestRunes = 1024

var edgeUpstreams = map[string][]string{
	"intent":        {},
	"scout":         {},
	"triage":        {"scout"},
	"tdd":           {"scout", "triage"},
	"build-planner": {"scout", "triage"},
	"swarm-plan":    {"scout", "triage"},
	"build":         {"scout", "triage"},
	"debugger":      {"triage", "build"},
	"audit":         {},
}

func EdgeUpstreams(phase string) ([]string, bool) {
	upstreams, declared := edgeUpstreams[phase]
	if !declared {
		return nil, false
	}
	return append([]string{}, upstreams...), true
}

func (h Handoffs) UpstreamDigest(capRunes int) string {
	lines := h.sectionLines(func(string) bool { return true })
	for _, miss := range h.degraded {
		lines = append(lines, fmt.Sprintf("degraded: %q", miss))
	}
	return capDigest(append(lines, h.genericLines(func(string) bool { return true })...), capRunes)
}

func (h Handoffs) DigestFor(phase string) string {
	upstreams, _ := EdgeUpstreams(phase)
	declared := map[string]bool{}
	for _, upstream := range upstreams {
		declared[upstream] = true
	}
	inGenericSection := func(key string) bool {
		section, _, found := strings.Cut(key, ".")
		return found && declared[section]
	}
	lines := h.sectionLines(func(section string) bool { return declared[section] })
	return capDigest(append(lines, h.genericLines(inGenericSection)...), 0)
}

func (h Handoffs) sectionLines(include func(section string) bool) []string {
	var lines []string
	if h.scout != nil && include("scout") {
		lines = append(lines, fmt.Sprintf("scout: cycle_size=%q items=%d carryover=%d backlog=%d", h.scout.CycleSizeEstimate, h.scout.ItemCount, h.scout.CarryoverCount, h.scout.BacklogSize))
	}
	if h.triage != nil && include("triage") {
		lines = append(lines, fmt.Sprintf("triage: cycle_size=%q phase_skip=%q", h.triage.CycleSize, h.triage.PhaseSkip))
	}
	if h.build != nil && include("build") {
		lines = append(lines, fmt.Sprintf("build: verdict=%q acs=%d/%d red=%d severity=%q", h.build.Verdict, h.build.ACSGreen, h.build.ACSTotal, h.build.ACSRed, h.build.SeverityMax))
	}
	if h.audit != nil && include("audit") {
		lines = append(lines, fmt.Sprintf("audit: verdict=%q confidence=%g red=%d", h.audit.Verdict, h.audit.Confidence, h.audit.RedCount))
	}
	return lines
}

func (h Handoffs) genericLines(include func(key string) bool) []string {
	keys := make([]string, 0, len(h.generic))
	for key := range h.generic {
		if include(key) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		value, err := json.Marshal(h.generic[key])
		if err != nil {
			value = []byte(`"<unencodable>"`)
		}
		lines = append(lines, fmt.Sprintf("generic %q: %s", key, value))
	}
	return lines
}

func capDigest(lines []string, capRunes int) string {
	if len(lines) == 0 {
		return ""
	}
	if capRunes <= 0 {
		capRunes = defaultUpstreamDigestRunes
	}
	joined := strings.Join(lines, "\n")
	if len([]rune(joined)) <= capRunes {
		return joined
	}

	remaining := capRunes - len(lines) + 1
	if remaining < 0 {
		return string([]rune(joined)[:capRunes])
	}
	for i, line := range lines {
		runes := []rune(line)
		share := remaining / (len(lines) - i)
		if len(runes) > share {
			runes = runes[:share]
		}
		lines[i] = string(runes)
		remaining -= len(runes)
	}
	return strings.Join(lines, "\n")
}
