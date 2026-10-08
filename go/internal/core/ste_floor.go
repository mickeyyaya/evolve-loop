package core

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
)

func steLintWarn(worktree, stageWord string, paths []string) {
	if stage, _ := config.GateStage(stageWord); stage == config.StageOff || worktree == "" {
		return
	}
	for _, line := range steLintFloorLines(worktree, paths) {
		fmt.Fprintln(os.Stderr, line)
	}
}

func steLintFloorLines(worktree string, paths []string) []string {
	var docs []string
	for _, p := range paths {
		if stelint.InDocsScope(p) {
			docs = append(docs, p)
		}
	}
	if len(docs) == 0 {
		return nil
	}
	words, err := stelint.LoadStandard(worktree)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return []string{fmt.Sprintf("[ste-lint] WARN: the lint did not run: %v", err)}
	}
	return steFindingLines(worktree, docs, stelint.Options{Words: words})
}

func steFindingLines(worktree string, docs []string, opt stelint.Options) []string {
	var lines []string
	findings, files, first := 0, 0, ""
	for _, p := range docs {
		fileFindings, err := stelint.LintFile(filepath.Join(worktree, filepath.FromSlash(p)), opt)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			lines = append(lines, fmt.Sprintf("[ste-lint] WARN: %s was not checked: %v", p, err))
			continue
		}
		counted := 0
		for _, f := range fileFindings {
			if f.IsSkipNotice() {
				continue
			}
			if first == "" {
				first = fmt.Sprintf("%s:%d %s", p, f.Line, f.Rule)
			}
			counted++
		}
		if counted > 0 {
			findings += counted
			files++
		}
	}
	if findings > 0 {
		lines = append(lines, fmt.Sprintf("[ste-lint] WARN: %d finding(s) in %d file(s) (first: %s)", findings, files, first))
	}
	return lines
}
