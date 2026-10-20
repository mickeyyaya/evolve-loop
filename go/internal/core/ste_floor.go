package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
)

const steLintRemedy = "fix each finding; `evolve docs ste-lint --strict --changed <base> --added` lists them"

type steKeep func(path string, findings []stelint.Finding) ([]stelint.Finding, error)

func steLintFloor(ctx context.Context, worktree, base, stageWord string, paths []string, warn io.Writer) []string {
	stage, _ := config.GateStage(stageWord)
	if stage == config.StageOff || worktree == "" {
		return nil
	}
	var keep steKeep
	if stage == config.StageEnforce {
		keep = steAddedKeep(ctx, gitexec.Git{Dir: worktree, Exec: gitRunner}, worktree, base)
	}
	warnings, finding := steLintFloorLines(worktree, paths, keep)
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

func steLintFloorLines(worktree string, paths []string, keep steKeep) (warnings []string, finding string) {
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
	return steFindingLines(worktree, docs, stelint.Options{Words: words}, keep)
}

func steFindingLines(worktree string, docs []string, opt stelint.Options, keep steKeep) (lines []string, finding string) {
	findings, files, first := 0, 0, ""
	for _, p := range docs {
		fileFindings, err := stelint.LintFile(filepath.Join(worktree, filepath.FromSlash(p)), opt)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err == nil && keep != nil {
			if scoped, kerr := keep(p, fileFindings); kerr != nil {
				lines = append(lines, fmt.Sprintf("[ste-lint] WARN: the added lines of %s could not be read (%v); the floor judges the whole file", p, kerr))
			} else {
				fileFindings = scoped
			}
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

func steAddedKeep(ctx context.Context, git gitexec.Git, worktree, base string) steKeep {
	return func(path string, findings []stelint.Finding) ([]stelint.Finding, error) {
		_, _, code, err := git.Capture(ctx, "cat-file", "-e", base+":"+path)
		if err != nil {
			return nil, err
		}
		if code != 0 {
			return findings, nil
		}
		out, errOut, code, err := git.Capture(ctx, "-c", "core.quotePath=false", "diff", "-U0", "--no-color", "--no-ext-diff", "--no-renames", base, "--", path)
		if err == nil && code != 0 {
			err = fmt.Errorf("git diff -U0 %s exit %d: %s", base, code, strings.TrimSpace(errOut))
		}
		if err != nil {
			return nil, err
		}
		diff, err := covchanged.ParseDiff([]byte(out))
		if err != nil {
			return nil, err
		}
		added := map[int]bool{}
		for _, n := range diff.Added[path] {
			added[n] = true
		}
		raw, err := os.ReadFile(filepath.Join(worktree, filepath.FromSlash(path)))
		if err != nil {
			return nil, err
		}
		return stelint.KeepAdded(raw, false, added, findings), nil
	}
}
