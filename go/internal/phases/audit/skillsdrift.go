package audit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/audit/ciparitygate"
	"github.com/mickeyyaya/evolve-loop/go/internal/skillcheck"
)

const skillsGeneratorPrefix = "go/internal/skillcheck/"

var worktreeSkillsCheckLimit = ciparitygate.DefaultTimeouts().GoVet

type gateWarning string

func (w gateWarning) Error() string { return string(w) }

func skillsDriftCheckDefault(req core.PhaseRequest) ([]string, error) {
	root := req.Worktree
	if root == "" {
		root = req.ProjectRoot
	}
	if root == "" {
		return nil, nil
	}
	if carriesSkillsGenerator(root) {
		return worktreeSkillsDrift(context.Background(), root)
	}
	return skillcheck.Check(root)
}

func carriesSkillsGenerator(root string) bool {
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(skillsGeneratorPrefix)))
	return err == nil && info.IsDir()
}

func worktreeSkillsDrift(parent context.Context, root string) ([]string, error) {
	inv := core.WorktreeEvolveInvocation(root, "skills", "check")
	ctx, cancel := context.WithTimeout(parent, worktreeSkillsCheckLimit)
	defer cancel()
	var report strings.Builder
	code, err := runCmd(ctx, "go", inv.Dir, inv.Args, inv.Env, nil, nil, &report)
	if offenders := skillsCheckOffenders(report.String()); code != 0 && len(offenders) > 0 {
		return offenders, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, laneNotGraded(ctxErr)
	}
	if err != nil {
		return nil, laneNotGraded(err)
	}
	if code != 0 {
		return worktreeFailureOffenders(code, report.String()), nil
	}
	host, hostErr := skillcheck.Check(root)
	return nil, generatorDisagreement(host, hostErr)
}

func laneNotGraded(cause error) error {
	return fmt.Errorf("the worktree generator `evolve skills check` did not run: the lane is NOT graded and only CI TestSkills_NoDrift checks it: %w", cause)
}

func worktreeFailureOffenders(code int, report string) []string {
	if offenders := skillsCheckOffenders(report); len(offenders) > 0 {
		return offenders
	}
	out := strings.TrimSpace(report)
	if out == "" {
		out = "(no output)"
	}
	return []string{fmt.Sprintf("the worktree generator skills check exited %d without a drift report: %s", code, out)}
}

func generatorDisagreement(host []string, hostErr error) error {
	switch {
	case hostErr != nil:
		return gateWarning("the worktree generator graded the lane clean, but the host generator could not compare (" + hostErr.Error() + "); the worktree generator grades the lane")
	case len(host) > 0:
		return gateWarning(fmt.Sprintf("the worktree generator and the host generator disagree on %d artifact(s); the worktree generator grades the lane: %s", len(host), strings.Join(host, ", ")))
	}
	return nil
}

func skillsCheckOffenders(report string) []string {
	var offenders []string
	for _, line := range strings.Split(report, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "DRIFT: "):
			artifact := strings.Fields(strings.TrimPrefix(line, "DRIFT: "))[0]
			offenders = append(offenders, strings.TrimSuffix(artifact, ":"))
		case strings.HasPrefix(line, "MANIFEST: "):
			offenders = append(offenders, line)
		}
	}
	return offenders
}
