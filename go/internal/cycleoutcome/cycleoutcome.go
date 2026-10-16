// Package cycleoutcome is the importable FAIL-closeout seam: the symmetric
// sibling of the PASS half that already lives inside the cycle process
// (internal/phases/ship/postship.go).
package cycleoutcome

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleclassify"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type FailureInputs struct {
	ProjectRoot    string
	Workspace      string
	Cycle          int
	Ceiling        int
	SystemLevel    bool
	Refusal        string
	RefusalDetail  string
	RefusalSubject string
	Reason         string
	Stderr         io.Writer
	Ledger         inboxmover.LedgerAppender
	Signals        *signalcenter.Center
}

func (in FailureInputs) WithLedger(l inboxmover.LedgerAppender) FailureInputs {
	in.Ledger = l
	return in
}

func (in FailureInputs) WithSignals(c *signalcenter.Center) FailureInputs {
	in.Signals = c
	return in
}

func ApplyFailure(in FailureInputs) (inboxmover.OutcomeResult, error) {
	stderr := in.Stderr
	if stderr == nil {
		stderr = io.Discard
	}
	committed := CommittedIDsFor(in.Workspace)
	if len(committed) == 0 {
		committed = committedset.Unanswered(in.Workspace)
	}
	opts := inboxmover.Options{
		ProjectRoot: in.ProjectRoot,
		Ledger:      in.Ledger,
		Stderr:      stderr,
		Signals:     in.Signals,
	}
	routed := false
	if cyclestate.RefusalDisposition(in.Refusal).RouteConsole && in.RefusalSubject != "" {
		if !containsID(committed, in.RefusalSubject) {
			fmt.Fprintf(stderr, "[cycleoutcome] WARN: route-console skipped: refused subject %q is not in the committed set %v — task-level bump instead\n", in.RefusalSubject, committed)
		} else if _, rerr := inboxmover.RouteConsole(opts, in.RefusalSubject, in.RefusalDetail, in.Cycle); rerr == nil {
			routed = true
		}
	}
	res, err := inboxmover.ApplyCycleOutcome(opts, inboxmover.CycleOutcome{
		Cycle:        in.Cycle,
		Passed:       false,
		CommittedIDs: committed,
		Reason:       in.Reason,
		Ceiling:      in.Ceiling,
		SystemLevel:  in.SystemLevel,
		Routed:       routed,
	})
	if err != nil {
		return res, fmt.Errorf("apply cycle %d failure outcome: %w", in.Cycle, err)
	}
	return res, nil
}

func FailureInputsFor(projectRoot, evolveDir, workspace string, cycle int, stderr io.Writer) FailureInputs {
	failPol := policy.DefaultSystemFailurePolicy()
	if pol, err := policy.Load(filepath.Join(evolveDir, "policy.json")); err == nil {
		if fp, fpErr := pol.FailurePolicyConfig(); fpErr == nil {
			failPol = fp
		}
	}
	cls := cycleclassify.Classify(workspace)
	in := FailureInputs{
		ProjectRoot: projectRoot,
		Workspace:   workspace,
		Cycle:       cycle,
		Ceiling:     failPol.Thresholds.TaskRetryCeiling,
		SystemLevel: !IsTaskLevelResult(cls),
		Reason:      "cycle-failure-release",
		Stderr:      stderr,
	}
	if cls.Class == cycleclassify.ClassPhaseRefusal {
		in.Refusal, in.RefusalDetail, in.RefusalSubject = cls.Marker, cls.Detail, cls.Subject
	}
	return in
}

func IsTaskLevelFailure(c cycleclassify.Classification) bool {
	switch c {
	case cycleclassify.ClassBuildFail, cycleclassify.ClassAuditFail, cycleclassify.ClassShipGateConfig:
		return true
	default:
		return false
	}
}

func CommittedIDsFor(workspace string) []string {
	body, err := os.ReadFile(filepath.Join(workspace, "triage-decision.json"))
	if err != nil {
		return nil
	}
	return inboxmover.CommittedIDs(body)
}

func LaneScopeIDs(workspace string) []string {
	return core.LaneScopeIDs(workspace)
}

func IsTaskLevelResult(cls cycleclassify.Result) bool {
	if cls.Class == cycleclassify.ClassPhaseRefusal {
		return cyclestate.RefusalDisposition(cls.Marker).TaskLevel
	}
	return IsTaskLevelFailure(cls.Class)
}

func containsID(ids []string, id string) bool {
	for _, c := range ids {
		if c == id {
			return true
		}
	}
	return false
}
