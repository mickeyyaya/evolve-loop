package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
)

var (
	steParseDiff = covchanged.ParseDiff
	steReadFile  = os.ReadFile
)

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
	return stelint.KeepAdded(raw, strings.HasSuffix(t.abs, ".go"), t.added, findings), nil
}
