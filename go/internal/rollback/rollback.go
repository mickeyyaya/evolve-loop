// Package rollback reverts a failed release: it deletes the GitHub release and
// the remote tag, reverts the release commit, and records each step's outcome.
// See docs/architecture/packages/internal-rollback.md.
package rollback

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

var (
	ErrJournalNotFound  = errors.New("rollback: journal not found")
	ErrJournalMalformed = errors.New("rollback: journal malformed")
	ErrPartial          = errors.New("rollback: partial — at least one step failed")
)

type Journal struct {
	Version     string `json:"version"`
	Tag         string `json:"tag"`
	CommitSHA   string `json:"commit_sha"`
	Branch      string `json:"branch"`
	ReleaseURL  string `json:"release_url,omitempty"`
	StartedAt   string `json:"started_at,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
}

type Steps struct {
	GhDeleteRelease func(tag string) string

	DeleteRemoteTag func(repoRoot, tag string) string

	RevertAndShip func(repoRoot, commitSHA, reason, version string) string
}

type Options struct {
	JournalPath string
	Reason      string
	DryRun      bool
	RepoRoot    string
	LedgerPath  string
	Stderr      io.Writer

	Now   func() time.Time
	Steps Steps
}

type Result struct {
	Version          string
	Tag              string
	CommitSHA        string
	Reason           string
	ReleaseDelete    string
	TagDelete        string
	Revert           string
	DryRun           bool
	LedgerEntryJSON  string
	OverallSucceeded bool
}

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

const (
	stepDeleted           = "deleted"
	stepNotPresent        = "not-present"
	stepFailed            = "failed"
	stepSkipped           = "skipped"
	stepDryRunOK          = "dry-run-ok"
	stepReverted          = "reverted"
	stepRevertedLocalOnly = "local-only"
)

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

func DefaultSteps() Steps {
	return Steps{
		GhDeleteRelease: defaultGhDeleteRelease,
		DeleteRemoteTag: defaultDeleteRemoteTag,
		RevertAndShip:   defaultRevertAndShip,
	}
}

func dryRunSteps(logf func(string, ...any)) Steps {
	return Steps{
		GhDeleteRelease: func(tag string) string {
			logf("DRY-RUN: would gh release delete %s --yes", tag)
			return stepDryRunOK
		},
		DeleteRemoteTag: func(_, tag string) string {
			logf("DRY-RUN: would git push origin :refs/tags/%s", tag)
			return stepDryRunOK
		},
		RevertAndShip: func(_, sha, reason, version string) string {
			logf("DRY-RUN: would git revert --no-edit %s", sha)
			logf("DRY-RUN: would evolve ship --class manual \"revert: %s [rollback of v%s]\"",
				reason, version)
			return stepDryRunOK
		},
	}
}

func Run(opts Options) (Result, error) {
	res := Result{Reason: opts.Reason, DryRun: opts.DryRun}
	logf := newRollbackLogger(opts.Stderr)

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

	steps := resolveRollbackSteps(opts, logf)
	runRollbackSteps(&res, steps, opts, j, reason, logf)

	res.LedgerEntryJSON = buildRollbackLedgerEntry(j, reason, res, now)
	writeRollbackLedger(opts, res.LedgerEntryJSON, logf)

	return finalizeRollbackResult(&res, j, opts.DryRun, logf)
}

func newRollbackLogger(stderr io.Writer) func(string, ...any) {
	logw := stderr
	if logw == nil {
		logw = io.Discard
	}
	return func(format string, args ...any) {
		fmt.Fprintf(logw, "[rollback] "+format+"\n", args...)
	}
}

func resolveRollbackSteps(opts Options, logf func(string, ...any)) Steps {
	if opts.DryRun {
		return dryRunSteps(logf)
	}
	steps := opts.Steps
	if steps.GhDeleteRelease == nil {
		steps.GhDeleteRelease = defaultGhDeleteRelease
	}
	if steps.DeleteRemoteTag == nil {
		steps.DeleteRemoteTag = defaultDeleteRemoteTag
	}
	if steps.RevertAndShip == nil {
		steps.RevertAndShip = defaultRevertAndShip
	}
	return steps
}

func runRollbackSteps(res *Result, steps Steps, opts Options, j Journal, reason string, logf func(string, ...any)) {
	logf("step 1: delete GitHub release %s", j.Tag)
	res.ReleaseDelete = steps.GhDeleteRelease(j.Tag)
	logf("  → %s", res.ReleaseDelete)

	logf("step 2: delete remote tag %s", j.Tag)
	res.TagDelete = steps.DeleteRemoteTag(opts.RepoRoot, j.Tag)
	logf("  → %s", res.TagDelete)

	logf("step 3: create revert commit + push via ship.sh")
	res.Revert = steps.RevertAndShip(opts.RepoRoot, j.CommitSHA, reason, j.Version)
	logf("  → %s", res.Revert)
}

func buildRollbackLedgerEntry(j Journal, reason string, res Result, now func() time.Time) string {
	entry := LedgerEntry{
		Timestamp:     now().UTC().Format(time.RFC3339),
		Version:       j.Version,
		Tag:           j.Tag,
		CommitSHA:     j.CommitSHA,
		Reason:        reason,
		ReleaseDelete: res.ReleaseDelete,
		TagDelete:     res.TagDelete,
		Revert:        res.Revert,
		DryRun:        res.DryRun,
	}
	ledgerJSON, _ := json.Marshal(entry)
	return string(ledgerJSON)
}

func writeRollbackLedger(opts Options, ledgerJSON string, logf func(string, ...any)) {
	if opts.DryRun {
		logf("DRY-RUN: would append to ledger: %s", ledgerJSON)
		return
	}
	ledgerPath := opts.LedgerPath
	if ledgerPath == "" {
		ledgerPath = filepath.Join(opts.RepoRoot, ".evolve", "release-rollbacks.jsonl")
	}
	if err := appendLedger(ledgerPath, []byte(ledgerJSON)); err != nil {
		logf("WARN: failed to append rollback ledger: %v", err)
	}
}

func finalizeRollbackResult(res *Result, j Journal, dryRun bool, logf func(string, ...any)) (Result, error) {
	if dryRun {
		logf("DONE: dry-run complete for v%s", j.Version)
		res.OverallSucceeded = true
		return *res, nil
	}
	if res.Revert == stepReverted &&
		res.ReleaseDelete != stepFailed &&
		res.TagDelete != stepFailed {
		logf("DONE: rollback complete for v%s (release_delete=%s, tag_delete=%s, revert=%s)",
			j.Version, res.ReleaseDelete, res.TagDelete, res.Revert)
		res.OverallSucceeded = true
		return *res, nil
	}
	logf("PARTIAL: rollback incomplete (release_delete=%s, tag_delete=%s, revert=%s)",
		res.ReleaseDelete, res.TagDelete, res.Revert)
	return *res, fmt.Errorf("%w (release_delete=%s, tag_delete=%s, revert=%s)",
		ErrPartial, res.ReleaseDelete, res.TagDelete, res.Revert)
}

func appendLedger(path string, line []byte) (err error) {
	if err = os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	if _, werr := f.Write(line); werr != nil {
		return werr
	}
	if _, werr := f.Write([]byte("\n")); werr != nil {
		return werr
	}
	return nil
}

func defaultGhDeleteRelease(tag string) string {
	if _, err := exec.LookPath("gh"); err != nil {
		return stepSkipped
	}
	if err := sysexec.Command(context.Background(), "gh", "release", "view", tag).Run(); err != nil {
		return stepNotPresent
	}
	if err := sysexec.Command(context.Background(), "gh", "release", "delete", tag, "--yes").Run(); err != nil {
		return stepFailed
	}
	return stepDeleted
}

func defaultDeleteRemoteTag(repoRoot, tag string) string {
	return deleteRemoteTagWith(gitexec.Default(repoRoot), tag)
}

func deleteRemoteTagWith(g gitexec.Git, tag string) string {
	ctx := context.Background()
	out, _, _, _ := g.Capture(ctx, "ls-remote", "--tags", "origin", "refs/tags/"+tag)
	if !strings.Contains(out, tag) {
		_ = g.Run(ctx, "tag", "-d", tag)
		return stepNotPresent
	}
	if err := g.Run(ctx, "push", "origin", ":refs/tags/"+tag); err != nil {
		return stepFailed
	}
	_ = g.Run(ctx, "tag", "-d", tag)
	return stepDeleted
}

func defaultRevertAndShip(repoRoot, commitSHA, reason, version string) string {
	return revertAndShipWith(gitexec.Default(repoRoot), repoRoot, commitSHA, reason, version)
}

func revertAndShipWith(g gitexec.Git, repoRoot, commitSHA, reason, version string) string {
	if err := g.Run(context.Background(), "revert", "--no-edit", commitSHA); err != nil {
		return stepFailed
	}
	msg := fmt.Sprintf("revert: %s [rollback of v%s]", reason, version)
	binPath := resolveEvolveBinForRollback(repoRoot)
	if binPath == "" {
		return stepRevertedLocalOnly
	}
	cmd := sysexec.Command(context.Background(), binPath, "ship", "--class", "manual", msg)
	cmd.Env = append(os.Environ(),
		"EVOLVE_SHIP_AUTO_CONFIRM=1",
	)
	if err := cmd.Run(); err != nil {
		return stepRevertedLocalOnly
	}
	return stepReverted
}

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
