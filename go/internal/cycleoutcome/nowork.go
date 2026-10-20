package cycleoutcome

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type NoWorkResult struct {
	Routed  []string
	Retired []string
}

type NoWorkInputs struct {
	ProjectRoot string
	Workspace   string
	Cycle       int
	Stderr      io.Writer
	Ledger      inboxmover.LedgerAppender
	Signals     *signalcenter.Center
}

func (in NoWorkInputs) WithLedger(l inboxmover.LedgerAppender) NoWorkInputs {
	in.Ledger = l
	return in
}

func (in NoWorkInputs) WithSignals(c *signalcenter.Center) NoWorkInputs {
	in.Signals = c
	return in
}

func ApplyNoWork(in NoWorkInputs) (NoWorkResult, error) {
	res := NoWorkResult{}
	scope := LaneScopeIDs(in.Workspace)
	if len(scope) == 0 {
		return res, nil
	}
	stderr := in.Stderr
	if stderr == nil {
		stderr = io.Discard
	}
	opts := inboxmover.Options{ProjectRoot: in.ProjectRoot, Ledger: in.Ledger, Stderr: stderr, Signals: in.Signals}
	inboxDir := filepath.Join(in.ProjectRoot, ".evolve", "inbox")
	var errs []error
	answers := map[string]string{}
	for _, d := range committedset.Dispositions(in.Workspace) {
		if !containsID(scope, d.ID) {
			continue
		}
		answers[d.ID] = noWorkRouteReason(in.Cycle, d)
		routed, err := handOffDisposition(in, opts, inboxDir, d, answers[d.ID])
		if routed {
			res.Routed = append(res.Routed, d.ID)
		}
		errs = appendErr(errs, err)
	}
	if err := committedset.WriteHandoff(in.Workspace, committedset.Handoff{Cycle: in.Cycle, Routed: res.Routed}); err != nil {
		errs = append(errs, fmt.Errorf("record the no-work hand-off: %w", err))
	}
	retired, err := retireUnbackedScope(opts, in.Cycle, scope, answers)
	res.Retired = retired
	if err != nil {
		errs = append(errs, err)
	}
	if _, err := inboxmover.ReleaseCycleProcessingWithReason(opts, in.Cycle, "planned no-work: the lane's triage answered without committing"); err != nil {
		errs = append(errs, err)
	}
	return res, errors.Join(errs...)
}

func handOffDisposition(in NoWorkInputs, opts inboxmover.Options, inboxDir string, d committedset.Disposition, reason string) (bool, error) {
	path, alreadyRouted := itemRouteState(inboxDir, d.ID, os.ReadFile)
	switch {
	case path == "":
		return false, nil
	case alreadyRouted:
		return false, stampDropEvidence(in, path, d)
	}
	routed, err := inboxmover.RouteConsole(opts, d.ID, reason, in.Cycle)
	if err != nil {
		if errors.Is(err, inboxmover.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, stampDropEvidence(in, routed.Path, d)
}

func appendErr(errs []error, err error) []error {
	if err == nil {
		return errs
	}
	return append(errs, err)
}

func retireUnbackedScope(opts inboxmover.Options, cycle int, scope []string, answers map[string]string) ([]string, error) {
	var retired []string
	var errs []error
	for _, id := range scope {
		reason, answered := answers[id]
		if !answered {
			reason = fmt.Sprintf("planned no-work (cycle %d): the lane's triage answered for nothing", cycle)
		}
		ids, err := inboxmover.RetireUnbacked(opts, cycle, inboxmover.StateRejected, reason, "", []string{id})
		retired = append(retired, ids...)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return retired, errors.Join(errs...)
}

func itemRouteState(inboxDir, id string, readFile func(string) ([]byte, error)) (path string, consoleRouted bool) {
	loc, err := inboxmover.Locate(inboxDir, id)
	if err != nil {
		return "", false
	}
	body, err := readFile(loc.Path)
	if err != nil {
		return loc.Path, false
	}
	var item struct {
		Route string `json:"route"`
	}
	_ = json.Unmarshal(body, &item)
	return loc.Path, strings.HasPrefix(item.Route, "console")
}

func noWorkRouteReason(cycle int, d committedset.Disposition) string {
	reason := fmt.Sprintf("lane triage (cycle %d) %s", cycle, d.Bucket)
	if d.Reason != "" {
		reason += ": " + d.Reason
	}
	return reason
}
