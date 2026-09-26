package advisor

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// writeRubricLines projects the routing config the kernel walks, so a threshold cannot disagree between the walk
// and the prompt. A conditional rule says when a phase is pinned, so its op is negated to say when it may be skipped.
func writeRubricLines(b *strings.Builder, cfg config.RoutingConfig) {
	seen := make(map[string]struct{}, len(cfg.Triggers)+len(cfg.Conditional))
	for p := range cfg.Conditional {
		seen[p] = struct{}{}
	}
	for p := range cfg.Triggers {
		seen[p] = struct{}{}
	}
	names := make([]string, 0, len(seen))
	for p := range seen {
		names = append(names, p)
	}
	sort.Strings(names)
	for _, p := range names {
		if rule, ok := cfg.Conditional[p]; ok {
			// One exemption line per clause: the rule pins only while EVERY
			// clause holds, so the negation of any single clause is a release.
			for _, c := range rule.Clauses() {
				if op, ok := negateOp(c.Op); ok {
					fmt.Fprintf(b, "- %s %s %s → skip %s (conditional-mandatory exemption)\n", c.Field, op, c.Value, p)
				}
			}
		}
		blk := cfg.Triggers[p]
		if len(blk.InsertWhen) > 0 {
			clauses := make([]string, len(blk.InsertWhen))
			for i, c := range blk.InsertWhen {
				clauses[i] = fmt.Sprintf("%s %s %v", c.Field, opSymbol(c.Op), c.Value)
			}
			fmt.Fprintf(b, "- %s → insert %s\n", strings.Join(clauses, " OR "), p)
		}
		for _, hint := range blk.RubricHint {
			fmt.Fprintf(b, "- %s\n", hint)
		}
	}
}

func opSymbol(op string) string {
	switch op {
	case "eq":
		return "=="
	case "ne":
		return "!="
	case "gt":
		return ">"
	case "gte":
		return ">="
	case "lt":
		return "<"
	case "lte":
		return "<="
	}
	return op
}

// negateOp also accepts word-form ops so an in-process caller cannot lose an exemption line; unknown ops yield none.
func negateOp(op string) (string, bool) {
	switch op {
	case "==", "eq":
		return "!=", true
	case "!=", "ne":
		return "==", true
	case ">", "gt":
		return "<=", true
	case ">=", "gte":
		return "<", true
	case "<", "lt":
		return ">=", true
	case "<=", "lte":
		return ">", true
	}
	return "", false
}
