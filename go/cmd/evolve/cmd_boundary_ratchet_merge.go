package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/rawgitratchet"
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
)

var ratchetLists = []string{"go/" + sizeratchet.OffendersRelPath, "go/" + rawgitratchet.BaselineRelPath}

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
		g.stage, err = pol.BoundaryRatchetMergeStage()
	}
	if err == nil && g.stage != config.StageOff {
		g.merger, err = ratchetPRMerger(root, stderr)
	}
	return g, err
}

func landTightenPR(root, wt string, pr ratchetPR, stdout, stderr io.Writer) bool {
	g, err := ratchetMergeGateFor(root, stderr)
	attempted := false
	if err == nil {
		attempted, err = g.land(context.Background(), wt, pr, stdout)
	}
	var refusal prRefusal
	switch {
	case errors.As(err, &refusal):
		fmt.Fprintf(stdout, "ratchet merge: #%d held: %v\n", pr.Number, refusal)
	case err != nil:
		fmt.Fprintf(stderr, "%sWARN: ratchet merge: #%d: %v\n", boundaryPrefix, pr.Number, err)
	}
	return attempted
}

func (g ratchetMergeGate) land(ctx context.Context, wt string, pr ratchetPR, stdout io.Writer) (bool, error) {
	if g.stage == config.StageOff {
		return false, nil
	}
	n := strconv.Itoa(pr.Number)
	if err := g.check(ctx, wt, pr); err != nil {
		return false, err
	}
	if g.stage != config.StageEnforce {
		fmt.Fprintf(stdout, "ratchet merge: would merge #%s at %s\n", n, shortSHA(pr.HeadRefOid))
		return false, nil
	}
	sha, err := g.merger.mergeHead(ctx, n, pr.HeadRefOid)
	if err == nil {
		fmt.Fprintf(stdout, "ratchet merge: merged #%s %s\n", n, sha)
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
	if _, err := ratchetCmd(wt, "git", "merge-base", "--is-ancestor", tip+"^", "HEAD"); err != nil {
		return fmt.Errorf("the base of %s is not on origin/main: %w", shortSHA(tip), err)
	}
	if err := scanWithListsOf(wt, tip); err != nil {
		return err
	}
	return g.merger.awaitGreen(ctx, strconv.Itoa(pr.Number), tip)
}

func fetchBranchTip(wt, ref string) (string, error) {
	if err := runGitSteps(wt, []string{"fetch", "--quiet", "origin", ref}); err != nil {
		return "", err
	}
	return ratchetCmd(wt, "git", "rev-parse", "FETCH_HEAD")
}

func scanWithListsOf(wt, tip string) error {
	if err := runGitSteps(wt, append([]string{"checkout", tip, "--"}, ratchetLists...)); err != nil {
		return err
	}
	module := filepath.Join(wt, "go")
	sizeErr := sizeratchet.Scan(module)
	_, rawErr := rawgitratchet.Scan(module)
	return errors.Join(sizeErr, rawErr, runGitSteps(wt, append([]string{"checkout", "HEAD", "--"}, ratchetLists...)))
}

func tipHoldsLists(wt, ref, lease string) (bool, error) {
	if lease == "" {
		return false, nil
	}
	_, err := fetchBranchTip(wt, ref)
	var diff string
	if err == nil {
		diff, err = ratchetCmd(wt, "git", append([]string{"diff", "--name-only", lease, "--"}, ratchetLists...)...)
	}
	return err == nil && diff == "", err
}
