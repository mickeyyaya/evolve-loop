package advisor

import "github.com/mickeyyaya/evolve-loop/go/internal/router"

func ParseProposal(stdout string) (*router.Proposal, error) {
	raw, _ := SpanBytes(stdout, '{', '}')
	return DecodeProposal(raw)
}

func ParsePhasePlan(stdout string) (ParsedPlan, error) {
	raw, _ := SpanBytes(stdout, '[', ']')
	return DecodePhasePlan(raw)
}

func StdoutSpan(stdout string, open, close byte) func() ([]byte, bool) {
	return func() ([]byte, bool) { return SpanBytes(stdout, open, close) }
}

func SpanBytes(stdout string, open, close byte) ([]byte, bool) {
	start, end, ok := LastBalancedSpan(stdout, open, close)
	if !ok {
		return nil, false
	}
	return []byte(stdout[start : end+1]), true
}

func LastBalancedSpan(s string, open, close byte) (start, end int, ok bool) {
	depth, spanStart := 0, -1
	inStr, esc := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case open:
			if depth == 0 {
				spanStart = i
			}
			depth++
		case close:
			if depth > 0 {
				depth--
				if depth == 0 && spanStart >= 0 {
					start, end, ok = spanStart, i, true
				}
			}
		}
	}
	return start, end, ok
}
