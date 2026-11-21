package inboxcmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentrequest"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxcurator"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxfile"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/verifylock"
)

type RefusedError struct {
	Cause string
	Err   error
}

func (e *RefusedError) Error() string { return agentrequest.ErrItemRefused.Error() + ": " + e.Cause }

func (e *RefusedError) Unwrap() []error { return []error{agentrequest.ErrItemRefused, e.Err} }

var (
	errItemDecode     = errors.New("the item does not decode")
	errOriginConflict = errors.New("the item origin conflicts with the origin of the request")
)

func refuse(cause error) *RefusedError { return &RefusedError{Cause: cause.Error(), Err: cause} }

func refuseAs(kind error, cause string) *RefusedError { return &RefusedError{Cause: cause, Err: kind} }

func refusalOf(id string, err error) *RefusedError {
	var overlap *inboxcurator.OverlapError
	if errors.As(err, &overlap) {
		return &RefusedError{Cause: strings.Join(overlapLines(id, overlap.Refused, ""), "\n"), Err: err}
	}
	return refuse(err)
}

type intakeInput struct {
	root        string
	raw         []byte
	req         inboxAddRequest
	source      string
	pol         policy.Policy
	isProtected func(path string) bool
	signals     *signalcenter.Center
	stderr      io.Writer
}

func inboxDir(root string) string { return filepath.Join(root, ".evolve", "inbox") }

func intakeItem(ctx context.Context, in intakeInput) (inboxmover.FileResult, error) {
	lockCtx, cancel := context.WithTimeout(ctx, inboxfile.LockWait)
	defer cancel()
	release, err := verifylock.AcquireAt(lockCtx, filepath.Join(inboxDir(in.root), inboxfile.LockName), in.stderr)
	if err != nil {
		return inboxmover.FileResult{}, fmt.Errorf("the overlap check cannot lock the inbox: %w", err)
	}
	defer release()
	if prior, replayed, err := priorFiling(in); err != nil || replayed {
		return prior, err
	}
	opts := inboxmover.Options{ProjectRoot: in.root, Stderr: in.stderr, IsProtectedPath: in.isProtected, Signals: in.signals,
		PriorityClasses: in.pol.InboxPriorityConfig().ClassOrder, Origin: in.req.origin, Source: in.source}
	if err := admitInboxItem(in.raw, in.req, in.pol.InboxCuratorConfig(), &opts, in.stderr); err != nil {
		return inboxmover.FileResult{}, err
	}
	res, err := inboxmover.File(opts, in.raw)
	if errors.Is(err, inboxmover.ErrInvalidItem) {
		return res, refuse(err)
	}
	return res, err
}

func NewFiler(deps Deps, stderr io.Writer) agentrequest.FileFunc {
	return func(ctx context.Context, root string, raw []byte, source string) (agentrequest.Filed, error) {
		if deps.LaneForbidden == nil || deps.NewSignalCenter == nil {
			return agentrequest.Filed{}, errors.New("the inbox filer lacks the host functions LaneForbidden and NewSignalCenter")
		}
		pol, err := policy.Load(filepath.Join(root, ".evolve", "policy.json"))
		if err != nil {
			return agentrequest.Filed{}, fmt.Errorf("load the policy: %w", err)
		}
		var item inboxbatch.Item
		if jsonErr := json.Unmarshal(raw, &item); jsonErr != nil {
			return agentrequest.Filed{}, refuseAs(errItemDecode, errItemDecode.Error()+": "+jsonErr.Error())
		}
		signals, closeSignals := deps.NewSignalCenter("inbox", root, filepath.Join(root, ".evolve"), stderr)
		defer closeSignals()
		in := intakeInput{root: root, raw: raw, source: source, pol: pol, isProtected: deps.LaneForbidden(root, stderr), signals: signals, stderr: stderr,
			req: inboxAddRequest{origin: inboxbatch.OriginCycle}}
		res, err := intakeItem(ctx, in)
		return agentrequest.Filed{ID: item.ID, Path: res.Path}, err
	}
}

func admitInboxItem(raw []byte, req inboxAddRequest, cfg policy.InboxCuratorConfig, opts *inboxmover.Options, stderr io.Writer) error {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return nil
	}
	var item inboxbatch.Item
	if err := json.Unmarshal(raw, &item); err != nil {
		return refuseAs(errItemDecode, errItemDecode.Error()+": "+err.Error())
	}
	if req.originSet && item.Origin != "" && item.Origin != req.origin {
		return refuseAs(errOriginConflict, fmt.Sprintf("the item names origin %q and --origin names %q; name one", item.Origin, req.origin))
	}
	if item.Origin == "" {
		item.Origin = req.origin
	}
	open, warnings, err := inboxcurator.OpenItems(inboxDir(opts.ProjectRoot))
	if err != nil {
		return fmt.Errorf("the overlap check: %w", err)
	}
	for _, w := range warnings {
		if !w.Unreadable {
			continue
		}
		fmt.Fprintf(stderr, "inbox add: WARN the overlap check skips a file: %s\n", w.String())
	}
	tree, err := probeTree(opts.ProjectRoot, item)
	if err != nil {
		return fmt.Errorf("the probe check: %w", err)
	}
	rules := inboxcurator.Rules{Kinds: cfg.Kinds, IsProtected: opts.IsProtectedPath, Tree: tree,
		Thresholds: inboxcurator.Thresholds{Refuse: cfg.OverlapRefuse, Warn: cfg.OverlapWarn}}
	adm, err := inboxcurator.Admit(item, open, rules, inboxcurator.Override{Reason: req.reason, AllowOverlap: req.allowOverlap})
	reportOverlaps(item.ID, adm.Warned, "WARN ", stderr)
	if err != nil {
		return refusalOf(item.ID, err)
	}
	if adm.Record != nil {
		opts.IntakeRecord, _ = json.Marshal(adm.Record)
	}
	return nil
}

func priorFiling(in intakeInput) (inboxmover.FileResult, bool, error) {
	var item inboxbatch.Item
	if in.source == "" || json.Unmarshal(in.raw, &item) != nil || item.ID == "" {
		return inboxmover.FileResult{}, false, nil
	}
	for _, dir := range append([]string{inboxDir(in.root)}, inboxbatch.ProcessingCycleDirs(inboxDir(in.root))...) {
		scan, err := inboxbatch.ScanDir(dir)
		if err != nil {
			return inboxmover.FileResult{}, false, fmt.Errorf("read the open items: %w", err)
		}
		for _, it := range scan.Items {
			if it.ID == item.ID && it.Source == in.source {
				return inboxmover.FileResult{Path: filepath.Join(dir, it.Path)}, true, nil
			}
		}
	}
	return inboxmover.FileResult{}, false, nil
}
