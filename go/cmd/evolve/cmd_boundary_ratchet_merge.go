package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type ratchetMergeGate struct {
	stage  config.Stage
	merger prMerger
}

var ratchetPRMerger = func(root string, stderr io.Writer) (prMerger, error) {
	return newPRMerger(root, prMergeArgs{}, io.Discard, stderr)
}

func ratchetMergeGateFor(root string, stderr io.Writer) (ratchetMergeGate, error) {
	var g ratchetMergeGate
	pol, err := policy.Load(paths.PolicyPath(paths.EvolveDirOf(root)))
	if err == nil {
		g.stage = pol.BoundaryRatchetMergeStage()
	}
	if err == nil && g.stage != config.StageOff {
		g.merger, err = ratchetPRMerger(root, stderr)
	}
	return g, err
}

func (s ratchetStep) landTightenPR(pr ratchetPR) bool {
	g, err := ratchetMergeGateFor(s.root, s.stderr)
	attempted := false
	if err == nil {
		attempted, err = g.land(context.Background(), s, pr)
	}
	var refusal prRefusal
	switch {
	case errors.As(err, &refusal):
		fmt.Fprintf(s.stdout, "ratchet merge: #%d held: %v\n", pr.Number, refusal)
	case err != nil:
		fmt.Fprintf(s.stderr, "%sWARN: ratchet merge: #%d: %v\n", boundaryPrefix, pr.Number, err)
	}
	return attempted
}

func (g ratchetMergeGate) land(ctx context.Context, s ratchetStep, pr ratchetPR) (bool, error) {
	if g.stage == config.StageOff {
		return false, nil
	}
	n := strconv.Itoa(pr.Number)
	if err := g.check(ctx, s.wt, pr); err != nil {
		return false, err
	}
	if g.stage != config.StageEnforce {
		fmt.Fprintf(s.stdout, "ratchet merge: would merge #%s at %s\n", n, shortSHA(pr.HeadRefOid))
		return false, nil
	}
	sha, err := g.merger.mergeHead(ctx, n, pr.HeadRefOid)
	if err == nil {
		fmt.Fprintf(s.stdout, "ratchet merge: merged #%s %s\n", n, sha)
	}
	return true, err
}

func (g ratchetMergeGate) check(ctx context.Context, wt string, pr ratchetPR) error {
	tip, err := fetchBranchTip(wt, "refs/heads/"+pr.HeadRefName)
	switch {
	case err != nil:
		return err
	case tip != pr.HeadRefOid:
		return fmt.Errorf("the head moved from %s to %s", shortSHA(pr.HeadRefOid), shortSHA(tip))
	}
	if err := stepCommitError(wt, tip); err != nil {
		return err
	}
	holds, err := tipHoldsLists(wt, tip)
	switch {
	case err != nil:
		return err
	case !holds:
		return refusef("the lists at %s are not the lists that the step writes from origin/main", shortSHA(tip))
	}
	return g.merger.awaitGreen(ctx, strconv.Itoa(pr.Number), tip)
}

func stepCommitError(wt, tip string) error {
	parents, err := ratchetCmd(wt, "git", "rev-list", "--parents", "-n1", tip)
	if err != nil {
		return err
	}
	if n := len(strings.Fields(parents)) - 1; n != 1 {
		return fmt.Errorf("%s has %d parents, and a tighten commit has one", shortSHA(tip), n)
	}
	if _, err := ratchetCmd(wt, "git", "merge-base", "--is-ancestor", tip+"^", "HEAD"); err != nil {
		return fmt.Errorf("the base of %s is not on origin/main: %w", shortSHA(tip), err)
	}
	names, err := ratchetCmd(wt, "git", "diff", "--no-renames", "--name-only", "-z", "HEAD..."+tip)
	if err != nil {
		return err
	}
	return onlyRatchetLists(tip, slices.DeleteFunc(strings.Split(names, "\x00"), func(p string) bool { return p == "" }))
}

func onlyRatchetLists(tip string, changed []string) error {
	if len(changed) == 0 {
		return fmt.Errorf("%s changes no ratchet list", shortSHA(tip))
	}
	allowed := ratchetListPaths()
	for _, path := range changed {
		if !slices.Contains(allowed, path) {
			return fmt.Errorf("%s changes %s, and a tighten commit changes only %s", shortSHA(tip), path, strings.Join(allowed, ", "))
		}
	}
	return nil
}

func fetchBranchTip(wt, ref string) (string, error) {
	if err := runGitSteps(wt, []string{"fetch", "--quiet", "origin", ref}); err != nil {
		return "", err
	}
	return ratchetCmd(wt, "git", "rev-parse", "FETCH_HEAD")
}

func tipHoldsLists(wt, tip string) (bool, error) {
	diff, err := ratchetCmd(wt, "git", append([]string{"diff", "--no-renames", "--name-only", tip, "--"}, ratchetListPaths()...)...)
	return err == nil && diff == "", err
}

func leaseHoldsTheStepLists(wt, lease string) (bool, error) {
	if lease == "" || stepCommitError(wt, lease) != nil {
		return false, nil
	}
	return tipHoldsLists(wt, lease)
}
