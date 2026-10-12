package core

import (
	"context"
	"fmt"
	"strings"
)

func CarryforwardCandidateLandable(ctx context.Context, dir, candidateRef, base string) (bool, error) {
	superseded, err := refSuperseded(ctx, dir, candidateRef, base)
	if err != nil {
		return false, err
	}
	if superseded {
		return false, nil
	}

	_, code, err := gitCapture(ctx, dir, "merge-tree", "--write-tree", base, candidateRef)
	if err != nil {
		return false, fmt.Errorf("carryforward filter: merge-tree %s %s: %w", base, candidateRef, err)
	}
	switch code {
	case 0:
		return true, nil
	case 1:
		return false, nil
	default:
		return false, fmt.Errorf("carryforward filter: merge-tree %s %s exited %d", base, candidateRef, code)
	}
}

type FleetRebaseVerdict int

const (
	FleetRebaseAlreadyLanded FleetRebaseVerdict = iota
	FleetRebaseClean
	FleetRebaseConflict
)

func ClassifyFleetRebaseCandidate(ctx context.Context, dir, candidateRef, base string) (FleetRebaseVerdict, error) {
	landable, err := CarryforwardCandidateLandable(ctx, dir, candidateRef, base)
	if err != nil {
		return FleetRebaseAlreadyLanded, err
	}
	if landable {
		return FleetRebaseClean, nil
	}
	superseded, err := refSuperseded(ctx, dir, candidateRef, base)
	if err != nil {
		return FleetRebaseAlreadyLanded, err
	}
	if superseded {
		return FleetRebaseAlreadyLanded, nil
	}
	return FleetRebaseConflict, nil
}

func refSuperseded(ctx context.Context, dir, ref, base string) (bool, error) {
	if _, code, err := gitCapture(ctx, dir, "merge-base", "--is-ancestor", ref, base); err != nil {
		return false, fmt.Errorf("carryforward filter: is-ancestor %s %s: %w", ref, base, err)
	} else if code == 0 {
		return true, nil
	}
	return commitsFullyLanded(ctx, dir, ref, base)
}

func commitsFullyLanded(ctx context.Context, dir, ref, base string) (bool, error) {
	out, code, err := gitCapture(ctx, dir, "cherry", base, ref)
	if err != nil {
		return false, fmt.Errorf("carryforward filter: git cherry %s %s: %w", base, ref, err)
	}
	if code != 0 {
		return false, fmt.Errorf("carryforward filter: git cherry %s %s exited %d", base, ref, code)
	}
	hasCommit, hasNew := false, false
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		hasCommit = true
		if strings.HasPrefix(line, "+") {
			hasNew = true
		}
	}
	return hasCommit && !hasNew, nil
}
