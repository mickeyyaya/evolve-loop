package filter

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var operators = []string{"!=", ">=", "<=", "=", ">", "<"}

type term struct {
	key    keySpec
	op     string
	values []value
}

func Parse(expr string, cat Catalog) (Filter, []Warning, error) {
	var terms []term
	var warnings []Warning
	for _, raw := range strings.Fields(expr) {
		t, w, err := parseTerm(raw, cat)
		if err != nil {
			return Filter{}, nil, err
		}
		terms = append(terms, t)
		warnings = append(warnings, w...)
	}
	return Filter{terms: terms}, warnings, nil
}

func parseTerm(raw string, cat Catalog) (term, []Warning, error) {
	name, op, rest, err := splitTerm(raw)
	if err != nil {
		return term{}, nil, err
	}
	spec, ok := lookupKey(name)
	if !ok {
		return term{}, nil, fmt.Errorf("%w: unknown key %q in term %q", ErrUsage, name, raw)
	}
	texts := strings.Split(rest, ",")
	if err := checkShape(spec, op, texts, raw); err != nil {
		return term{}, nil, err
	}
	values, err := parseValues(spec.class, texts, raw)
	if err != nil {
		return term{}, nil, err
	}
	warnings, err := checkVocabulary(spec, texts, cat, name)
	if err != nil {
		return term{}, nil, err
	}
	return term{key: spec, op: op, values: values}, warnings, nil
}

func splitTerm(raw string) (string, string, string, error) {
	i := strings.IndexAny(raw, "=!<>")
	if i > 0 {
		for _, op := range operators {
			if strings.HasPrefix(raw[i:], op) {
				return raw[:i], op, raw[i+len(op):], nil
			}
		}
	}
	return "", "", "", fmt.Errorf("%w: term %q is not KEY OP VALUE", ErrUsage, raw)
}

func checkShape(spec keySpec, op string, texts []string, raw string) error {
	isOrderOp := op != "=" && op != "!="
	switch {
	case isOrderOp && !spec.class.ordered():
		return fmt.Errorf("%w: term %q uses an order operator on a key without an order", ErrUsage, raw)
	case isOrderOp && len(texts) > 1:
		return fmt.Errorf("%w: term %q gives an order operator more than one value", ErrUsage, raw)
	}
	for _, text := range texts {
		if text == "" {
			return fmt.Errorf("%w: term %q has an empty value", ErrUsage, raw)
		}
		if strings.Contains(text, "*") && spec.class != globClass {
			return fmt.Errorf("%w: term %q has a glob, and only kind and code take a glob", ErrUsage, raw)
		}
	}
	return nil
}

func parseValues(class keyClass, texts []string, raw string) ([]value, error) {
	values := make([]value, len(texts))
	for i, text := range texts {
		v, err := parseValue(class, text)
		if err != nil {
			return nil, fmt.Errorf("%w: term %q: %v", ErrUsage, raw, err)
		}
		values[i] = v
	}
	return values, nil
}

func parseValue(class keyClass, text string) (value, error) {
	switch class {
	case numberClass:
		n, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return value{}, fmt.Errorf("value %q is not a number", text)
		}
		return value{text: text, rank: n}, nil
	case severityClass:
		sev := signalcenter.Severity(text)
		if !sev.Valid() {
			return value{}, fmt.Errorf("value %q is not INFO, WARN or INCIDENT", text)
		}
		return value{text: text, rank: int64(sev.Level())}, nil
	}
	return value{text: text}, nil
}

func checkVocabulary(spec keySpec, texts []string, cat Catalog, name string) ([]Warning, error) {
	if spec.onUnknown == unknownIsFree {
		return nil, nil
	}
	var warnings []Warning
	vocab := spec.vocab(cat)
	for _, text := range texts {
		if matchesAny(text, vocab) {
			continue
		}
		if spec.onUnknown == unknownIsRefused {
			return nil, fmt.Errorf("%w: %s %q matches no registered %s", ErrRefused, name, text, name)
		}
		warnings = append(warnings, Warning{Key: name, Value: text})
	}
	return warnings, nil
}

func matchesAny(pattern string, vocab []string) bool {
	for _, v := range vocab {
		if globMatch(pattern, v) {
			return true
		}
	}
	return false
}

func globMatch(pattern, s string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == s
	}
	if !strings.HasPrefix(s, parts[0]) {
		return false
	}
	rest := s[len(parts[0]):]
	for _, part := range parts[1 : len(parts)-1] {
		i := strings.Index(rest, part)
		if i < 0 {
			return false
		}
		rest = rest[i+len(part):]
	}
	return strings.HasSuffix(rest, parts[len(parts)-1])
}

func (t term) match(r Record) bool {
	got, present := t.key.read(r)
	switch t.op {
	case "=":
		return present && t.anyEqual(got)
	case "!=":
		return !present || !t.anyEqual(got)
	}
	return present && t.ordered(got)
}

func (t term) anyEqual(got value) bool {
	for _, want := range t.values {
		if t.key.class.equal(got, want) {
			return true
		}
	}
	return false
}

func (t term) ordered(got value) bool {
	c := cmp.Compare(got.rank, t.values[0].rank)
	switch t.op {
	case ">=":
		return c >= 0
	case ">":
		return c > 0
	case "<=":
		return c <= 0
	}
	return c < 0
}

type Warning struct {
	Key   string
	Value string
}

func (w Warning) String() string {
	return fmt.Sprintf("%s %q matches no registered %s", w.Key, w.Value, w.Key)
}
