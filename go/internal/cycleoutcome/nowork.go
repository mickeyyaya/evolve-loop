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
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/landedclaim"
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
	LandedCheck landedclaim.Check
	Protected   func(string) bool
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
	claims := landedClaimsOf(in.Workspace)
	for _, d := range committedset.Dispositions(in.Workspace) {
		if !containsID(scope, d.ID) {
			continue
		}
		answers[d.ID] = noWorkRouteReason(in.Cycle, d)
		if present, alreadyRouted := itemRouteState(inboxDir, d.ID, os.ReadFile); !present || alreadyRouted {
			continue
		}
		if stampLanded(in, d.ID, claims[d.ID]) {
			continue
		}
		if err := stampDisposition(in, d); err != nil {
			errs = append(errs, err)
		}
		if _, err := inboxmover.RouteConsole(opts, d.ID, answers[d.ID], in.Cycle); err != nil {
			if !errors.Is(err, inboxmover.ErrNotFound) {
				errs = append(errs, err)
			}
			continue
		}
		res.Routed = append(res.Routed, d.ID)
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

func landedClaimsOf(workspace string) map[string]string {
	body, err := os.ReadFile(filepath.Join(workspace, committedset.DecisionFile))
	if err != nil {
		return nil
	}
	return inboxmover.LandedClaims(body)
}

func stampLanded(in NoWorkInputs, id, sha string) bool {
	if in.LandedCheck == nil || sha == "" {
		return false
	}
	loc, err := inboxmover.Locate(filepath.Join(in.ProjectRoot, ".evolve", "inbox"), id)
	if err != nil {
		return false
	}
	body, _ := os.ReadFile(loc.Path)
	var item inboxbatch.Item
	if json.Unmarshal(body, &item) != nil || !in.LandedCheck(sha, item.ClaimPaths()) {
		return false
	}
	stamp, _ := json.Marshal(sha)
	return inboxmover.UpdateItemJSON(loc.Path, func(m map[string]json.RawMessage) { m[inboxbatch.LandedSHAField] = stamp }) == nil
}

func stampProtectedPath(in NoWorkInputs, d committedset.Disposition) error {
	if in.Protected == nil || (d.Bucket != "dropped" && d.Bucket != "escalate_block") {
		return nil
	}
	tag, path := inboxmover.DropReasonToken(d.Reason)
	if tag != inboxmover.ProtectedSurfaceTag || path == "" || !in.Protected(path) {
		return nil
	}
	return addItemProtectedFiles(in.ProjectRoot, d.ID, []string{path})
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

func itemRouteState(inboxDir, id string, readFile func(string) ([]byte, error)) (present, consoleRouted bool) {
	loc, err := inboxmover.Locate(inboxDir, id)
	if err != nil {
		return false, false
	}
	body, err := readFile(loc.Path)
	if err != nil {
		return true, false
	}
	var item struct {
		Route string `json:"route"`
	}
	_ = json.Unmarshal(body, &item)
	return true, strings.HasPrefix(item.Route, "console")
}

func noWorkRouteReason(cycle int, d committedset.Disposition) string {
	reason := fmt.Sprintf("%s%d) %s", inboxbatch.NoWorkRoutePrefix, cycle, d.Bucket)
	if d.Reason != "" {
		reason += ": " + d.Reason
	}
	return reason
}
