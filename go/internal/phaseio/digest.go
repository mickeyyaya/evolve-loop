package phaseio

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const defaultUpstreamDigestRunes = 1024

// UpstreamDigest returns a deterministic, rune-capped rendering of the sealed
// upstream handoffs. It is intended only for prompt context, so untrusted
// strings are JSON-quoted before a caller places the digest in a text fence.
func (h Handoffs) UpstreamDigest(capRunes int) string {
	var lines []string
	if h.scout != nil {
		lines = append(lines, fmt.Sprintf("scout: cycle_size=%q items=%d carryover=%d backlog=%d", h.scout.CycleSizeEstimate, h.scout.ItemCount, h.scout.CarryoverCount, h.scout.BacklogSize))
	}
	if h.triage != nil {
		lines = append(lines, fmt.Sprintf("triage: cycle_size=%q phase_skip=%q", h.triage.CycleSize, h.triage.PhaseSkip))
	}
	if h.build != nil {
		lines = append(lines, fmt.Sprintf("build: verdict=%q acs=%d/%d red=%d severity=%q", h.build.Verdict, h.build.ACSGreen, h.build.ACSTotal, h.build.ACSRed, h.build.SeverityMax))
	}
	if h.audit != nil {
		lines = append(lines, fmt.Sprintf("audit: verdict=%q confidence=%g red=%d", h.audit.Verdict, h.audit.Confidence, h.audit.RedCount))
	}
	for _, miss := range h.degraded {
		lines = append(lines, fmt.Sprintf("degraded: %q", miss))
	}

	keys := make([]string, 0, len(h.generic))
	for key := range h.generic {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value, err := json.Marshal(h.generic[key])
		if err != nil {
			value = []byte(`"<unencodable>"`)
		}
		lines = append(lines, fmt.Sprintf("generic %q: %s", key, value))
	}

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

	// Reserve space for separators, then share the remaining budget so one
	// oversized section cannot evict every section rendered after it.
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
