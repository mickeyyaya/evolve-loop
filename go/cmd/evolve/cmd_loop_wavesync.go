package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// waveSyncTimeout bounds the fetch — a wedged network must delay one wave by
// at most this, never hang the boundary.
const waveSyncTimeout = 60 * time.Second

var errMainCIRed = errors.New("main CI red")

type mainCheckRuns struct {
	CheckRuns []struct {
		Name       string `json:"name"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
	} `json:"check_runs"`
}

// mainCIRedForSHA reads the latest origin/main check-runs through the gh CLI.
// An unavailable API is returned as an error for the caller's loud fail-open
// path; only a completed failing conclusion is positive evidence of RED.
func mainCIRedForSHA(ctx context.Context, projectRoot, sha string) (bool, []string, error) {
	cmd := exec.CommandContext(ctx, "gh", "api", "--method", "GET",
		"repos/{owner}/{repo}/commits/"+sha+"/check-runs", "-f", "filter=latest", "-f", "per_page=100")
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, nil, fmt.Errorf("gh api check-runs: %w: %s", err, strings.TrimSpace(string(out)))
	}
	var payload mainCheckRuns
	if err := json.Unmarshal(out, &payload); err != nil {
		return false, nil, fmt.Errorf("decode check-runs: %w", err)
	}
	completed := 0
	var failed []string
	for _, run := range payload.CheckRuns {
		if run.Status != "completed" {
			continue
		}
		completed++
		switch run.Conclusion {
		case "success", "neutral", "skipped":
		default:
			failed = append(failed, run.Name+"="+run.Conclusion)
		}
	}
	if completed == 0 {
		return false, nil, fmt.Errorf("no completed check-runs visible for %.12s", sha)
	}
	return len(failed) > 0, failed, nil
}

// syncMainFromOriginAtWaveBoundary fetches origin and fast-forwards a
// checked-out `main` onto origin/main; synced is true only when the tree
// actually moved. A DIVERGED relation halts rather than skips, since
// gitexec.RelationToRemote is the one main-relation resolver laneStartRef
// also reads — continuing here would let the two consumers disagree about
// whether main is safe to lane from.
func syncMainFromOriginAtWaveBoundary(ctx context.Context, projectRoot string, warn io.Writer) (synced bool, halt error) {
	if info, err := plane.Classify(projectRoot); err != nil || info.Branch != "main" {
		return false, nil
	}
	sctx, cancel := context.WithTimeout(ctx, waveSyncTimeout)
	defer cancel()
	g := gitexec.Git{Dir: projectRoot, Exec: sysexec.DefaultRunner}
	if _, _, code, err := g.Capture(sctx, "remote", "get-url", "origin"); err != nil || code != 0 {
		return false, nil
	}
	if _, stderr, code, err := g.Capture(sctx, "fetch", "-q", "origin", "main"); err != nil || code != 0 {
		fmt.Fprintf(warn, "[loop] WARN: wave-boundary sync: fetch origin failed (rc=%d %v: %s) — planning against the local main\n", code, err, strings.TrimSpace(stderr))
		return false, nil
	}
	rel, err := g.RelationToRemote(sctx, "origin/main")
	if err != nil {
		fmt.Fprintf(warn, "[loop] WARN: wave-boundary sync: %v — planning against the local main\n", err)
		return false, nil
	}
	if rel.Kind != gitexec.RelationDiverged {
		red, failed, err := mainCIRedForSHA(sctx, projectRoot, rel.Remote)
		switch {
		case err != nil:
			fmt.Fprintf(warn, "[loop] WARN: wave-boundary sync: main CI status unavailable (%v) — proceeding without false-green or false-red classification\n", err)
		case red:
			return false, fmt.Errorf("%w at origin/main %.12s: %s", errMainCIRed, rel.Remote, strings.Join(failed, ", "))
		}
	}
	switch rel.Kind {
	case gitexec.RelationCurrent:
		return false, nil // the steady state stays silent
	case gitexec.RelationAhead:
		// --ff-only would "succeed" without moving HEAD here; say what is true.
		fmt.Fprintf(warn, "[loop] WARN: wave-boundary sync: %s; the next lane ship's push publishes them\n", rel)
		return false, nil
	case gitexec.RelationDiverged:
		fmt.Fprintf(warn, "[loop] WARN: wave-boundary sync: %s\n", rel)
		return false, fmt.Errorf("wave-boundary sync: %s", rel)
	}
	// Behind: fast-forward. --ff-only can still refuse when LOCAL TRACKED
	// CHANGES would be overwritten — a normal runtime state (rebuilt-binary
	// churn) that must be named as such, never as divergence (whose "next
	// ship reconciles" remedy is the stowaway class).
	if !fastForwardMain(sctx, g, warn) {
		return false, nil
	}
	fmt.Fprintf(warn, "[loop] wave-boundary sync: fast-forwarded main to origin/main (%.12s)\n", rel.Remote)
	return true, nil
}

func fastForwardMain(ctx context.Context, g gitexec.Git, warn io.Writer) bool {
	_, stderr, code, err := g.Capture(ctx, "merge", "--ff-only", "origin/main")
	if err == nil && code == 0 {
		return true
	}
	fmt.Fprintf(warn, "[loop] WARN: wave-boundary sync: local tracked changes block the fast-forward (rc=%d: %s) — resolve the dirt (or console-lease it) rather than shipping it\n", code, strings.TrimSpace(stderr))
	if blocking := blockingInboxFiles(ctx, g); len(blocking) > 0 {
		fmt.Fprintf(warn, "[loop] WARN: wave-boundary sync: %d plane-side inbox file(s) block it: %s — discard the plane copy once origin holds everything it carries, then run evolve sync-main (runtime-reference: the console route at a boundary)\n", len(blocking), strings.Join(blocking, ", "))
	}
	return false
}

const inboxPathspec = ".evolve/inbox/"

func blockingInboxFiles(ctx context.Context, g gitexec.Git) []string {
	local, _, code, err := g.Capture(ctx, "status", "--porcelain", "-z", "--untracked-files=all", "--", inboxPathspec)
	if err != nil || code != 0 {
		return nil
	}
	remote, _, code, err := g.Capture(ctx, "diff", "-z", "--no-renames", "--name-only", "HEAD", "origin/main", "--", inboxPathspec)
	if err != nil || code != 0 {
		return nil
	}
	changed := map[string]bool{}
	for _, path := range strings.Split(remote, "\x00") {
		changed[path] = true
	}
	var blocking []string
	for _, entry := range strings.Split(local, "\x00") {
		if len(entry) > 3 && entry[1] != 'D' && changed[entry[3:]] {
			blocking = append(blocking, entry[3:])
		}
	}
	sort.Strings(blocking)
	return blocking
}
