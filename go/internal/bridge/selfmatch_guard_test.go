package bridge

import (
	"os"
	"regexp/syntax"
	"strings"
	"testing"
)

const selfMatchPaneDepth = 64

type paneFire struct {
	bottom int
	action string
	rc     int
}

func requireNoRuleFiresAtAnyPaneBottom(t *testing.T, rules []ManifestPrompt, files []string) {
	t.Helper()
	for _, p := range rules {
		if p.TailLines <= 0 || p.TailLines > selfMatchPaneDepth {
			t.Fatalf("rule %s has tail_lines %d; a %d-line pane must hold every rule's whole tail, or this guard sees less than the rule does", p.Name, p.TailLines, selfMatchPaneDepth)
		}
	}
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v (this guard is worthless if it cannot read its own sources)", f, err)
		}
		if fires := firesAtPaneBottoms(string(body), rules); len(fires) > 0 {
			t.Fatalf("(%q, rc %d) on a pane whose bottom line is %s:%d: an agent that prints this tracked file can stop its output there, so its text must never take the live dialog's shape", fires[0].action, fires[0].rc, f, fires[0].bottom+1)
		}
	}
}

func firesAtPaneBottoms(body string, rules []ManifestPrompt) []paneFire {
	starts := lineStarts(body)
	needles := make([][]string, len(rules))
	lastHit := make([]int, len(rules))
	for i, p := range rules {
		needles[i] = requiredLiterals(p.Regex)
		lastHit[i] = -selfMatchPaneDepth
	}
	var fires []paneFire
	for end := range starts {
		line := body[starts[end]:lineEnd(body, starts, end)]
		var live []ManifestPrompt
		for i, p := range rules {
			if needles[i] == nil || containsAny(line, needles[i]) {
				lastHit[i] = end
			}
			if end-lastHit[i] < selfMatchPaneDepth {
				live = append(live, p)
			}
		}
		if len(live) == 0 {
			continue
		}
		pane := body[starts[max(0, end+1-selfMatchPaneDepth)]:lineEnd(body, starts, end)]
		if a, rc := decideAutoRespond(pane, live, map[string]int{}, false); rc != 0 || a != "noop" {
			fires = append(fires, paneFire{bottom: end, action: a, rc: rc})
		}
	}
	return fires
}

func firesAtPaneBottomsExhaustively(body string, rules []ManifestPrompt) []paneFire {
	lines := strings.Split(body, "\n")
	var fires []paneFire
	for end := range lines {
		pane := strings.Join(lines[max(0, end+1-selfMatchPaneDepth):end+1], "\n")
		if a, rc := decideAutoRespond(pane, rules, map[string]int{}, false); rc != 0 || a != "noop" {
			fires = append(fires, paneFire{bottom: end, action: a, rc: rc})
		}
	}
	return fires
}

func lineStarts(body string) []int {
	starts := []int{0}
	for i := 0; i < len(body); i++ {
		if body[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

func lineEnd(body string, starts []int, line int) int {
	if line+1 < len(starts) {
		return starts[line+1] - 1
	}
	return len(body)
}

func containsAny(s string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}

func requiredLiterals(expr string) []string {
	re, err := syntax.Parse(expr, syntax.Perl)
	if err != nil {
		return nil
	}
	return mustContain(re.Simplify())
}

func mustContain(re *syntax.Regexp) []string {
	switch re.Op {
	case syntax.OpLiteral:
		s := string(re.Rune)
		if re.Flags&syntax.FoldCase != 0 || strings.Contains(s, "\n") {
			return nil
		}
		return []string{s}
	case syntax.OpCapture, syntax.OpPlus:
		return mustContain(re.Sub[0])
	case syntax.OpRepeat:
		if re.Min >= 1 {
			return mustContain(re.Sub[0])
		}
	case syntax.OpConcat:
		var best []string
		for _, sub := range re.Sub {
			if c := mustContain(sub); c != nil && shortestLen(c) > shortestLen(best) {
				best = c
			}
		}
		return best
	case syntax.OpAlternate:
		var all []string
		for _, sub := range re.Sub {
			c := mustContain(sub)
			if c == nil {
				return nil
			}
			all = append(all, c...)
		}
		return all
	}
	return nil
}

func shortestLen(set []string) int {
	if len(set) == 0 {
		return 0
	}
	n := len(set[0])
	for _, s := range set[1:] {
		n = min(n, len(s))
	}
	return n
}
