package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/scopedset"
)

const shadowStage = "shadow"

func recordScopedShadow(ctx context.Context, log io.Writer, worktree string, results map[string]ciparity.GateOutcome) {
	site, ok := core.RecheckSiteFrom(ctx)
	if !ok {
		return
	}
	rec := scopedset.Record{Stage: shadowStage, Cycle: site.Cycle, AuditedBase: site.AuditedBase}
	lane, laneOK := laneFiles(worktree)
	main, mainOK := mainFiles(ctx, worktree, site.AuditedBase)
	rec.Lane, rec.Main = lane, main
	set := scopedset.Set{All: true, Reason: "the lane change against main cannot be read"}
	if laneOK && !mainOK {
		set.Reason = "the new commits of main since " + site.AuditedBase + " cannot be read"
	}
	if laneOK && mainOK {
		set = scopedset.Compute(filepath.Clean(worktree), lane, main)
	}
	rec.All, rec.Reason, rec.Packages = set.All, set.Reason, set.Packages
	for _, gate := range ciparity.RequiredComposedGates {
		outcome, ran := results[gate]
		if !ran || outcome.Status != "fail" {
			continue
		}
		reds := outcome.Reds
		if len(reds.Keys) == 0 {
			reds.Unnamed = true
		}
		rec.Escapes = append(rec.Escapes, scopedset.Escapes(set, gate, reds)...)
	}
	if _, err := scopedset.Append(filepath.Join(site.ProjectRoot, ".evolve"), rec); err != nil {
		fmt.Fprintf(log, "[orchestrator] scoped re-check shadow record not written: %v; the full re-check stands\n", err)
	}
}

func laneFiles(worktree string) ([]string, bool) {
	files, ok := changedpkgs.ChangedFilesChecked(worktree, "main")
	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	return sortedPaths(paths), ok
}

func mainFiles(ctx context.Context, worktree, auditedBase string) ([]string, bool) {
	if auditedBase == "" {
		return nil, false
	}
	out, err := gitexec.Default(worktree).Output(ctx, "diff", "--name-only", auditedBase, "main")
	if err != nil {
		return nil, false
	}
	return sortedPaths(strings.Split(strings.TrimSpace(out), "\n")), true
}

func sortedPaths(paths []string) []string {
	set := map[string]struct{}{}
	var out []string
	for _, p := range paths {
		if p == "" {
			continue
		}
		if _, dup := set[p]; !dup {
			set[p] = struct{}{}
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}
