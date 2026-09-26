package looppreflight

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

// reconcileCommand is named in the halt so the stop comes with a next step.
const reconcileCommand = "evolve sync-main"

// baseDivergenceTimeout bounds the whole probe, fetch included, so a wedged remote cannot hang boot.
const baseDivergenceTimeout = 60 * time.Second

// baseState is the probe verdict; Skipped means there is nothing to compare, and Reason says why.
type baseState struct {
	Skipped bool
	Reason  string
	Branch  string
	Ahead   int
	Behind  int
}

// baseDivergenceProbe is a test seam, so the fast tier never shells out to git.
var baseDivergenceProbe = defaultBaseDivergenceProbe

// newGit is a test seam for scripted git replies.
var newGit = gitexec.Default

// defaultBaseDivergenceProbe compares HEAD with the freshly fetched origin branch;
// an error means the comparison is unverified.
func defaultBaseDivergenceProbe(ctx context.Context, projectRoot string) (baseState, error) {
	g := newGit(projectRoot)

	if _, err := g.Output(ctx, "rev-parse", "--is-inside-work-tree"); err != nil {
		return baseState{Skipped: true, Reason: "not a git work tree"}, nil
	}
	branch, err := g.Output(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return baseState{}, fmt.Errorf("resolve current branch: %w", err)
	}
	if branch == "" || branch == "HEAD" {
		return baseState{Skipped: true, Reason: "detached HEAD — no base branch to compare"}, nil
	}
	remotes, err := g.Output(ctx, "remote")
	if err != nil {
		return baseState{}, fmt.Errorf("list remotes: %w", err)
	}
	if !hasRemoteOrigin(remotes) {
		return baseState{Skipped: true, Reason: "no `origin` remote"}, nil
	}

	// Compare against the FETCH_HEAD this fetch writes: a local origin/<base> ref may itself be stale.
	if err := g.Run(ctx, "fetch", "origin", branch); err != nil {
		return baseState{}, fmt.Errorf("fetch origin %s: %w", branch, err)
	}
	counts, err := g.Output(ctx, "rev-list", "--left-right", "--count", "HEAD...FETCH_HEAD")
	if err != nil {
		return baseState{}, fmt.Errorf("count divergence against origin/%s: %w", branch, err)
	}
	ahead, behind, err := parseLeftRightCount(counts)
	if err != nil {
		return baseState{}, err
	}
	return baseState{Branch: branch, Ahead: ahead, Behind: behind}, nil
}

func hasRemoteOrigin(remotes string) bool {
	for _, r := range strings.Fields(remotes) {
		if r == "origin" {
			return true
		}
	}
	return false
}

// parseLeftRightCount parses the "<left>\t<right>" output of `rev-list --left-right --count HEAD...FETCH_HEAD`.
func parseLeftRightCount(out string) (ahead, behind int, err error) {
	fields := strings.Fields(out)
	if len(fields) != 2 {
		return 0, 0, fmt.Errorf("unexpected rev-list --count output %q", out)
	}
	if ahead, err = strconv.Atoi(fields[0]); err != nil {
		return 0, 0, fmt.Errorf("parse ahead count %q: %w", fields[0], err)
	}
	if behind, err = strconv.Atoi(fields[1]); err != nil {
		return 0, 0, fmt.Errorf("parse behind count %q: %w", fields[1], err)
	}
	return ahead, behind, nil
}

// checkBaseDivergence halts when the base is behind origin; ahead-only passes, an unverifiable comparison warns.
// See ADR-0081.
func checkBaseDivergence(o resolved) CheckResult {
	const name = "base-divergence"

	ctx, cancel := context.WithTimeout(context.Background(), baseDivergenceTimeout)
	defer cancel()

	st, err := baseDivergenceProbe(ctx, o.projectRoot)
	if err != nil {
		return CheckResult{
			Name:    name,
			Level:   LevelWarn,
			Message: "base vs origin UNVERIFIED",
			Detail: fmt.Sprintf("could not compare %s against origin: %v\n"+
				"lanes may be cut from a stale base; run `%s` if the base is behind",
				o.projectRoot, err, reconcileCommand),
		}
	}
	if st.Skipped {
		return CheckResult{
			Name:    name,
			Level:   LevelPass,
			Message: "base divergence not applicable",
			Detail:  st.Reason,
		}
	}
	if st.Behind > 0 {
		return CheckResult{
			Name:    name,
			Level:   LevelHalt,
			Message: fmt.Sprintf("local %s is %d commit(s) behind origin/%s", st.Branch, st.Behind, st.Branch),
			Detail: fmt.Sprintf(
				"every lane in this batch would be cut from a stale base and its ship would be rejected at push (GIT_PUSH_REJECTED).\n"+
					"local %s: %d ahead / %d behind origin/%s (freshly fetched).\n"+
					"reconcile first: `%s`",
				st.Branch, st.Ahead, st.Behind, st.Branch, reconcileCommand),
		}
	}
	return CheckResult{
		Name:    name,
		Level:   LevelPass,
		Message: fmt.Sprintf("base %s is up to date with origin", st.Branch),
		Detail:  fmt.Sprintf("%d ahead / 0 behind origin/%s", st.Ahead, st.Branch),
	}
}
