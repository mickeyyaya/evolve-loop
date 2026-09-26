package core

import (
	"context"
	"fmt"
	"strings"
)

// CarryforwardCandidateLandable reports whether candidateRef merges cleanly onto base and is not already landed; only a git failure is an error.
func CarryforwardCandidateLandable(ctx context.Context, dir, candidateRef, base string) (bool, error) {
	// Supersession first: a dry-run merge of an already-absorbed change reads clean.
	superseded, err := refSuperseded(ctx, dir, candidateRef, base)
	if err != nil {
		return false, err
	}
	if superseded {
		return false, nil
	}

	// `merge-tree --write-tree` is a real 3-way merge that writes only to the
	// object DB; exit 1 is a conflict. The bare 1-arg form reports clean on conflicts.
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

// FleetRebaseVerdict classifies a fleet-rebase candidate before the ship-recovery rebase replay.
type FleetRebaseVerdict int

const (
	// FleetRebaseAlreadyLanded means the candidate is superseded: no replay, no re-audit.
	FleetRebaseAlreadyLanded FleetRebaseVerdict = iota
	// FleetRebaseClean means the candidate merges cleanly and is not landed: replay the rebase.
	FleetRebaseClean
	// FleetRebaseConflict means real overlapping work: route to the debugger, never short-circuit.
	FleetRebaseConflict
)

// ClassifyFleetRebaseCandidate splits CarryforwardCandidateLandable's answer into clean, already landed or conflict.
func ClassifyFleetRebaseCandidate(ctx context.Context, dir, candidateRef, base string) (FleetRebaseVerdict, error) {
	landable, err := CarryforwardCandidateLandable(ctx, dir, candidateRef, base)
	if err != nil {
		return FleetRebaseAlreadyLanded, err
	}
	if landable {
		return FleetRebaseClean, nil
	}
	// Not landable means superseded or a real conflict; calling a conflict
	// landed would silently drop overlapping work, so re-screen supersession.
	superseded, err := refSuperseded(ctx, dir, candidateRef, base)
	if err != nil {
		return FleetRebaseAlreadyLanded, err
	}
	if superseded {
		return FleetRebaseAlreadyLanded, nil
	}
	return FleetRebaseConflict, nil
}

// refSuperseded reports whether ref is an ancestor of base or a patch-id
// duplicate of it. The carry-forward filter and the orphan-prune walker share it.
func refSuperseded(ctx context.Context, dir, ref, base string) (bool, error) {
	if _, code, err := gitCapture(ctx, dir, "merge-base", "--is-ancestor", ref, base); err != nil {
		return false, fmt.Errorf("carryforward filter: is-ancestor %s %s: %w", ref, base, err)
	} else if code == 0 {
		return true, nil
	}
	return commitsFullyLanded(ctx, dir, ref, base)
}

// commitsFullyLanded reads `git cherry`, which marks each base..ref commit `-`
// when base holds a patch-id equivalent and `+` when it does not.
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
