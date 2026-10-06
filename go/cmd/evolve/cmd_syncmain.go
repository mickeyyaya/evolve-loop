package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxstamps"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

var syncMainFetchSleep = time.Sleep

func runSyncMain(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve sync-main", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var projectRoot string
	fs.StringVar(&projectRoot, "project-root", "", "project root (default: $EVOLVE_PROJECT_ROOT or cwd)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if projectRoot == "" {
		projectRoot = os.Getenv("EVOLVE_PROJECT_ROOT")
	}
	if projectRoot == "" {
		var err error
		if projectRoot, err = os.Getwd(); err != nil {
			fmt.Fprintf(stderr, "evolve sync-main: cwd: %v\n", err)
			return 1
		}
	}
	absRoot := paths.AbsoluteRoot("--project-root", projectRoot, func(m string) {
		fmt.Fprintf(stderr, "evolve sync-main: WARN: %s\n", m)
	})

	git := func(gitArgs ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", absRoot}, gitArgs...)...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	if ws := liveLeaseWorkspace(absRoot); ws != "" {
		if lease, ok := runlease.LiveOwner(ws, time.Now()); ok {
			fmt.Fprintf(stderr, "evolve sync-main: refused — a run lease is live (pid %d, heartbeat fresh); another evolve loop owns this tree.\n", lease.OwnerPID)
			fmt.Fprintln(stderr, "evolve sync-main:   • let it finish, or `evolve loop --resume` to attach, then retry.")
			return 1
		}
	}

	stamps, code := refuseDirtBeyondInboxStamps(absRoot, stderr)
	if code != 0 {
		return code
	}

	branch, err := git("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		fmt.Fprintf(stderr, "evolve sync-main: cannot resolve current branch: %v\n%s", err, branch)
		return 1
	}
	branch = strings.TrimSpace(branch)

	if err := fetchSyncBranch(absRoot, branch, stderr); err != nil {
		fmt.Fprintf(stderr, "evolve sync-main: %v\n", err)
		return 1
	}

	landing := stampLanding{root: absRoot, stdout: stdout, stderr: stderr}
	merge := func() int { return mergeOrigin(git, branch, stderr) }
	if code := landing.aroundMerge("origin/"+branch, stamps, merge); code != 0 {
		return code
	}

	fmt.Fprintf(stdout, "sync-main: reconciled local %s with origin/%s (merge only; nothing pushed).\n", branch, branch)
	return 0
}

func fetchSyncBranch(root, branch string, stderr io.Writer) error {
	if branch == "HEAD" {
		return errors.New("refused — detached HEAD; check out the branch to sync")
	}
	retry := gitexec.FetchRetry{
		Sleep: func(d time.Duration) { syncMainFetchSleep(d) },
		OnRetry: func(attempt, attempts, code int, _ string) {
			fmt.Fprintf(stderr, "evolve sync-main: retry %d/%d: git fetch origin %s after ref-lock contention rc=%d\n", attempt, attempts-1, branch, code)
		},
	}
	_, gitErr, code, err := gitexec.Default(root).FetchOriginBranch(context.Background(), retry, branch)
	if err != nil || code != 0 {
		return fmt.Errorf("git fetch origin %s failed: rc=%d err=%v: %s", branch, code, err, strings.TrimSpace(gitErr))
	}
	return nil
}

func mergeOrigin(git func(...string) (string, error), branch string, stderr io.Writer) int {
	out, err := git("merge", "--no-edit", "origin/"+branch)
	if err == nil {
		return 0
	}
	if _, inProgress := git("rev-parse", "-q", "--verify", "MERGE_HEAD"); inProgress != nil {
		fmt.Fprintf(stderr, "evolve sync-main: refused — git would not merge origin/%s:\n%s", branch, out)
		return 1
	}
	if _, aerr := git("merge", "--abort"); aerr != nil {
		fmt.Fprintf(stderr, "evolve sync-main: merge conflicted AND abort failed: %v\n%s", aerr, out)
		return 1
	}
	fmt.Fprintf(stderr, "evolve sync-main: refused — merging origin/%s conflicts with local history (aborted cleanly, tree unchanged).\n", branch)
	fmt.Fprintln(stderr, "evolve sync-main:   • resolve manually, or re-audit the local commit on the new base.")
	return 1
}

// Matches gc.discover's minimal cycle_id/workspace schema rather than
// depending on the heavier core loader.
func liveLeaseWorkspace(root string) string {
	b, err := os.ReadFile(filepath.Join(root, ".evolve", "cycle-state.json"))
	if err != nil {
		return ""
	}
	var cs struct {
		WorkspacePath string `json:"workspace_path"`
	}
	if json.Unmarshal(b, &cs) != nil {
		return ""
	}
	return cs.WorkspacePath
}

func refuseDirtBeyondInboxStamps(root string, stderr io.Writer) ([]inboxstamps.Stamp, int) {
	partition, err := inboxstamps.Classify(context.Background(), gitexec.Default(root))
	if err != nil {
		fmt.Fprintf(stderr, "evolve sync-main: reading the tree's changes failed: %v\n", err)
		return nil, 1
	}
	if len(partition.Other) > 0 {
		fmt.Fprintf(stderr, "evolve sync-main: refused — tracked files have uncommitted changes beyond the loop's inbox stamps; commit or stash first:\n  %s\n", strings.Join(partition.Other, "\n  "))
		return nil, 1
	}
	return partition.Stamps, 0
}

type stampLanding struct {
	root           string
	stdout, stderr io.Writer
}

func (l stampLanding) aroundMerge(remoteRef string, stamps []inboxstamps.Stamp, merge func() int) int {
	plan, code := l.prepare(remoteRef, stamps)
	if code != 0 {
		return code
	}
	if code := merge(); code != 0 {
		l.restore(plan)
		return code
	}
	return l.replay(plan)
}

func (l stampLanding) prepare(remoteRef string, stamps []inboxstamps.Stamp) (inboxstamps.Plan, int) {
	ctx, g := context.Background(), gitexec.Default(l.root)
	plan, err := inboxstamps.PlanAgainst(ctx, g, stamps, remoteRef)
	if err == nil {
		err = plan.Prepare(ctx, g)
	}
	if err == nil {
		err = plan.CommitKept(ctx, g)
	}
	if err != nil {
		fmt.Fprintf(l.stderr, "evolve sync-main: landing the loop's inbox stamps failed: %v\n", err)
		l.restore(plan)
		return inboxstamps.Plan{}, 1
	}
	reportInboxStamps(l.stdout, "committed %d inbox stamp(s) the loop wrote", plan.Kept)
	return plan, 0
}

func (l stampLanding) replay(plan inboxstamps.Plan) int {
	reportInboxStamps(l.stdout, "dropped %d inbox stamp(s) on items origin retired", plan.Retired)
	reportInboxStamps(l.stdout, "origin superseded %d inbox stamp(s), changing every field they set", plan.Superseded)
	err := plan.ApplyReplay(l.root)
	if err == nil {
		err = plan.CommitReplay(context.Background(), gitexec.Default(l.root))
	}
	if err != nil {
		fmt.Fprintf(l.stderr, "evolve sync-main: replaying the loop's inbox stamps onto origin's edits failed: %v — every stamp named above was not applied; the rest are in the tree, uncommitted\n", err)
		return 1
	}
	reportInboxStamps(l.stdout, "replayed %d inbox stamp(s) onto origin's edits", plan.Replay)
	return 0
}

func (l stampLanding) restore(plan inboxstamps.Plan) {
	if err := plan.Restore(l.root); err != nil {
		fmt.Fprintf(l.stderr, "evolve sync-main: restoring the loop's inbox stamps failed: %v\n", err)
	}
}

func reportInboxStamps(w io.Writer, format string, stamps []inboxstamps.Stamp) {
	if len(stamps) > 0 {
		fmt.Fprintf(w, "sync-main: "+format+": %s\n", len(stamps), strings.Join(inboxstamps.Paths(stamps), ", "))
	}
}
