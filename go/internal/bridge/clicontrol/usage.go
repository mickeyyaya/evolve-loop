package clicontrol

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type Window struct {
	Name        string
	PercentLeft int
	ResetHint   string
}

type Usage struct {
	Family  string
	Windows []Window
	Parsed  bool
	Raw     string
}

const unknownPercentLeft = -1

func (u Usage) MinPercentLeft() int {
	if !u.Parsed || len(u.Windows) == 0 {
		return unknownPercentLeft
	}
	min := u.Windows[0].PercentLeft
	for _, w := range u.Windows[1:] {
		if w.PercentLeft < min {
			min = w.PercentLeft
		}
	}
	return min
}

type UsageParser func(pane string) (windows []Window, ok bool)

var usageParsers = map[string]UsageParser{
	"codex":  parseCodexUsage,
	"claude": parseClaudeUsage,
}

var codexUsageLine = regexp.MustCompile(`([A-Za-z0-9.\- ]*?limit):\s*(?:\[[^\]]*\]\s*)?(\d{1,3})%\s*left\s*(\([^)]*\))?`)

func parseCodexUsage(pane string) ([]Window, bool) {
	var out []Window
	for _, m := range codexUsageLine.FindAllStringSubmatch(pane, -1) {
		pct, err := strconv.Atoi(m[2])
		if err != nil || pct < 0 || pct > 100 {
			continue
		}
		out = append(out, Window{
			Name:        strings.TrimSpace(m[1]),
			PercentLeft: pct,
			ResetHint:   strings.Trim(strings.TrimSpace(m[3]), "()"),
		})
	}
	return out, len(out) > 0
}

var claudeUsageLine = regexp.MustCompile(`used\s+(\d{1,3})%\s+of\s+your\s+(\w+)\s+limit(?:\s*[·|-]\s*(resets[^\n]*))?`)

func parseClaudeUsage(pane string) ([]Window, bool) {
	m := claudeUsageLine.FindStringSubmatch(pane)
	if m == nil {
		return nil, false
	}
	used, err := strconv.Atoi(m[1])
	if err != nil || used < 0 || used > 100 {
		return nil, false
	}
	return []Window{{
		Name:        m[2] + " limit",
		PercentLeft: 100 - used,
		ResetHint:   strings.TrimSpace(m[3]),
	}}, true
}

func ParseUsage(family, pane string) (Usage, bool) {
	u := Usage{Family: family, Raw: pane}
	parse, ok := usageParsers[family]
	if !ok {
		return u, false
	}
	windows, ok := parse(pane)
	if !ok {
		return u, false
	}
	u.Windows, u.Parsed = windows, true
	return u, true
}

func QueryUsage(ctx context.Context, ctrl Controller, family string) (Usage, error) {
	if ctrl == nil {
		return Usage{Family: family}, fmt.Errorf("clicontrol: QueryUsage requires a Controller")
	}
	resp, err := ctrl.Do(ctx, family, EventUsage)
	if err != nil {
		return Usage{Family: family}, err
	}
	u, _ := ParseUsage(family, resp.Pane)
	return u, nil
}
