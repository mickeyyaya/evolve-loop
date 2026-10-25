// Package cycleoutcome is the importable FAIL-closeout seam: the symmetric
// sibling of the PASS half that already lives inside the cycle process
// (internal/phases/ship/postship.go).
package cycleoutcome

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleclassify"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type FailureInputs struct {
	ProjectRoot     string
	Workspace       string
	Cycle           int
	Ceiling         int
	SystemLevel     bool
	Refusal         string
	RefusalDetail   string
	RefusalSubject  string
	Reason          string
	FailureClass    retryladder.Class
	Fingerprint     string
	Protected       func(string) bool
	Stderr          io.Writer
	Ledger          inboxmover.LedgerAppender
	Signals         *signalcenter.Center
	Defer           core.RecordedDefer
	PriorityClasses []string
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
		ProjectRoot:     in.ProjectRoot,
		Ledger:          in.Ledger,
		Stderr:          stderr,
		Signals:         in.Signals,
		PriorityClasses: in.PriorityClasses,
	}
	routed := routeRefusal(opts, in, committed, stderr)
	var routedIDs []string
	if !routed {
		routedIDs = routeDeferral(opts, in, committed, stderr)
	}
	res, err := inboxmover.ApplyCycleOutcome(opts, inboxmover.CycleOutcome{
		Cycle:        in.Cycle,
		Passed:       false,
		CommittedIDs: committed,
		Reason:       in.Reason,
		Ceiling:      in.Ceiling,
		SystemLevel:  in.SystemLevel,
		Routed:       routed,
		RoutedIDs:    routedIDs,
		FailureClass: string(in.FailureClass),
		Fingerprint:  in.Fingerprint,
	})
	if err != nil {
		return res, fmt.Errorf("apply cycle %d failure outcome: %w", in.Cycle, err)
	}
	return res, nil
}

func routeRefusal(opts inboxmover.Options, in FailureInputs, committed []string, stderr io.Writer) bool {
	if !cyclestate.RefusalDisposition(in.Refusal).RouteConsole || in.RefusalSubject == "" {
		return false
	}
	if !containsID(committed, in.RefusalSubject) {
		fmt.Fprintf(stderr, "[cycleoutcome] WARN: route-console skipped: refused subject %q is not in the committed set %v — task-level bump instead\n", in.RefusalSubject, committed)
		return false
	}
	detail, files := in.RefusalDetail, []string(nil)
	if in.Refusal == cyclestate.DiagCodeProtectedPredicateRoute {
		proven, why := proveProtectedRoute(in, committed)
		if why != "" {
			fmt.Fprintf(stderr, "[cycleoutcome] WARN: protected-predicate route not proven (%s) — task-level bump instead, the claim is released\n", why)
			return false
		}
		detail, files = core.ProtectedRouteMessage(proven), protectedPaths(proven)
	}
	if len(files) > 0 {
		if ferr := addItemProtectedFiles(opts.ProjectRoot, in.RefusalSubject, files); ferr != nil {
			fmt.Fprintf(stderr, "[cycleoutcome] WARN: the item %q could not gain its protected files (%v) — task-level bump instead\n", in.RefusalSubject, ferr)
			return false
		}
	}
	_, err := inboxmover.RouteConsole(opts, in.RefusalSubject, detail, in.Cycle)
	return err == nil
}

func FailureInputsFor(projectRoot, evolveDir, workspace string, cycle int, stderr io.Writer) FailureInputs {
	failPol := policy.DefaultSystemFailurePolicy()
	pol := policy.LoadOrDefaults(filepath.Join(evolveDir, "policy.json"), stderr)
	if fp, err := pol.FailurePolicyConfig(); err == nil {
		failPol = fp
	} else {
		fmt.Fprintf(stderr, "[cycle-outcome] WARN: failure policy unreadable (%v); the compiled retry ceiling applies\n", err)
	}
	cls := cycleclassify.Classify(workspace)
	in := FailureInputs{
		ProjectRoot:     projectRoot,
		Workspace:       workspace,
		Cycle:           cycle,
		Ceiling:         failPol.Thresholds.TaskRetryCeiling,
		SystemLevel:     !IsTaskLevelResult(cls),
		Reason:          "cycle-failure-release",
		FailureClass:    failureClassOf(cls),
		Fingerprint:     retryladder.Fingerprint(firstFailureReason(workspace)),
		Stderr:          stderr,
		PriorityClasses: pol.InboxPriorityConfig().ClassOrder,
	}
	if cls.Class == cycleclassify.ClassPhaseRefusal {
		in.Refusal, in.RefusalDetail, in.RefusalSubject = cls.Marker, cls.Detail, cls.Subject
	}
	return in
}

func IsTaskLevelFailure(c cycleclassify.Classification) bool {
	norm := failurelog.NormalizeLegacy(string(c))
	switch ladderClassOf(norm) {
	case retryladder.Spec, retryladder.Capability:
		return true
	}
	return norm == failurelog.ShipGateConfig
}

func failureClassOf(cls cycleclassify.Result) retryladder.Class {
	if cls.Marker == cycleclassify.MarkerQuotaLikelyEmptyOutput {
		return retryladder.Quota
	}
	return ladderClassOf(failurelog.NormalizeLegacy(string(cls.Class)))
}

func ladderClassOf(c failurelog.Classification) retryladder.Class {
	switch c {
	case failurelog.InfrastructureTransient, failurelog.InfrastructureSystemic, failurelog.ExitTransportHang:
		return retryladder.Infra
	}
	return retryladder.Classify(retryladder.Facts{AuditClass: string(c)}).Class
}

func firstFailureReason(workspace string) string {
	body, err := os.ReadFile(filepath.Join(workspace, "audit-fail-reason.json"))
	if err != nil {
		return ""
	}
	var doc struct {
		Reasons []string `json:"reasons"`
	}
	if json.Unmarshal(body, &doc) != nil || len(doc.Reasons) == 0 {
		return ""
	}
	return doc.Reasons[0]
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
