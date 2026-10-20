package core

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
)

const steLintRemedy = "fix each finding; `evolve docs ste-lint --strict --changed <base> --added` lists them"

func steLintFloor(worktree, stageWord string, paths []string, warn io.Writer) []string {
	stage, _ := config.GateStage(stageWord)
	if stage == config.StageOff || worktree == "" {
		return nil
	}
	warnings, finding := steLintFloorLines(worktree, paths)
	for _, line := range warnings {
		fmt.Fprintln(warn, line)
	}
	switch {
	case finding == "":
		return nil
	case stage == config.StageEnforce:
		return []string{"[ste-lint] the changed documents fail the STE lint: " + finding + "; " + steLintRemedy}
	default:
		fmt.Fprintln(warn, "[ste-lint] WARN: "+finding)
		return nil
	}
}

func steLintFloorLines(worktree string, paths []string) (warnings []string, finding string) {
	var docs []string
	for _, p := range paths {
		if stelint.InDocsScope(p) {
			docs = append(docs, p)
		}
	}
	if len(docs) == 0 {
		return nil, ""
	}
	words, err := stelint.LoadStandard(worktree)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ""
	}
	if err != nil {
		return []string{fmt.Sprintf("[ste-lint] WARN: the lint did not run: %v", err)}, ""
	}
	return steFindingLines(worktree, docs, stelint.Options{Words: words})
}

func steFindingLines(worktree string, docs []string, opt stelint.Options) (lines []string, finding string) {
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
		finding = fmt.Sprintf("%d finding(s) in %d file(s) (first: %s)", findings, files, first)
	}
	return lines, finding
}
