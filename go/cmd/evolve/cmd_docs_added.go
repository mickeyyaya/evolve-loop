package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
)

var (
	steParseDiff = covchanged.ParseDiff
	steReadFile  = os.ReadFile
)

var steListItem = regexp.MustCompile(`^\s*([-*+]|\d{1,9}[.)])\s+`)

func scopeToAddedLines(root, base string, targets []steTarget, untracked map[string]bool) ([]steTarget, error) {
	out, errOut, code, err := gitexec.Default(root).Capture(context.Background(),
		"-c", "core.quotePath=false", "diff", "-U0", "--no-color", "--no-ext-diff", "--no-renames", "--relative", base, "--")
	if err == nil && code != 0 {
		err = fmt.Errorf("exit %d: %s", code, strings.TrimSpace(errOut))
	}
	if err != nil {
		return nil, fmt.Errorf("git diff -U0 %s: %w", base, err)
	}
	diff, err := steParseDiff([]byte(out))
	if err != nil {
		return nil, err
	}
	scoped := make([]steTarget, 0, len(targets))
	for _, t := range targets {
		if !untracked[t.display] {
			t.added = map[int]bool{}
			for _, n := range diff.Added[t.display] {
				t.added[n] = true
			}
		}
		scoped = append(scoped, t)
	}
	return scoped, nil
}

func keepAddedFindings(t steTarget, findings []stelint.Finding) ([]stelint.Finding, error) {
	if t.added == nil {
		return findings, nil
	}
	raw, err := steReadFile(t.abs)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(raw), "\n")
	exactLines := strings.HasSuffix(t.abs, ".go")
	kept := make([]stelint.Finding, 0, len(findings))
	for _, f := range findings {
		from, to := f.Line, f.Line
		if !exactLines {
			from, to = steBlockSpan(lines, f.Line)
		}
		if f.IsSkipNotice() || f.Line < 1 || addedWithin(t.added, from, to) {
			kept = append(kept, f)
		}
	}
	return kept, nil
}

func addedWithin(added map[int]bool, from, to int) bool {
	for n := from; n <= to; n++ {
		if added[n] {
			return true
		}
	}
	return false
}

func steBlockSpan(lines []string, line int) (int, int) {
	from, to := line, line
	if line < 1 || line > len(lines) || steBlockEdge(lines[line-1]) {
		return from, to
	}
	for from > 1 && !steListItem.MatchString(lines[from-1]) && !steBlockEdge(lines[from-2]) {
		from--
	}
	for to < len(lines) && !steListItem.MatchString(lines[to]) && !steBlockEdge(lines[to]) {
		to++
	}
	return from, to
}

func steBlockEdge(line string) bool {
	t := strings.TrimSpace(line)
	return t == "" || strings.HasPrefix(t, "|") || strings.HasPrefix(t, "#")
}
