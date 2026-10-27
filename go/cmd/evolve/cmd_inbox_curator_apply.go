package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxcurator"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

const curatorCommitMessage = "chore(inbox): the inbox curator's applied proposals"

type curatorCursor struct {
	Head string `json:"head"`
	At   string `json:"at"`
}

func (r *curatorRun) ctx() context.Context { return context.Background() }

func (r *curatorRun) settle(proposals []inboxcurator.Proposal, isProtected func(string) bool) curateReport {
	report := curateReport{Schema: curatorSchema, Stage: r.cfg.Stage, Proposals: make([]curatedProposal, len(proposals))}
	publish := r.req.apply || r.req.event || r.req.follow
	consumed := map[string]bool{}
	for i, p := range proposals {
		applied := r.req.apply && r.cfg.Enforced() && p.Deterministic && !consumed[p.ID] && r.applyProposal(p, isProtected)
		consumed[p.ID] = consumed[p.ID] || (applied && p.Kind == inboxcurator.ProposeConsume)
		report.Proposals[i] = curatedProposal{Proposal: p, Applied: applied}
		if publish {
			r.signals.Emit(p.Event(r.cfg.Stage, applied))
		}
	}
	return report
}

func (r *curatorRun) applyProposal(p inboxcurator.Proposal, isProtected func(string) bool) bool {
	item := filepath.Join(".evolve", "inbox", p.Path)
	if _, err := os.Stat(filepath.Join(r.req.root, item)); err != nil {
		return false
	}
	why := "inbox curator: " + p.Reason + " (" + strings.Join(p.Evidence, "; ") + ")"
	var ok bool
	switch {
	case p.Kind == inboxcurator.ProposeConsume:
		stamp := inboxConsumedStamp{Via: "inbox curate", Cycle: "console", Resolution: why}
		_, rc := consumeInboxItem(filepath.Join(r.req.root, ".evolve"), filepath.Join(r.req.root, item), stamp, r.stderr, r.stderr)
		ok = rc == 0
		r.recordLanding(ok, item, filepath.Join(".evolve", "inbox", "consumed", p.Path))
	case p.Kind == inboxcurator.ProposeReroute && p.Target == inboxcurator.RouteConsoleTarget:
		opts := inboxmover.Options{ProjectRoot: r.req.root, Stderr: r.stderr, IsProtectedPath: isProtected, Signals: r.signals}
		_, err := inboxmover.RouteConsole(opts, p.ID, why, 0)
		if err != nil {
			fmt.Fprintf(r.stderr, "inbox curate: reroute %s: %v\n", p.ID, err)
		}
		ok = err == nil
		r.recordLanding(ok, item)
	default:
		return false
	}
	return ok
}

func (r *curatorRun) recordLanding(ok bool, paths ...string) {
	if !ok {
		r.applyFailed = true
		return
	}
	r.landed = append(r.landed, paths...)
}

func (r *curatorRun) land() error {
	if paths := r.landable(); len(paths) > 0 {
		if err := r.git.Run(r.ctx(), append([]string{"add", "--all", "--"}, paths...)...); err != nil {
			return fmt.Errorf("stage the applied proposals: %w", err)
		}
		if err := r.git.Run(r.ctx(), append([]string{"commit", "-q", "-m", curatorCommitMessage, "--"}, paths...)...); err != nil {
			return fmt.Errorf("commit the applied proposals: %w", err)
		}
	}
	if r.rangeSkipped || r.applyFailed {
		fmt.Fprintln(r.stderr, "inbox curate: WARN the cursor stays, so the next boundary reads the range again")
		return nil
	}
	head, err := r.git.HEAD(r.ctx())
	if err != nil {
		return fmt.Errorf("read HEAD for the cursor: %w", err)
	}
	return atomicwrite.JSON(filepath.Join(r.req.root, ".evolve", curatorCursorFile), curatorCursor{Head: head, At: time.Now().UTC().Format(time.RFC3339)})
}

func (r *curatorRun) landable() []string {
	var paths []string
	for _, p := range r.landed {
		_, statErr := os.Stat(filepath.Join(r.req.root, p))
		_, _, code, err := r.git.Capture(r.ctx(), "ls-files", "--error-unmatch", "--", p)
		if statErr == nil || (err == nil && code == 0) {
			paths = append(paths, p)
		}
	}
	return paths
}

func (r *curatorRun) sinceCursor() ([]inboxcurator.Change, error) {
	raw, err := os.ReadFile(filepath.Join(r.req.root, ".evolve", curatorCursorFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var cur curatorCursor
	if err := json.Unmarshal(raw, &cur); err != nil {
		fmt.Fprintf(r.stderr, "inbox curate: WARN the cursor %s does not decode (%v); the range is lost and the cursor restarts at HEAD\n", curatorCursorFile, err)
		return nil, nil
	}
	if cur.Head == "" {
		return nil, nil
	}
	if !r.commitExists(cur.Head) || !inboxcurator.IsCommitID(cur.Head) {
		fmt.Fprintf(r.stderr, "inbox curate: WARN the cursor commit %q is not in this repository; the range is lost and the cursor restarts at HEAD\n", cur.Head)
		return nil, nil
	}
	changes, err := inboxcurator.Landed(r.ctx(), r.git, cur.Head, "HEAD")
	if err != nil {
		fmt.Fprintf(r.stderr, "inbox curate: WARN %v; the range is skipped and the cursor stays\n", err)
		r.rangeSkipped = true
		return nil, nil
	}
	return changes, nil
}

func (r *curatorRun) commitExists(sha string) bool {
	if strings.HasPrefix(sha, "-") {
		return false
	}
	_, _, code, err := r.git.Capture(r.ctx(), "cat-file", "-e", sha+"^{commit}")
	return err == nil && code == 0
}
