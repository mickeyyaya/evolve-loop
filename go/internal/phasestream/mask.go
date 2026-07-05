package phasestream

import "fmt"

// isEvictable reports whether an envelope kind is a bulky tool observation whose
// payload MaskStaleObservations may replace with a placeholder. Only tool_use /
// tool_result carry the large file-read / test-run / build-log content that
// dominates agentic input cost; every other kind (verdict, error, thinking,
// interaction, correlation, ...) is a never-evict class and is returned
// unchanged regardless of age.
func isEvictable(k Kind) bool {
	return k == KindToolUse || k == KindToolResult
}

// MaskStaleObservations is the deterministic, LLM-free observation-masking
// transform (research-backed #1 token lever, arXiv 2508.21433 / Anthropic
// context editing): it RETAINS the newest windowTurns evictable tool
// observations (KindToolUse / KindToolResult) and MASKS every older one —
// replacing its bulky content field (input_excerpt for tool_use, excerpt for
// tool_result) with a compact placeholder and marking Data["masked"]=true,
// while preserving the identity keys (name / id / tool_use_id) so the
// reasoning/action chain still reads.
//
// It is PURE: the caller's input envelopes are never mutated in place. The
// returned slice shares the (immutable) envelope values for untouched entries
// and holds fresh copies (with a cloned Data map) for masked entries.
//
// windowTurns <= 0 is the feature-off state: a content-identical copy of the
// input is returned with nothing masked (mirrors this repo's "count<2
// byte-identical" regression guarantee for new config knobs). Non-evictable
// kinds are always returned unchanged, even when they are the oldest envelopes.
func MaskStaleObservations(events []Envelope, windowTurns int) []Envelope {
	out := make([]Envelope, len(events))
	copy(out, events)
	if windowTurns <= 0 {
		return out
	}
	// Walk newest→oldest; keep the first windowTurns evictable observations
	// unmasked, then mask every earlier evictable observation.
	kept := 0
	for i := len(out) - 1; i >= 0; i-- {
		if !isEvictable(out[i].Kind) {
			continue
		}
		if kept < windowTurns {
			kept++
			continue
		}
		out[i] = maskObservation(out[i])
	}
	return out
}

// maskObservation returns a copy of an evictable envelope with its bulky content
// field replaced by a placeholder and Data["masked"]=true. The input is never
// mutated: the returned envelope carries a freshly cloned Data map.
func maskObservation(e Envelope) Envelope {
	masked := e // copy value fields (SchemaVersion, Seq, Kind, ...)
	nd := make(map[string]any, len(e.Data)+1)
	for k, v := range e.Data {
		nd[k] = v
	}
	switch e.Kind {
	case KindToolUse:
		name, _ := nd["name"].(string)
		nd["input_excerpt"] = fmt.Sprintf("[output of %s at seq %d masked - re-run if needed]", name, e.Seq)
	case KindToolResult:
		id, _ := nd["tool_use_id"].(string)
		nd["excerpt"] = fmt.Sprintf("[output of %s at seq %d masked - re-run if needed]", id, e.Seq)
	}
	nd["masked"] = true
	masked.Data = nd
	return masked
}
