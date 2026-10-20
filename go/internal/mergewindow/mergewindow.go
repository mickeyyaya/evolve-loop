package mergewindow

import (
	"fmt"
	"regexp"
	"strings"
)

type Class string

const (
	Window   Class = "window"
	Boundary Class = "boundary"
)

const (
	ciOnlyPrefix  = ".github/"
	pathLoopStart = "while IFS= read -r -d '' path; do"
	pathLoopEnd   = "done <"
	blockStart    = `case "$path" in`
	blockEnd      = "esac"
	docsFlag      = "docs_found"
	otherFlag     = "other_found"
	goFlag        = "go_required"
	landingFlag   = "landing_required"
	inboxFlag     = "inbox_found"
)

var (
	armLine    = regexp.MustCompile(`^(\S+?)\)\s*(.*?)\s*;;$`)
	actionWord = regexp.MustCompile(`^([a-z_]+)=true$`)
	blockRules = [2]blockRule{
		{known: map[string]bool{docsFlag: true, otherFlag: true}, needs: []string{docsFlag}, catchAllSets: []string{otherFlag}},
		{known: map[string]bool{goFlag: true, landingFlag: true, inboxFlag: true}, catchAllSets: []string{goFlag, landingFlag}},
	}
)

type blockRule struct {
	known        map[string]bool
	needs        []string
	catchAllSets []string
}

type arm struct {
	patterns []*regexp.Regexp
	sets     map[string]bool
	catchAll bool
}

type Rule struct {
	classes []arm
	suites  []arm
}

type Verdict struct {
	Class         Class
	BoundaryPaths []string
}

func Parse(workflow []byte) (Rule, error) {
	lines, err := pathLoop(string(workflow))
	if err != nil {
		return Rule{}, err
	}
	blocks, err := caseBlocks(lines)
	if err != nil {
		return Rule{}, err
	}
	for i, block := range blocks {
		if err := blockRules[i].check(block); err != nil {
			return Rule{}, fmt.Errorf("required.yml case block %d: %w", i+1, err)
		}
	}
	return Rule{classes: blocks[0], suites: blocks[1]}, nil
}

func pathLoop(text string) ([]string, error) {
	if n := strings.Count(text, pathLoopStart); n != 1 {
		return nil, fmt.Errorf("required.yml: want one path loop %q, found %d", pathLoopStart, n)
	}
	_, body, _ := strings.Cut(text, pathLoopStart)
	var lines []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, pathLoopEnd) {
			return lines, nil
		}
		lines = append(lines, line)
	}
	return nil, fmt.Errorf("required.yml: no path loop end %q", pathLoopEnd)
}

func caseBlocks(lines []string) ([][]arm, error) {
	var blocks [][]arm
	var current []arm
	open := false
	for _, line := range lines {
		switch {
		case line == blockStart:
			open, current = true, nil
		case line == blockEnd && open:
			open, blocks = false, append(blocks, current)
		case open:
			a, err := parseArm(line)
			if err != nil {
				return nil, err
			}
			current = append(current, a)
		}
	}
	if open || len(blocks) != 2 {
		return nil, fmt.Errorf("required.yml: the path loop must have 2 case blocks, found %d complete", len(blocks))
	}
	return blocks, nil
}

func parseArm(line string) (arm, error) {
	m := armLine.FindStringSubmatch(line)
	if m == nil {
		return arm{}, fmt.Errorf("required.yml: unreadable arm %q", line)
	}
	a := arm{sets: map[string]bool{}}
	for _, glob := range strings.Split(m[1], "|") {
		a.catchAll = a.catchAll || glob == "*"
		re, err := globRegexp(glob)
		if err != nil {
			return arm{}, err
		}
		a.patterns = append(a.patterns, re)
	}
	for _, action := range strings.Split(m[2], ";") {
		action = strings.TrimSpace(action)
		if action == "" {
			continue
		}
		w := actionWord.FindStringSubmatch(action)
		if w == nil {
			return arm{}, fmt.Errorf("required.yml: unreadable action %q in %q", action, line)
		}
		a.sets[w[1]] = true
	}
	return a, nil
}

func globRegexp(glob string) (*regexp.Regexp, error) {
	if glob == "" || strings.ContainsAny(glob, "?[]\\\"'$") {
		return nil, fmt.Errorf("required.yml: unsupported glob %q", glob)
	}
	parts := strings.Split(glob, "*")
	for i, p := range parts {
		parts[i] = regexp.QuoteMeta(p)
	}
	return regexp.MustCompile("(?s)^" + strings.Join(parts, ".*") + "$"), nil
}

func (rule blockRule) check(block []arm) error {
	last := len(block) - 1
	if last < 0 || !block[last].catchAll {
		return fmt.Errorf("no catch-all arm `*)` at the end")
	}
	for _, flag := range rule.catchAllSets {
		if !block[last].sets[flag] {
			return fmt.Errorf("the catch-all arm must set %s", flag)
		}
	}
	seen := map[string]bool{}
	for i, a := range block {
		if a.catchAll && i != last {
			return fmt.Errorf("a catch-all arm before the end hides arm %d", i+2)
		}
		for flag := range a.sets {
			if !rule.known[flag] {
				return fmt.Errorf("unknown flag %s", flag)
			}
			seen[flag] = true
		}
	}
	for _, flag := range rule.needs {
		if !seen[flag] {
			return fmt.Errorf("the block never sets %s", flag)
		}
	}
	return nil
}

func (a arm) matches(path string) bool {
	for _, re := range a.patterns {
		if re.MatchString(path) {
			return true
		}
	}
	return false
}

func firstMatch(arms []arm, path string) map[string]bool {
	last := len(arms) - 1
	for _, a := range arms[:last] {
		if a.matches(path) {
			return a.sets
		}
	}
	return arms[last].sets
}

func (r Rule) pathClass(path string) Class {
	if strings.HasPrefix(path, ciOnlyPrefix) {
		return Window
	}
	if c := firstMatch(r.classes, path); c[docsFlag] && !c[otherFlag] {
		return Window
	}
	if s := firstMatch(r.suites, path); !s[goFlag] && !s[landingFlag] {
		return Window
	}
	return Boundary
}

func (r Rule) Classify(paths []string) Verdict {
	if len(paths) == 0 {
		return Verdict{Class: Boundary}
	}
	var boundary []string
	seen := map[string]bool{}
	for _, p := range paths {
		if seen[p] {
			continue
		}
		seen[p] = true
		if r.pathClass(p) == Boundary {
			boundary = append(boundary, p)
		}
	}
	if len(boundary) > 0 {
		return Verdict{Class: Boundary, BoundaryPaths: boundary}
	}
	return Verdict{Class: Window}
}
