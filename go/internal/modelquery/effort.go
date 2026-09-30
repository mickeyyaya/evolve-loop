package modelquery

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

type EffortLister interface {
	ListEfforts(ctx context.Context, cli string) ([]string, error)
}

type HelpEffortLister struct {
	Run Runner
}

var effortToken = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

var effortFlag = regexp.MustCompile(`^--effort(?:[\s=]|$)`)

var flagStart = regexp.MustCompile(`^--?[A-Za-z]`)

func (l HelpEffortLister) ListEfforts(ctx context.Context, cli string) ([]string, error) {
	run := l.Run
	if run == nil {
		run = defaultRunner
	}
	out, err := run(ctx, cli, []string{"--help"}, "")
	if err != nil {
		return nil, fmt.Errorf("%s --help: %w", cli, err)
	}
	return parseEffortEnum(out), nil
}

func parseEffortEnum(help string) []string {
	lines := strings.Split(help, "\n")
	start := -1
	for i, ln := range lines {
		if effortFlag.MatchString(strings.TrimSpace(ln)) {
			start = i
			break
		}
	}
	if start < 0 {
		return nil
	}
	block := []string{lines[start]}
	for _, ln := range lines[start+1:] {
		t := strings.TrimSpace(ln)
		if t == "" || flagStart.MatchString(t) {
			break
		}
		block = append(block, t)
	}
	return firstEnumIn(strings.Join(block, " "))
}

func firstEnumIn(s string) []string {
	for {
		open := strings.Index(s, "(")
		if open < 0 {
			return nil
		}
		closeIdx := strings.Index(s[open:], ")")
		if closeIdx < 0 {
			return nil
		}
		inner := s[open+1 : open+closeIdx]
		if toks := enumTokens(inner); len(toks) > 0 {
			return toks
		}
		s = s[open+closeIdx+1:]
	}
}

func enumTokens(inner string) []string {
	fields := strings.FieldsFunc(inner, func(r rune) bool { return r == ',' || r == '|' })
	if len(fields) < 2 {
		return nil
	}
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		t := strings.TrimSpace(f)
		if !effortToken.MatchString(t) {
			return nil
		}
		out = append(out, t)
	}
	return out
}

func DefaultEffortListers() map[string]EffortLister {
	return map[string]EffortLister{
		"claude": HelpEffortLister{},
		"agy":    HelpEffortLister{},
	}
}
