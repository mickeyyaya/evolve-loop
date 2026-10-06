package main

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/wtcheckpoint"
)

type checkpointSaveRow struct {
	wtcheckpoint.SaveResult
	Error  string `json:"error,omitempty"`
	Pushed bool   `json:"pushed,omitempty"`
}

func runCheckpointSave(args []string, stdout, stderr io.Writer) int {
	f := newCheckpointFlags("save", stderr)
	worktree := f.fs.String("worktree", "", "the linked worktree to save (default: --project-root)")
	all := f.fs.Bool("all", false, "save every linked worktree under the hub's dev/ directory (never the runtime plane or a cycle worktree)")
	label := f.fs.String("label", "", "text recorded in the checkpoint's commit message")
	push := f.fs.Bool("push", false, "also push the saved refs to origin refs/checkpoints/* (explicit only, never the default)")
	if !f.parse(args) || (*all && *worktree != "") {
		fmt.Fprintln(stderr, checkpointUsage)
		return 10
	}
	ctx := context.Background()
	hub, ok := f.hub(stderr)
	if !ok {
		return 1
	}
	var targets []wtcheckpoint.Worktree
	var err error
	if *all {
		targets, err = hub.DevWorktrees(ctx)
	} else {
		targets, err = singleWorktree(ctx, hub, cmp.Or(*worktree, *f.projectRoot))
	}
	if err != nil {
		fmt.Fprintf(stderr, "evolve checkpoint save: %v\n", err)
		return 1
	}
	rows := saveEach(ctx, targets, wtcheckpoint.SaveOptions{Label: *label, Keep: f.keepOrNoPrune(stderr), Now: checkpointClock})
	var pushErr error
	if *push {
		if rows, pushErr = pushSaved(ctx, hub, rows); pushErr != nil {
			fmt.Fprintf(stderr, "evolve checkpoint save: %v\n", pushErr)
		}
	}
	if code := reportSaves(f, rows, stdout, stderr); code != 0 || pushErr != nil {
		return 1
	}
	return 0
}

func singleWorktree(ctx context.Context, hub wtcheckpoint.Hub, dir string) ([]wtcheckpoint.Worktree, error) {
	w, err := hub.Worktree(ctx, dir)
	if err != nil {
		return nil, err
	}
	return []wtcheckpoint.Worktree{w}, nil
}

func saveEach(ctx context.Context, targets []wtcheckpoint.Worktree, o wtcheckpoint.SaveOptions) []checkpointSaveRow {
	rows := make([]checkpointSaveRow, 0, len(targets))
	for _, w := range targets {
		res, err := wtcheckpoint.Save(ctx, w, o)
		row := checkpointSaveRow{SaveResult: res}
		if err != nil {
			row.Error = err.Error()
		}
		rows = append(rows, row)
	}
	return rows
}

func pushSaved(ctx context.Context, hub wtcheckpoint.Hub, rows []checkpointSaveRow) ([]checkpointSaveRow, error) {
	var refs []string
	for _, r := range rows {
		if r.Ref != "" {
			refs = append(refs, r.Ref)
		}
	}
	if err := wtcheckpoint.Push(ctx, hub, refs); err != nil {
		return rows, err
	}
	pushed := make([]checkpointSaveRow, len(rows))
	for i, r := range rows {
		r.Pushed = r.Ref != ""
		pushed[i] = r
	}
	return pushed, nil
}

func reportSaves(f checkpointFlags, rows []checkpointSaveRow, stdout, stderr io.Writer) int {
	code := 0
	for _, r := range rows {
		if r.Error != "" {
			fmt.Fprintf(stderr, "evolve checkpoint save: %s: %s\n", r.Worktree, r.Error)
			code = 1
		}
	}
	if *f.asJSON {
		if !f.emitJSON(stdout, stderr, rows) {
			return 1
		}
		return code
	}
	for _, r := range rows {
		if r.Error == "" {
			fmt.Fprintln(stdout, saveLine(r))
		}
	}
	return code
}

func saveLine(r checkpointSaveRow) string {
	line := strings.TrimSpace(fmt.Sprintf("%s %s %s", r.Status, r.Worktree, r.Ref))
	if n := len(r.Pruned); n > 0 {
		line += fmt.Sprintf(" (pruned %d)", n)
	}
	if r.Pushed {
		line += " (pushed)"
	}
	return line
}

func runCheckpointList(args []string, stdout, stderr io.Writer) int {
	f := newCheckpointFlags("list", stderr)
	worktree := f.fs.String("worktree", "", "only this worktree's checkpoints; the directory may already be gone, its base name is used")
	if !f.parse(args) {
		fmt.Fprintln(stderr, checkpointUsage)
		return 10
	}
	ctx := context.Background()
	hub, ok := f.hub(stderr)
	if !ok {
		return 1
	}
	entries, err := wtcheckpoint.List(ctx, hub, *worktree)
	if err != nil {
		fmt.Fprintf(stderr, "evolve checkpoint list: %v\n", err)
		return 1
	}
	if *f.asJSON {
		if !f.emitJSON(stdout, stderr, entries) {
			return 1
		}
		return 0
	}
	for _, e := range entries {
		fmt.Fprintln(stdout, listLine(e))
	}
	return 0
}

func listLine(e wtcheckpoint.Entry) string {
	line := fmt.Sprintf("%s  %s  base %s  %d file(s) +%d -%d", e.Ref, e.Time.Format("2006-01-02T15:04:05Z"), shortSHA(e.Base), e.Files, e.Added, e.Deleted)
	if e.Label != "" {
		line += "  label: " + e.Label
	}
	return line
}

func runCheckpointRestore(args []string, stdout, stderr io.Writer) int {
	ref, rest := "", args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		ref, rest = args[0], args[1:]
	}
	f := newCheckpointFlags("restore", stderr)
	into := f.fs.String("into", "", "a new path, or a clean linked worktree, to restore into")
	if f.fs.Parse(rest) != nil || f.fs.NArg() > 1 || (ref != "" && f.fs.NArg() > 0) {
		fmt.Fprintln(stderr, checkpointUsage)
		return 10
	}
	if ref == "" {
		ref = f.fs.Arg(0)
	}
	if ref == "" || *into == "" {
		fmt.Fprintln(stderr, "evolve checkpoint restore: needs a checkpoint ref and --into DIR\n"+checkpointUsage)
		return 10
	}
	ctx := context.Background()
	hub, ok := f.hub(stderr)
	if !ok {
		return 1
	}
	res, err := wtcheckpoint.Restore(ctx, hub, ref, *into)
	if err != nil {
		fmt.Fprintf(stderr, "evolve checkpoint restore: %v\n", err)
		return 1
	}
	if *f.asJSON {
		if !f.emitJSON(stdout, stderr, res) {
			return 1
		}
		return 0
	}
	fmt.Fprintln(stdout, restoreLine(res))
	return 0
}

func restoreLine(r wtcheckpoint.RestoreResult) string {
	line := fmt.Sprintf("restored %s into %s at base %s", r.Ref, r.Dir, shortSHA(r.Base))
	if r.Created {
		line = fmt.Sprintf("restored %s into %s, a new worktree, at base %s", r.Ref, r.Dir, shortSHA(r.Base))
	}
	if r.Detached {
		line += "; HEAD is detached there (no branch ref was moved): run `git switch -c <branch>` to keep working on a branch"
	}
	return line
}

func runCheckpointPrune(args []string, stdout, stderr io.Writer) int {
	f := newCheckpointFlags("prune", stderr)
	landed := f.fs.Bool("landed", false, "also delete each checkpoint whose every changed path already matches origin/main")
	if !f.parse(args) {
		fmt.Fprintln(stderr, checkpointUsage)
		return 10
	}
	ctx := context.Background()
	hub, ok := f.hub(stderr)
	if !ok {
		return 1
	}
	keep, err := f.keep()
	if err != nil {
		fmt.Fprintf(stderr, "evolve checkpoint prune: refused: %v\n", err)
		return 1
	}
	pruned, err := wtcheckpoint.Prune(ctx, hub, wtcheckpoint.PruneOptions{Keep: keep, Landed: *landed})
	printed := reportPrunes(f, pruned, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "evolve checkpoint prune: %v\n", err)
	}
	if err != nil || !printed {
		return 1
	}
	return 0
}

func reportPrunes(f checkpointFlags, pruned []wtcheckpoint.Pruned, stdout, stderr io.Writer) bool {
	if *f.asJSON {
		return f.emitJSON(stdout, stderr, pruned)
	}
	for _, p := range pruned {
		fmt.Fprintf(stdout, "pruned %s (%s)\n", p.Ref, p.Reason)
	}
	fmt.Fprintf(stdout, "pruned %d checkpoint(s)\n", len(pruned))
	return true
}
