package phasestream

import "fmt"

func isEvictable(k Kind) bool {
	return k == KindToolUse || k == KindToolResult
}

func MaskStaleObservations(events []Envelope, windowTurns int) []Envelope {
	out := make([]Envelope, len(events))
	copy(out, events)
	if windowTurns <= 0 {
		return out
	}
	retained := 0
	for i := len(out) - 1; i >= 0; i-- {
		if !isEvictable(out[i].Kind) {
			continue
		}
		if retained < windowTurns {
			retained++
			continue
		}
		out[i] = maskObservation(out[i])
	}
	return out
}

func maskObservation(e Envelope) Envelope {
	masked := e
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
