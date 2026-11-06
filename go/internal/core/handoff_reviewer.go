package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/request"
	"github.com/mickeyyaya/evolve-loop/go/internal/inputindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type HandoffChecker func(in ReviewInput, pair string) handoff.Check

type HandoffReviewer struct {
	config policy.HandoffConfig
	check  HandoffChecker
	tree   func(ReviewInput) string
}

var errHandoffLedgerNotObject = errors.New("the ledger is not a JSON object, so it is kept as written")

var errHandoffLedgerNotRegular = errors.New("the ledger is not a regular file, so the pipeline neither reads nor rewrites it")

func ReadHandoffLedger(path string) ([]byte, error) {
	return readHandoffLedgerWith(path, openLedgerWithoutBlocking)
}

func openLedgerWithoutBlocking(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}

func readHandoffLedgerWith(path string, open func(string) (*os.File, error)) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || inputindex.HardLinked(before) {
		return nil, errHandoffLedgerNotRegular
	}
	f, err := open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	if after, err := f.Stat(); err != nil || !os.SameFile(before, after) {
		return nil, errHandoffLedgerNotRegular
	}
	return io.ReadAll(f)
}

var handoffPairEndingAt = map[string]string{
	string(PhaseTDD):   request.PairTriageTDD,
	string(PhaseBuild): request.PairTDDBuild,
}

func NewHandoffReviewer(config policy.HandoffConfig, check HandoffChecker) HandoffReviewer {
	return HandoffReviewer{config: config, check: check}
}

const handoffOutcomeInvalid = "invalid"

func (r HandoffReviewer) outputTree(ctx context.Context, in ReviewInput) string {
	if r.tree != nil {
		return r.tree(in)
	}
	return worktreeContentSHA(ctx, in.ProjectRoot, in.Worktree, in.Workspace)
}

func staleOutputCheck(check handoff.Check, stale string) handoff.Check {
	if stale == "" {
		return check
	}
	check.Outcome = handoffOutcomeInvalid
	check.Errors = append(slices.Clone(check.Errors), fmt.Sprintf("output_digest %s is stale: the ledger names an earlier output than the current tree", stale))
	return check
}

func (r HandoffReviewer) Review(ctx context.Context, in ReviewInput) ReviewResult {
	tree := ""
	if _, paired := handoffLedgerPairOf[in.Phase]; paired {
		tree = r.outputTree(ctx, in)
	}
	stale := stampHandoffAuthor(in, tree)
	pair, ok := handoffPairEndingAt[in.Phase]
	if !ok {
		return ReviewResult{Approve: true}
	}
	check := staleOutputCheck(r.check(in, pair), stale)
	check.Stage = r.config[pair].Stage
	if err := recordHandoffCheck(in.Workspace, pair, check); err != nil {
		fmt.Fprintf(in.errOut(), "[orchestrator] WARN handoff %s: the check record was not written: %v\n", pair, err)
	}
	if check.Stage != config.StageEnforce.String() || check.Outcome == string(handoff.ExitPass) {
		return ReviewResult{Approve: true}
	}
	return enforcedHandoffRejection(in.Phase, pair, check)
}

func enforcedHandoffRejection(phase, pair string, check handoff.Check) ReviewResult {
	if check.Outcome == string(handoff.ExitBlocked) {
		return ReviewResult{
			Reason:      fmt.Sprintf("handoff %s: blocked: asks %s", pair, strings.Join(check.Blocked, ", ")),
			Remediation: fmt.Sprintf("The check of pair %s proved that asks %s cannot be met (proof: %s). Name each conflict for the console in handoff-%s.json.", pair, strings.Join(check.Blocked, ", "), strings.Join(check.Paths, ", "), pair),
			ProvenBlocker: fmt.Sprintf("blocked: needs console — phase %s stopped on the handoff check of pair %s: the request cannot be met for asks %s (proof: %s)",
				phase, pair, strings.Join(check.Blocked, ", "), strings.Join(check.Paths, ", ")),
		}
	}
	reason := fmt.Sprintf("handoff %s: %s; unmet asks: %s", pair, check.Outcome, strings.Join(check.Unmet, ", "))
	var remedy strings.Builder
	fmt.Fprintf(&remedy, "Open the handoff-asks entry of inputs/index.json. Meet each unmet ask of the %s request, by its ask id, in handoff-%s.json, with evidence that resolves: %s.", pair, pair, strings.Join(check.Unmet, ", "))
	for _, line := range slices.Concat(check.FailedEvidence, check.Errors) {
		fmt.Fprintf(&remedy, "\n- %s", line)
	}
	return ReviewResult{Retry: true, Reason: reason, Remediation: remedy.String()}
}

func recordHandoffCheck(workspace, pair string, check handoff.Check) error {
	path := filepath.Join(workspace, "handoff-"+pair+".json")
	doc := map[string]any{}
	raw, err := ReadHandoffLedger(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		doc["schema_version"] = handoff.SchemaVersion
		doc["pair"] = pair
	case err != nil:
		return err
	default:
		var fields map[string]json.RawMessage
		if json.Unmarshal(raw, &fields) != nil || fields == nil {
			return errHandoffLedgerNotObject
		}
		for key, value := range fields {
			doc[key] = value
		}
	}
	doc["check"] = check
	return atomicwrite.JSON(path, doc)
}

func (o *Orchestrator) HandoffReviewerWired() bool {
	return reviewerChainHas(o.reviewer, func(r DeliverableReviewer) bool {
		_, ok := r.(HandoffReviewer)
		return ok
	})
}
