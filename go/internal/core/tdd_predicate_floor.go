package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/testdup"
)

const tddFloorReasonHead = "tdd handoff floor: "

type tddPredicateFloor struct{}

func withMandatoryReviewers(next DeliverableReviewer) DeliverableReviewer {
	return chainReviewer{tddPredicateFloor{}, withMandatoryExplanationReviewer(next)}
}

func (tddPredicateFloor) Review(_ context.Context, in ReviewInput) ReviewResult {
	if in.Phase != string(PhaseTDD) || in.Worktree == "" || in.Cycle <= 0 {
		return ReviewResult{Approve: true}
	}
	failures, err := tddHelperCopyFailures(filepath.Join(in.Worktree, "go"), in.Cycle)
	if err != nil {
		fmt.Fprintf(in.errOut(), "[tdd-floor] WARN helper-copy check skipped: %v; the ship ratchet still counts the copies\n", err)
		return ReviewResult{Approve: true}
	}
	if len(failures) == 0 {
		return ReviewResult{Approve: true}
	}
	reason := fmt.Sprintf("%s%d helper body copy group(s) in go/acs/cycle%d/ — no later phase can change these files, so fix them now: call pkg/acsassert or the existing helper instead of a copy (docs/architecture/test-system.md §6.4, §6.5):%s%s",
		tddFloorReasonHead, len(failures), in.Cycle, floorFailureSeparator, strings.Join(failures, floorFailureSeparator))
	fmt.Fprintf(in.errOut(), "[tdd-floor] REJECT: %s\n", reason)
	return ReviewResult{Approve: false, Retry: true, Reason: reason, Failures: failures}
}

func tddHelperCopyFailures(moduleDir string, cycle int) ([]string, error) {
	raw, err := os.ReadFile(filepath.Join(moduleDir, testdup.BaselineRelPath))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	baseline, err := testdup.ParseBaseline(raw)
	if err != nil {
		return nil, err
	}
	report, err := testdup.Scan(moduleDir, baseline.Date)
	if err != nil {
		return nil, err
	}
	var failures []string
	for _, g := range testdup.GroupsUnder(report, fmt.Sprintf("acs/cycle%d", cycle)) {
		failures = append(failures, strings.Join(g.Sites, " = "))
	}
	return failures, nil
}
