// Package rollback ports legacy/scripts/release/rollback.sh.
//
// Auto-revert a failed release in three independently-auditable steps:
//
//  1. Delete the GitHub Release (gh release delete vX.Y.Z)
//  2. Delete the remote tag (git push origin :refs/tags/vX.Y.Z)
//  3. Create a revert commit and push it via ship.sh (with
//     EVOLVE_BYPASS_SHIP_VERIFY=1, since the original audit was bound to
//     the now-reverted tree)
//
// Each step's status is appended as one NDJSON line to
// .evolve/release-rollbacks.jsonl for audit trail.
//
// MEDIUM-1 fix (audit cycle 8202): the script previously exited 0 when
// step 3 succeeded even if steps 1 or 2 had FAILED — masking dangling
// release/tag incidents. Post-fix: any "failed" step (not just step 3
// success) blocks exit 0.
//
// Exit codes (mapped by cmd layer):
//
//	0  — rollback complete (all 3 steps succeeded or were legitimately skipped)
//	1  — rollback partial (some step failed; ledger entry written)
//	2  — journal not found / malformed
//	10 — invalid arguments (cmd layer)
package rollback

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Sentinel errors.
var (
	ErrJournalNotFound = errors.New("rollback: journal not found")
	ErrJournalMalformed = errors.New("rollback: journal malformed")
	ErrPartial          = errors.New("rollback: partial — at least one step failed")
)

// Journal is the per-publish record written by release-pipeline.sh and read
// here. Fields with empty values are treated as malformed.
type Journal struct {
	Version    string `json:"version"`
	Tag        string `json:"tag"`
	CommitSHA  string `json:"commit_sha"`
	Branch     string `json:"branch"`
	ReleaseURL string `json:"release_url,omitempty"`
	StartedAt  string `json:"started_at,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
}

// Steps holds the injectable step implementations. Defaults shell out to
// real gh/git/ship.sh.
type Steps struct {
	// GhDeleteRelease deletes the GitHub release. Returns one of:
	//   "deleted" — release existed, delete succeeded
	//   "not-present" — release did not exist (already deleted upstream)
	//   "failed" — gh release delete returned non-zero
	//   "skipped" — gh CLI not installed
	GhDeleteRelease func(tag string) string

	// DeleteRemoteTag deletes the remote tag. Returns:
	//   "deleted" | "not-present" | "failed" | "skipped" (dry-run only)
	// Implementations should also best-effort delete the local tag.
	DeleteRemoteTag func(repoRoot, tag string) string

	// RevertAndShip creates the revert commit (git revert --no-edit) and
	// pushes via ship.sh with EVOLVE_BYPASS_SHIP_VERIFY=1. Returns:
	//   "reverted" — revert + push both succeeded
	//   "local-only" — revert succeeded; ship.sh push failed
	//   "failed" — git revert failed (no commit made)
	RevertAndShip func(repoRoot, commitSHA, reason, version string) string
}

// Options drives a Run() invocation.
type Options struct {
	JournalPath string
	Reason      string
	DryRun      bool
	RepoRoot    string
	LedgerPath  string // defaulted to <RepoRoot>/.evolve/release-rollbacks.jsonl
	Stderr      io.Writer

	Now   func() time.Time
	Steps Steps
}

// Result captures per-step outcomes and the overall success.
type Result struct {
	Version          string
	Tag              string
	CommitSHA        string
	Reason           string
	ReleaseDelete    string // step 1 status
	TagDelete        string // step 2 status
	Revert           string // step 3 status
	DryRun           bool
	LedgerEntryJSON  string
	OverallSucceeded bool
}

// LedgerEntry is what gets appended to release-rollbacks.jsonl per attempt.
type LedgerEntry struct {
	Timestamp     string `json:"timestamp"`
	Version       string `json:"version"`
	Tag           string `json:"tag"`
	CommitSHA     string `json:"commit_sha"`
	Reason        string `json:"reason"`
	ReleaseDelete string `json:"release_delete"`
	TagDelete     string `json:"tag_delete"`
	Revert        string `json:"revert"`
	DryRun        bool   `json:"dry_run"`
}

// ReadJournal loads + validates a journal JSON file.
func ReadJournal(path string) (Journal, error) {
	var j Journal
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return j, fmt.Errorf("%w: %s", ErrJournalNotFound, path)
		}
		return j, fmt.Errorf("%w: read failed: %v", ErrJournalMalformed, err)
	}
	if err := json.Unmarshal(body, &j); err != nil {
		return j, fmt.Errorf("%w: %v", ErrJournalMalformed, err)
	}
	if j.Version == "" {
		return j, fmt.Errorf("%w: missing 'version': %s", ErrJournalMalformed, path)
	}
	if j.Tag == "" {
		return j, fmt.Errorf("%w: missing 'tag': %s", ErrJournalMalformed, path)
	}
	if j.CommitSHA == "" {
		return j, fmt.Errorf("%w: missing 'commit_sha': %s", ErrJournalMalformed, path)
	}
	if j.Branch == "" {
		return j, fmt.Errorf("%w: missing 'branch': %s", ErrJournalMalformed, path)
	}
	return j, nil
}

// DefaultSteps wires the production gh/git/ship.sh implementations.
func DefaultSteps() Steps {
	return Steps{
		GhDeleteRelease:  defaultGhDeleteRelease,
		DeleteRemoteTag:  defaultDeleteRemoteTag,
		RevertAndShip:    defaultRevertAndShip,
	}
}

// dryRunSteps returns Steps that announce intent but never mutate.
func dryRunSteps(logf func(string, ...any)) Steps {
	return Steps{
		GhDeleteRelease: func(tag string) string {
			logf("DRY-RUN: would gh release delete %s --yes", tag)
			return "dry-run-ok"
		},
		DeleteRemoteTag: func(_, tag string) string {
			logf("DRY-RUN: would git push origin :refs/tags/%s", tag)
			return "dry-run-ok"
		},
		RevertAndShip: func(_, sha, reason, version string) string {
			logf("DRY-RUN: would git revert --no-edit %s", sha)
			logf("DRY-RUN: would EVOLVE_BYPASS_SHIP_VERIFY=1 bash ship.sh \"revert: %s [rollback of v%s]\"",
				reason, version)
			return "dry-run-ok"
		},
	}
}

// Run executes the rollback pipeline. Returns Result and error.
func Run(opts Options) (Result, error) {
	res := Result{Reason: opts.Reason, DryRun: opts.DryRun}

	logw := opts.Stderr
	if logw == nil {
		logw = io.Discard
	}
	logf := func(format string, args ...any) {
		fmt.Fprintf(logw, "[rollback] "+format+"\n", args...)
	}

	if opts.JournalPath == "" {
		return res, fmt.Errorf("%w: JournalPath required", ErrJournalNotFound)
	}

	j, err := ReadJournal(opts.JournalPath)
	if err != nil {
		return res, err
	}
	res.Version, res.Tag, res.CommitSHA = j.Version, j.Tag, j.CommitSHA

	logf("rolling back v%s (%s @ %s on %s)", j.Version, j.Tag, j.CommitSHA, j.Branch)
	reason := opts.Reason
	if reason == "" {
		reason = "release-pipeline failure"
		res.Reason = reason
	}
	logf("reason: %s", reason)

	now := opts.Now
	if now == nil {
		now = time.Now
	}

	// Pick step implementations.
	steps := opts.Steps
	if opts.DryRun {
		steps = dryRunSteps(logf)
	} else {
		if steps.GhDeleteRelease == nil {
			steps.GhDeleteRelease = defaultGhDeleteRelease
		}
		if steps.DeleteRemoteTag == nil {
			steps.DeleteRemoteTag = defaultDeleteRemoteTag
		}
		if steps.RevertAndShip == nil {
			steps.RevertAndShip = defaultRevertAndShip
		}
	}

	// Step 1: GitHub release delete.
	logf("step 1: delete GitHub release %s", j.Tag)
	res.ReleaseDelete = steps.GhDeleteRelease(j.Tag)
	logf("  → %s", res.ReleaseDelete)

	// Step 2: remote tag delete.
	logf("step 2: delete remote tag %s", j.Tag)
	res.TagDelete = steps.DeleteRemoteTag(opts.RepoRoot, j.Tag)
	logf("  → %s", res.TagDelete)

	// Step 3: revert + ship.
	logf("step 3: create revert commit + push via ship.sh")
	res.Revert = steps.RevertAndShip(opts.RepoRoot, j.CommitSHA, reason, j.Version)
	logf("  → %s", res.Revert)

	// Append ledger entry (dry-run skips disk write).
	entry := LedgerEntry{
		Timestamp:     now().UTC().Format(time.RFC3339),
		Version:       j.Version,
		Tag:           j.Tag,
		CommitSHA:     j.CommitSHA,
		Reason:        reason,
		ReleaseDelete: res.ReleaseDelete,
		TagDelete:     res.TagDelete,
		Revert:        res.Revert,
		DryRun:        opts.DryRun,
	}
	ledgerJSON, _ := json.Marshal(entry)
	res.LedgerEntryJSON = string(ledgerJSON)

	if !opts.DryRun {
		ledgerPath := opts.LedgerPath
		if ledgerPath == "" {
			ledgerPath = filepath.Join(opts.RepoRoot, ".evolve", "release-rollbacks.jsonl")
		}
		if err := appendLedger(ledgerPath, ledgerJSON); err != nil {
			logf("WARN: failed to append rollback ledger: %v", err)
		}
	} else {
		logf("DRY-RUN: would append to ledger: %s", string(ledgerJSON))
	}

	// Overall success determination — MEDIUM-1 fix.
	if opts.DryRun {
		logf("DONE: dry-run complete for v%s", j.Version)
		res.OverallSucceeded = true
		return res, nil
	}
	if res.Revert == "reverted" &&
		res.ReleaseDelete != "failed" &&
		res.TagDelete != "failed" {
		logf("DONE: rollback complete for v%s (release_delete=%s, tag_delete=%s, revert=%s)",
			j.Version, res.ReleaseDelete, res.TagDelete, res.Revert)
		res.OverallSucceeded = true
		return res, nil
	}
	logf("PARTIAL: rollback incomplete (release_delete=%s, tag_delete=%s, revert=%s)",
		res.ReleaseDelete, res.TagDelete, res.Revert)
	return res, fmt.Errorf("%w (release_delete=%s, tag_delete=%s, revert=%s)",
		ErrPartial, res.ReleaseDelete, res.TagDelete, res.Revert)
}

// appendLedger ensures the parent dir exists then appends one NDJSON line.
func appendLedger(path string, line []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(line); err != nil {
		return err
	}
	if _, err := f.Write([]byte("\n")); err != nil {
		return err
	}
	return nil
}

// --- Default step implementations ----------------------------------------

func defaultGhDeleteRelease(tag string) string {
	if _, err := exec.LookPath("gh"); err != nil {
		return "skipped"
	}
	if err := exec.Command("gh", "release", "view", tag).Run(); err != nil {
		return "not-present"
	}
	if err := exec.Command("gh", "release", "delete", tag, "--yes").Run(); err != nil {
		return "failed"
	}
	return "deleted"
}

func defaultDeleteRemoteTag(repoRoot, tag string) string {
	out, _ := exec.Command("git", "-C", repoRoot, "ls-remote", "--tags", "origin", "refs/tags/"+tag).Output()
	if !strings.Contains(string(out), tag) {
		// Best-effort local cleanup even when not on remote.
		_ = exec.Command("git", "-C", repoRoot, "tag", "-d", tag).Run()
		return "not-present"
	}
	if err := exec.Command("git", "-C", repoRoot, "push", "origin", ":refs/tags/"+tag).Run(); err != nil {
		return "failed"
	}
	_ = exec.Command("git", "-C", repoRoot, "tag", "-d", tag).Run()
	return "deleted"
}

func defaultRevertAndShip(repoRoot, commitSHA, reason, version string) string {
	if err := exec.Command("git", "-C", repoRoot, "revert", "--no-edit", commitSHA).Run(); err != nil {
		return "failed"
	}
	msg := fmt.Sprintf("revert: %s [rollback of v%s]", reason, version)
	// v12.0.0+: native evolve ship required (no bash fallback). Revert
	// commit is local-only if the binary is unavailable.
	binPath := resolveEvolveBinForRollback(repoRoot)
	if binPath == "" {
		return "local-only"
	}
	cmd := exec.Command(binPath, "ship", "--class", "manual", msg)
	cmd.Env = append(os.Environ(),
		"EVOLVE_BYPASS_SHIP_VERIFY=1",
		"EVOLVE_SHIP_AUTO_CONFIRM=1",
	)
	if err := cmd.Run(); err != nil {
		return "local-only"
	}
	return "reverted"
}

// resolveEvolveBinForRollback locates the native evolve binary for rollback's
// revert-and-ship flow. Mirrors releasepipeline.resolveEvolveBin.
func resolveEvolveBinForRollback(repoRoot string) string {
	if p := os.Getenv("EVOLVE_GO_BIN"); p != "" {
		if info, err := os.Stat(p); err == nil && info.Mode()&0o111 != 0 {
			return p
		}
	}
	candidate := filepath.Join(repoRoot, "go", "bin", "evolve")
	if info, err := os.Stat(candidate); err == nil && info.Mode()&0o111 != 0 {
		return candidate
	}
	if found, err := exec.LookPath("evolve"); err == nil {
		return found
	}
	return ""
}
