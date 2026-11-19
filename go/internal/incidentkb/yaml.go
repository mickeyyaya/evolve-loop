package incidentkb

import (
	"fmt"
	"strconv"
	"strings"
)

func splitFrontmatter(doc []byte) (string, string, bool, error) {
	s := strings.ReplaceAll(string(doc), "\r\n", "\n")
	if !strings.HasPrefix(s, "---\n") {
		return "", s, false, nil
	}
	rest := s[4:]
	if strings.HasPrefix(rest, "---\n") || rest == "---" {
		return "", strings.TrimPrefix(strings.TrimPrefix(rest, "---"), "\n"), true, nil
	}
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return "", "", true, fmt.Errorf("frontmatter missing closing ---")
	}
	after := rest[idx+4:]
	if after != "" && !strings.HasPrefix(after, "\n") {
		return "", "", true, fmt.Errorf("frontmatter closing --- has trailing characters")
	}
	return rest[:idx], strings.TrimPrefix(after, "\n"), true, nil
}

func Parse(doc []byte) (Record, error) {
	front, body, ok, err := splitFrontmatter(doc)
	if err != nil {
		return Record{}, err
	}
	if !ok {
		return Record{}, fmt.Errorf("missing frontmatter")
	}
	lines := tokenizeYAML(front)
	if len(lines) == 0 {
		return Record{}, fmt.Errorf("frontmatter is empty")
	}
	if lines[0].indent != 0 {
		return Record{}, fmt.Errorf("frontmatter: unexpected indent at %q", lines[0].text)
	}
	m, err := parseYAMLMap(lines)
	if err != nil {
		return Record{}, err
	}
	rec, err := decodeRecord(m)
	if err != nil {
		return Record{}, err
	}
	rec.Body = body
	return rec, nil
}

type yamlLine struct {
	indent int
	text   string
}

func tokenizeYAML(front string) []yamlLine {
	rawLines := strings.Split(front, "\n")
	out := make([]yamlLine, 0, len(rawLines))
	for _, raw := range rawLines {
		trimmed := stripYAMLComment(raw)
		if strings.TrimSpace(trimmed) == "" {
			continue
		}
		indent := len(trimmed) - len(strings.TrimLeft(trimmed, " "))
		out = append(out, yamlLine{indent: indent, text: strings.TrimSpace(trimmed)})
	}
	return out
}

func stripYAMLComment(s string) string {
	var inSingle, inDouble bool
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\'' && !inDouble {
			inSingle = !inSingle
		} else if c == '"' && !inSingle && (i == 0 || s[i-1] != '\\') {
			inDouble = !inDouble
		} else if c == '#' && !inSingle && !inDouble && (i == 0 || s[i-1] == ' ' || s[i-1] == '\t') {
			return strings.TrimRight(s[:i], " \t")
		}
	}
	return strings.TrimRight(s, " \t")
}

func collectChildLines(lines []yamlLine, start, parentIndent int) ([]yamlLine, int) {
	i := start
	for i < len(lines) && lines[i].indent > parentIndent {
		i++
	}
	return lines[start:i], i
}

func splitKeyValue(s string) (string, string, bool) {
	var inSingle, inDouble bool
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\'' && !inDouble {
			inSingle = !inSingle
		} else if c == '"' && !inSingle {
			inDouble = !inDouble
		} else if c == ':' && !inSingle && !inDouble && (i+1 == len(s) || s[i+1] == ' ' || s[i+1] == '\t') {
			return unquoteScalar(strings.TrimSpace(s[:i])), strings.TrimSpace(s[i+1:]), true
		}
	}
	return "", "", false
}

func parseYAMLMap(lines []yamlLine) (map[string]any, error) {
	out := map[string]any{}
	baseIndent := lines[0].indent
	for i := 0; i < len(lines); {
		ln := lines[i]
		if ln.indent != baseIndent {
			return nil, fmt.Errorf("frontmatter: unexpected indent at %q", ln.text)
		}
		k, v, ok := splitKeyValue(ln.text)
		if !ok {
			return nil, fmt.Errorf("frontmatter: expected key: value at %q", ln.text)
		}
		i++
		sub, next := collectChildLines(lines, i, ln.indent)
		i = next
		val, err := parseYAMLNodeValue(k, v, sub)
		if err != nil {
			return nil, err
		}
		out[k] = val
	}
	return out, nil
}

func parseYAMLNodeValue(ctx, v string, sub []yamlLine) (any, error) {
	if v != "" && len(sub) > 0 {
		return nil, fmt.Errorf("frontmatter: %s: cannot mix inline value and block", ctx)
	}
	if v != "" {
		return parseFlowValue(ctx, v)
	}
	if len(sub) == 0 {
		return "", nil
	}
	if strings.HasPrefix(sub[0].text, "-") {
		return parseYAMLSeq(sub)
	}
	return parseYAMLMap(sub)
}

func parseYAMLSeq(lines []yamlLine) ([]any, error) {
	baseIndent := lines[0].indent
	out := make([]any, 0, len(lines))
	for i := 0; i < len(lines); {
		ln := lines[i]
		if ln.indent != baseIndent || !strings.HasPrefix(ln.text, "-") {
			return nil, fmt.Errorf("frontmatter: expected - item at %q", ln.text)
		}
		rest := strings.TrimSpace(ln.text[1:])
		i++
		sub, next := collectChildLines(lines, i, ln.indent)
		i = next
		item, err := parseYAMLSeqItem(rest, sub)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func parseYAMLSeqItem(rest string, sub []yamlLine) (any, error) {
	if strings.HasPrefix(rest, "{") || strings.HasPrefix(rest, "[") {
		if len(sub) > 0 {
			return nil, fmt.Errorf("frontmatter: flow item cannot have nested block")
		}
		return parseFlowValue("item", rest)
	}
	if _, _, isKV := splitKeyValue(rest); isKV || len(sub) > 0 {
		merged := make([]yamlLine, 0, len(sub)+1)
		if rest != "" {
			merged = append(merged, yamlLine{indent: 0, text: rest})
		}
		for _, s := range sub {
			merged = append(merged, yamlLine{indent: 0, text: s.text})
		}
		return parseYAMLMap(merged)
	}
	return parseFlowValue("item", rest)
}

func parseFlowValue(ctx, raw string) (any, error) {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		inner := strings.TrimSpace(s[1 : len(s)-1])
		if inner == "" {
			return []any{}, nil
		}
		parts, err := splitTopLevelCommas(ctx, inner)
		if err != nil {
			return nil, err
		}
		out := make([]any, 0, len(parts))
		for _, p := range parts {
			v, err := parseFlowValue(ctx, p)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	}
	if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
		inner := strings.TrimSpace(s[1 : len(s)-1])
		out := map[string]any{}
		if inner == "" {
			return out, nil
		}
		parts, err := splitTopLevelCommas(ctx, inner)
		if err != nil {
			return nil, err
		}
		for _, p := range parts {
			k, v, ok := splitKeyValue(p)
			if !ok {
				return nil, fmt.Errorf("frontmatter: %s: expected key: value at %q", ctx, p)
			}
			pv, err := parseFlowValue(k, v)
			if err != nil {
				return nil, err
			}
			out[k] = pv
		}
		return out, nil
	}
	return unquoteScalar(s), nil
}

func splitTopLevelCommas(ctx, s string) ([]string, error) {
	var parts []string
	var depthBrace, depthBracket int
	var inSingle, inDouble bool
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\'' && !inDouble {
			inSingle = !inSingle
		} else if c == '"' && !inSingle && (i == 0 || s[i-1] != '\\') {
			inDouble = !inDouble
		} else if !inSingle && !inDouble {
			switch c {
			case '{':
				depthBrace++
			case '}':
				depthBrace--
			case '[':
				depthBracket++
			case ']':
				depthBracket--
			case ',':
				if depthBrace == 0 && depthBracket == 0 {
					parts = append(parts, strings.TrimSpace(s[start:i]))
					start = i + 1
				}
			}
			if depthBrace < 0 || depthBracket < 0 {
				return nil, fmt.Errorf("frontmatter: %s: unmatched delimiter in %q", ctx, s)
			}
		}
	}
	if inSingle || inDouble || depthBrace != 0 || depthBracket != 0 {
		return nil, fmt.Errorf("frontmatter: %s: unterminated delimiter in %q", ctx, s)
	}
	if tail := strings.TrimSpace(s[start:]); tail != "" {
		parts = append(parts, tail)
	}
	return parts, nil
}

func unquoteScalar(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		if u, err := strconv.Unquote(s); err == nil {
			return u
		}
		return s[1 : len(s)-1]
	}
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		return strings.ReplaceAll(s[1:len(s)-1], "''", "'")
	}
	return s
}

func decodeRecord(m map[string]any) (Record, error) {
	var rec Record
	var err error
	for k, v := range m {
		switch k {
		case "schema":
			rec.Schema, err = asString(k, v)
		case "id":
			rec.ID, err = asString(k, v)
		case "title":
			rec.Title, err = asString(k, v)
		case "level":
			rec.Level, err = asString(k, v)
		case "status":
			rec.Status, err = asString(k, v)
		case "selfheal":
			rec.SelfHeal, err = asString(k, v)
		case "fingerprint":
			rec.Fingerprint, err = asString(k, v)
		case "verified_at":
			rec.VerifiedAt, err = asString(k, v)
		case "signals":
			rec.Signals, err = asStringSlice(k, v)
		case "items":
			rec.Items, err = asStringSlice(k, v)
		case "prs":
			rec.PRs, err = asIntSlice(k, v)
		case "actions":
			rec.Actions, err = decodeActions(v)
		case "effectiveness":
			rec.Effectiveness, err = decodeEffectiveness(v)
		default:
			return Record{}, fmt.Errorf("frontmatter: unknown field %q", k)
		}
		if err != nil {
			return Record{}, err
		}
	}
	return rec, nil
}

func decodeActions(v any) ([]Action, error) {
	seq, err := asSeq("actions", v)
	if err != nil {
		return nil, err
	}
	out := make([]Action, 0, len(seq))
	for _, it := range seq {
		m, err := asMap("actions", it)
		if err != nil {
			return nil, err
		}
		var a Action
		for k, fv := range m {
			switch k {
			case "id":
				a.ID, err = asString("actions.id", fv)
			case "type":
				a.Type, err = asString("actions.type", fv)
			case "owner":
				a.Owner, err = asString("actions.owner", fv)
			case "item":
				a.Item, err = asString("actions.item", fv)
			case "done":
				a.Done, err = asString("actions.done", fv)
			case "state":
				a.State, err = asString("actions.state", fv)
			case "pr":
				a.PR, err = asInt("actions.pr", fv)
			default:
				return nil, fmt.Errorf("frontmatter: actions: unknown field %q", k)
			}
			if err != nil {
				return nil, err
			}
		}
		out = append(out, a)
	}
	return out, nil
}

func decodeEffectiveness(v any) (Effectiveness, error) {
	var eff Effectiveness
	m, err := asMap("effectiveness", v)
	if err != nil {
		return eff, err
	}
	for k, fv := range m {
		switch k {
		case "metric":
			eff.Metric, err = decodeMetric(fv)
		case "baseline":
			eff.Baseline, err = decodeBaseline(fv)
		case "target":
			eff.Target, err = asInt("effectiveness.target", fv)
		case "checks":
			eff.Checks, err = asInt("effectiveness.checks", fv)
		case "results":
			eff.Results, err = decodeResults(fv)
		default:
			return eff, fmt.Errorf("frontmatter: effectiveness: unknown field %q", k)
		}
		if err != nil {
			return eff, err
		}
	}
	return eff, nil
}

func decodeMetric(v any) (Metric, error) {
	var metric Metric
	m, err := asMap("effectiveness.metric", v)
	if err != nil {
		return metric, err
	}
	for k, fv := range m {
		switch k {
		case "code":
			metric.Code, err = asString("effectiveness.metric.code", fv)
		case "unit":
			metric.Unit, err = asString("effectiveness.metric.unit", fv)
		case "since":
			metric.Since, err = asString("effectiveness.metric.since", fv)
		case "filters":
			metric.Filters, err = decodeFilters(fv)
		default:
			return metric, fmt.Errorf("frontmatter: effectiveness.metric: unknown field %q", k)
		}
		if err != nil {
			return metric, err
		}
	}
	return metric, nil
}

func decodeFilters(v any) ([]Filter, error) {
	seq, err := asSeq("effectiveness.metric.filters", v)
	if err != nil {
		return nil, err
	}
	out := make([]Filter, 0, len(seq))
	for _, it := range seq {
		m, err := asMap("effectiveness.metric.filters", it)
		if err != nil {
			return nil, err
		}
		var f Filter
		for k, fv := range m {
			switch k {
			case "field":
				f.Field, err = asString("effectiveness.metric.filters.field", fv)
			case "op":
				f.Op, err = asString("effectiveness.metric.filters.op", fv)
			case "value":
				f.Value, err = asString("effectiveness.metric.filters.value", fv)
			default:
				return nil, fmt.Errorf("frontmatter: effectiveness.metric.filters: unknown field %q", k)
			}
			if err != nil {
				return nil, err
			}
		}
		out = append(out, f)
	}
	return out, nil
}

func decodeBaseline(v any) (Baseline, error) {
	var b Baseline
	m, err := asMap("effectiveness.baseline", v)
	if err != nil {
		return b, err
	}
	for k, fv := range m {
		switch k {
		case "value":
			b.Value, err = asInt("effectiveness.baseline.value", fv)
		case "units":
			b.Units, err = asInt("effectiveness.baseline.units", fv)
		default:
			return b, fmt.Errorf("frontmatter: effectiveness.baseline: unknown field %q", k)
		}
		if err != nil {
			return b, err
		}
	}
	return b, nil
}

func decodeResults(v any) ([]CheckResult, error) {
	seq, err := asSeq("effectiveness.results", v)
	if err != nil {
		return nil, err
	}
	out := make([]CheckResult, 0, len(seq))
	for _, it := range seq {
		m, err := asMap("effectiveness.results", it)
		if err != nil {
			return nil, err
		}
		var r CheckResult
		for k, fv := range m {
			switch k {
			case "check":
				r.Check, err = asInt("effectiveness.results.check", fv)
			case "observed":
				r.Observed, err = asInt("effectiveness.results.observed", fv)
			case "verdict":
				r.Verdict, err = asString("effectiveness.results.verdict", fv)
			case "at":
				r.At, err = asString("effectiveness.results.at", fv)
			default:
				return nil, fmt.Errorf("frontmatter: effectiveness.results: unknown field %q", k)
			}
			if err != nil {
				return nil, err
			}
		}
		out = append(out, r)
	}
	return out, nil
}

func asString(ctx string, v any) (string, error) {
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("frontmatter: %s: expected string", ctx)
	}
	return s, nil
}

func asInt(ctx string, v any) (int, error) {
	s, err := asString(ctx, v)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("frontmatter: %s: invalid integer %q", ctx, s)
	}
	return n, nil
}

func asStringSlice(ctx string, v any) ([]string, error) {
	seq, err := asSeq(ctx, v)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(seq))
	for _, it := range seq {
		s, err := asString(ctx, it)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func asIntSlice(ctx string, v any) ([]int, error) {
	seq, err := asSeq(ctx, v)
	if err != nil {
		return nil, err
	}
	out := make([]int, 0, len(seq))
	for _, it := range seq {
		n, err := asInt(ctx, it)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

func asSeq(ctx string, v any) ([]any, error) {
	seq, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("frontmatter: %s: expected sequence", ctx)
	}
	return seq, nil
}

func asMap(ctx string, v any) (map[string]any, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("frontmatter: %s: expected mapping", ctx)
	}
	return m, nil
}
