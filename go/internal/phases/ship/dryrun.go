// dryrun.go — DRY_RUN journal preview.
//
// Mirrors ship.sh lines 100-141. Every "would-be" mutation site appends
// a tag to the journal; on exit, the full journal is written to
// .evolve/release-journal/dry-run-<ts>.json so operators / parity audits
// can compare the planned ops against the actual sequence.
package ship

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// writeDryRunJournal emits .evolve/release-journal/dry-run-<ts>.json
// capturing the planned operations. Called once at end-of-Run when
// opts.DryRun is set. Best-effort: a missing/unwritable journal dir
// does not fail the run.
func writeDryRunJournal(ctx context.Context, opts *Options, res *RunResult, exitReason string) {
	if !opts.DryRun {
		return
	}
	dir := filepath.Join(opts.ProjectRoot, ".evolve", "release-journal")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	ts := time.Now().UTC().Format("20060102T150405Z")
	path := filepath.Join(dir, "dry-run-"+ts+".json")

	branch := tryGitOneShot(ctx, opts, "rev-parse", "--abbrev-ref", "HEAD")
	if branch == "" {
		branch = "unknown"
	}
	headSHA := tryGitOneShot(ctx, opts, "rev-parse", "HEAD")
	if headSHA == "" {
		headSHA = "unknown"
	}

	ops := []string{}
	for _, line := range res.Logs {
		if strings.Contains(line, "[DRY-RUN]") {
			ops = append(ops, strings.TrimSpace(line))
		}
	}

	body := map[string]any{
		"ts":                  ts,
		"class":               string(opts.Class),
		"branch":              branch,
		"head_sha_at_dry_run": headSHA,
		"commit_msg":          opts.CommitMessage,
		"exit_reason":         exitReason,
		"would_have":          ops,
	}
	buf, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(path, append(buf, '\n'), 0o644); err != nil {
		return
	}
	res.DryRunPath = path
	res.Logs = append(res.Logs, fmt.Sprintf("[ship] DRY-RUN: journal preview written to %s", path))
}

// tryGitOneShot is a fire-and-forget git probe. Empty on any error/exit.
func tryGitOneShot(ctx context.Context, opts *Options, args ...string) string {
	var buf strings.Builder
	exit, err := opts.Runner(ctx, "git", args, os.Environ(), opts.ProjectRoot, nil, &buf, io.Discard)
	if err != nil || exit != 0 {
		return ""
	}
	return strings.TrimSpace(buf.String())
}
