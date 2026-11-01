package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

const gcContinuationAuthority = "evolve gc (continuation max age)"

type continuationAge struct {
	maxAge int
	loaded bool
}

func (r gcRun) agedContinuations(projectRoot string, age continuationAge, base string) bool {
	if !age.loaded {
		fmt.Fprintln(r.stderr, "evolve gc: WARN: continuation sweep skipped: policy.json did not load, so the age of a binding is unknown and nothing is released")
		return false
	}
	return r.continuations(projectRoot, age.maxAge, base)
}

func (r gcRun) continuations(projectRoot string, maxAge int, base string) bool {
	last, err := readLastCycleNumber(r.ctx, storage.New(filepath.Join(projectRoot, ".evolve")))
	if err != nil {
		fmt.Fprintf(r.stderr, "evolve gc: continuation sweep: the last cycle is unreadable: %v\n", err)
		return true
	}
	entries, err := continuation.ListRegistryEntries(projectRoot)
	if err != nil {
		fmt.Fprintf(r.stderr, "evolve gc: continuation sweep: continuation registry unreadable: %v\n", err)
		return true
	}
	expired := expiredBindings(entries, last, maxAge)
	r.summary("%d continuation binding(s) older than %d cycles would be released", "released %d continuation binding(s) older than %d cycles", len(expired), maxAge)
	for _, id := range expired {
		fmt.Fprintf(r.stdout, "  %s cycle=%d snapshot=%s\n", id, entries[id].Cycle, entries[id].SnapshotSHA)
	}
	if r.dryRun || len(expired) == 0 {
		return false
	}
	leaves, failed := r.releaseExpired(projectRoot, expired, fmt.Sprintf("older than %d cycles", maxAge))
	return r.removeReleasedLanes(projectRoot, base, leaves) || failed
}

func expiredBindings(entries map[string]continuation.Continuation, last, maxAge int) []string {
	var expired []string
	for id, c := range entries {
		if c.Cycle > 0 && last-c.Cycle > maxAge {
			expired = append(expired, id)
		}
	}
	sort.Strings(expired)
	return expired
}

func (r gcRun) releaseExpired(projectRoot string, expired []string, reason string) (map[string]bool, bool) {
	opts := inboxmover.Options{ProjectRoot: projectRoot, Stderr: r.stderr}
	failed := false
	leaves := map[string]bool{}
	for _, id := range expired {
		c, released, err := inboxmover.ReleaseContinuationBinding(opts, id, reason, gcContinuationAuthority)
		if err != nil {
			fmt.Fprintf(r.stderr, "evolve gc: continuation sweep: release %s: %v\n", id, err)
			failed = true
			continue
		}
		if released && c.Worktree != "" {
			leaves[filepath.Base(c.Worktree)] = true
		}
	}
	return leaves, failed
}

func (r gcRun) removeReleasedLanes(projectRoot, base string, leaves map[string]bool) bool {
	lanes, err := planStaleLanes(r.ctx, projectRoot, base, time.Now())
	if err != nil {
		fmt.Fprintf(r.stderr, "evolve gc: continuation sweep: %v\n", err)
		return true
	}
	failed := false
	for _, l := range lanes {
		switch {
		case !leaves[l.leaf]:
		case l.keep != "":
			fmt.Fprintf(r.stdout, "  %s kept: %s path=%s\n", l.leaf, l.keep, l.path)
		default:
			failed = removeStaleLane(r.ctx, projectRoot, l, r.stdout, r.stderr) || failed
		}
	}
	return failed
}
