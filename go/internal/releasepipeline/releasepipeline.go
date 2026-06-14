// Package releasepipeline ports legacy/scripts/release-pipeline.sh.
//
// Self-healing release pipeline driver — the single declarative entry point
// for "publish a new release." Composes the 5 already-ported release Go
// libraries (releasepreflight, changeloggen, versionbump, marketplacepoll,
// rollback) plus shell-outs to release.sh (consistency check) and ship.sh
// (atomic commit+push+gh-release-create) into the full pipeline.
//
// Lifecycle (each step is a no-op when DryRun):
//
//  0. (optional) full-dry-run preflight        [legacy bash; only when RequirePreflight]
//  1. release preflight (5 gates)              [Go: releasepreflight.Run]
//  2. changelog-gen                            [Go: changeloggen.Run]
//  3. version-bump                             [Go: versionbump.Run]
//  4. release.sh consistency check             [bash: legacy/scripts/utility/release.sh]
//  5. ship.sh --class release                  [bash: legacy/scripts/lifecycle/ship.sh]
//  6. marketplace-poll                         [Go: marketplacepoll.Run]
//     on failure → auto-rollback               [Go: rollback.Run]
//
// Journal: .evolve/release-journal/<version>-<ts>.json — one file per attempt.
// rollback.Run reads it to know what to undo.
//
// Exit codes (cmd layer maps from sentinel errors):
//
//	0  — published + propagated successfully
//	1  — pre-publish step failed (preflight, bump, changelog, release.sh)
//	2  — ship.sh failed (nothing went out; no rollback needed)
//	3  — post-publish (poll/refresh) failed; auto-rollback ran or was skipped
//	10 — invalid arguments (handled in cmd layer)
package releasepipeline

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/semvercheck"
)

// Sentinel errors. The cmd layer maps these to exit codes.
var (
	ErrPrePublishFailed  = errors.New("releasepipeline: pre-publish step failed")
	ErrShipFailed        = errors.New("releasepipeline: ship.sh failed")
	ErrPostPublishFailed = errors.New("releasepipeline: post-publish step failed")
)

// Steps is the injectable composition of step functions. Each returns the
// step status (used for journal logging and overall outcome reporting).
// Defaults call into the real Go libraries / bash scripts.
type Steps struct {
	// FullDryRunPreflight runs when RequirePreflight is true (step 0).
	FullDryRunPreflight func(repoRoot, target string) error

	Preflight    func(repoRoot, target string, dryRun, skipTests bool) error
	ChangelogGen func(repoRoot, fromRef, toRef, target string, dryRun bool) error
	VersionBump  func(repoRoot, target string, dryRun bool) error
	// RebuildBinary runs `go build` with the Makefile-equivalent ldflags
	// (pkg/version.version=<target> + commit + builtAt, from <RepoRoot>/go,
	// output go/evolve) so the binary tracked at go/evolve is in sync with
	// the version-bumped release AND self-reports the target version.
	// Without this step, `evolve release X.Y.Z` ships source but leaves
	// the marketplace binary frozen at the previous build. Source incident:
	// v12.2.1 shipped source 2026-05-26 but marketplace binary stayed at
	// v12.1.1 (2026-05-25). The Ship step (--class release) stages the
	// rebuilt binary as part of the explicit release set.
	RebuildBinary   func(repoRoot, target string, dryRun bool) error
	ReleaseSh       func(repoRoot, target string) error // consistency check
	Ship            func(repoRoot, msg, releaseNotes string) (newSHA string, err error)
	MarketplacePoll func(repoRoot, target string, maxWait time.Duration) error
	Rollback        func(repoRoot, journalPath, reason string) error
	// ReleaseVerify is the terminal self-consistency proof (inbox
	// release-rebuild-binary-not-committed, v18.3.0→v18.5.0 recurrence):
	// tracked go/evolve on disk == the blob at <commitSHA>:go/evolve ==
	// state.json:expected_ship_sha (re-pinned to the committed blob when
	// stale — releases never went through repinPostCycle, which is
	// cycle-class-only), `go/evolve --version` contains <target>, and the
	// local tag v<target> exists at the release commit (created when the
	// gh-side release left it remote-only). Failure → post-publish error
	// (auto-rollback unless --no-rollback).
	ReleaseVerify func(repoRoot, target, commitSHA string) error
}

// Options drives a Run() invocation.
type Options struct {
	Target           string
	RepoRoot         string
	DryRun           bool
	NoRollback       bool
	SkipTests        bool
	RequirePreflight bool
	MaxPollWait      time.Duration
	FromTag          string // optional; auto-derived from `git describe --tags --abbrev=0` if empty
	JournalDir       string // defaulted to <RepoRoot>/.evolve/release-journal
	Stderr           io.Writer

	Now   func() time.Time
	Steps Steps
}

// Result captures per-step outcomes + the final journal path.
type Result struct {
	Target            string
	JournalPath       string
	StepsCompleted    []string
	StepsFailed       []string
	NewCommitSHA      string
	RollbackTriggered bool
	RollbackErr       error
}

// Journal is the on-disk per-publish record. release-pipeline.sh stores
// {version, tag, commit_sha, branch, release_url, started_at, completed_at,
// steps}. We mirror that schema for rollback.ReadJournal compat.
type Journal struct {
	Version     string       `json:"version"`
	Tag         string       `json:"tag"`
	CommitSHA   string       `json:"commit_sha"`
	Branch      string       `json:"branch"`
	ReleaseURL  string       `json:"release_url"`
	StartedAt   string       `json:"started_at"`
	CompletedAt string       `json:"completed_at"`
	Steps       []StepRecord `json:"steps"`
}

// StepRecord is one entry in journal.steps[].
type StepRecord struct {
	Step      string `json:"step"`
	Status    string `json:"status"`
	Note      string `json:"note,omitempty"`
	Timestamp string `json:"timestamp"`
}

// DefaultSteps wires real Go libraries / shell-outs for production use.
// Callers should NOT use DefaultSteps in tests — pass injected stubs via Options.Steps.
func DefaultSteps() Steps {
	return Steps{
		FullDryRunPreflight: defaultFullDryRunPreflight,
		Preflight:           defaultPreflight,
		ChangelogGen:        defaultChangelogGen,
		VersionBump:         defaultVersionBump,
		RebuildBinary:       defaultRebuildBinary,
		ReleaseSh:           defaultReleaseSh,
		Ship:                defaultShip,
		MarketplacePoll:     defaultMarketplacePoll,
		Rollback:            defaultRollback,
		ReleaseVerify:       defaultReleaseVerify,
	}
}

// applyDefaultSteps returns s with any nil function field replaced by the
// corresponding DefaultSteps() implementation. Callers in tests supply stubs;
// production callers supply a zero Steps{} and get the full default set.
func applyDefaultSteps(s Steps) Steps {
	d := DefaultSteps()
	if s.FullDryRunPreflight == nil {
		s.FullDryRunPreflight = d.FullDryRunPreflight
	}
	if s.Preflight == nil {
		s.Preflight = d.Preflight
	}
	if s.ChangelogGen == nil {
		s.ChangelogGen = d.ChangelogGen
	}
	if s.VersionBump == nil {
		s.VersionBump = d.VersionBump
	}
	if s.RebuildBinary == nil {
		s.RebuildBinary = d.RebuildBinary
	}
	if s.ReleaseSh == nil {
		s.ReleaseSh = d.ReleaseSh
	}
	if s.Ship == nil {
		s.Ship = d.Ship
	}
	if s.MarketplacePoll == nil {
		s.MarketplacePoll = d.MarketplacePoll
	}
	if s.Rollback == nil {
		s.Rollback = d.Rollback
	}
	if s.ReleaseVerify == nil {
		s.ReleaseVerify = d.ReleaseVerify
	}
	return s
}

// Run executes the pipeline. Returns Result + error mapped to bash exit codes.
func Run(opts Options) (Result, error) {
	res := Result{Target: opts.Target}

	logw := opts.Stderr
	if logw == nil {
		logw = io.Discard
	}
	logf := func(format string, args ...any) {
		fmt.Fprintf(logw, "[release-pipeline] "+format+"\n", args...)
	}

	// Argument validation (semver target).
	if !semvercheck.IsSemver(opts.Target) {
		return res, fmt.Errorf("%w: target version not semver: %s", ErrPrePublishFailed, opts.Target)
	}

	// Resolve FromTag if not provided.
	fromTag := opts.FromTag
	if fromTag == "" {
		if t, err := resolvePrevTag(opts.RepoRoot); err == nil && t != "" {
			fromTag = t
		} else {
			logf("WARN: no previous tag found; changelog range will start from initial commit")
			if init, err := resolveInitCommit(opts.RepoRoot); err == nil {
				fromTag = init
			}
		}
	}

	// Resolve now seam.
	now := opts.Now
	if now == nil {
		now = time.Now
	}

	// Steps defaults: only overlay missing fields.
	steps := applyDefaultSteps(opts.Steps)

	logf("target: v%s", opts.Target)
	logf("changelog range: %s..HEAD", fromTag)
	logf("dry-run: %v | no-rollback: %v | skip-tests: %v",
		opts.DryRun, opts.NoRollback, opts.SkipTests)

	// Init journal.
	journal, journalPath, err := initJournal(opts, fromTag, now())
	if err != nil {
		return res, fmt.Errorf("%w: journal init: %v", ErrPrePublishFailed, err)
	}
	res.JournalPath = journalPath
	logf("journal: %s", journalPath)

	// Step 0: full-dry-run preflight (opt-in).
	if opts.RequirePreflight {
		logf("step: full-dry-run preflight (--require-preflight)")
		if err := steps.FullDryRunPreflight(opts.RepoRoot, opts.Target); err != nil {
			appendStep(journal, journalPath, "full-dry-run-preflight", "fail", err.Error(), now())
			res.StepsFailed = append(res.StepsFailed, "full-dry-run-preflight")
			return res, fmt.Errorf("%w: full-dry-run preflight: %v", ErrPrePublishFailed, err)
		}
		appendStep(journal, journalPath, "full-dry-run-preflight", "ok", "", now())
		res.StepsCompleted = append(res.StepsCompleted, "full-dry-run-preflight")
	}

	// Step 1: preflight.
	logf("step: preflight")
	if err := steps.Preflight(opts.RepoRoot, opts.Target, opts.DryRun, opts.SkipTests); err != nil {
		appendStep(journal, journalPath, "preflight", "fail", err.Error(), now())
		res.StepsFailed = append(res.StepsFailed, "preflight")
		return res, fmt.Errorf("%w: preflight: %v", ErrPrePublishFailed, err)
	}
	appendStep(journal, journalPath, "preflight", "ok", "", now())
	res.StepsCompleted = append(res.StepsCompleted, "preflight")

	// Step 2: changelog-gen.
	logf("step: changelog-gen")
	if err := steps.ChangelogGen(opts.RepoRoot, fromTag, "HEAD", opts.Target, opts.DryRun); err != nil {
		appendStep(journal, journalPath, "changelog-gen", "fail", err.Error(), now())
		res.StepsFailed = append(res.StepsFailed, "changelog-gen")
		return res, fmt.Errorf("%w: changelog-gen: %v", ErrPrePublishFailed, err)
	}
	appendStep(journal, journalPath, "changelog-gen", "ok", "", now())
	res.StepsCompleted = append(res.StepsCompleted, "changelog-gen")

	// Step 3: version-bump.
	logf("step: version-bump")
	if err := steps.VersionBump(opts.RepoRoot, opts.Target, opts.DryRun); err != nil {
		appendStep(journal, journalPath, "version-bump", "fail", err.Error(), now())
		res.StepsFailed = append(res.StepsFailed, "version-bump")
		return res, fmt.Errorf("%w: version-bump: %v", ErrPrePublishFailed, err)
	}
	appendStep(journal, journalPath, "version-bump", "ok", "", now())
	res.StepsCompleted = append(res.StepsCompleted, "version-bump")

	// Step 3.5: rebuild-binary. Rebuilds go/evolve from the version-bumped
	// source so the marketplace binary is in sync with plugin.json:version
	// after this release. Without this step, operators install the new
	// plugin version but run the previous build. Best-effort in dry-run.
	if opts.DryRun {
		logf("step: rebuild-binary (DRY-RUN — would run `go build -ldflags '-X …version=%s …'` -o go/evolve ./cmd/evolve from <RepoRoot>/go)", opts.Target)
		appendStep(journal, journalPath, "rebuild-binary", "skipped-dry-run", "", now())
	} else {
		logf("step: rebuild-binary")
		if err := steps.RebuildBinary(opts.RepoRoot, opts.Target, false); err != nil {
			appendStep(journal, journalPath, "rebuild-binary", "fail", err.Error(), now())
			res.StepsFailed = append(res.StepsFailed, "rebuild-binary")
			return res, fmt.Errorf("%w: rebuild-binary: %v", ErrPrePublishFailed, err)
		}
		appendStep(journal, journalPath, "rebuild-binary", "ok", "", now())
		res.StepsCompleted = append(res.StepsCompleted, "rebuild-binary")
	}

	// Step 4: release.sh consistency check (skipped in dry-run).
	if opts.DryRun {
		logf("step: release.sh-check (DRY-RUN — skipping; markers not actually bumped)")
		appendStep(journal, journalPath, "release-sh-check", "skipped-dry-run", "", now())
	} else {
		logf("step: release.sh-check")
		if err := steps.ReleaseSh(opts.RepoRoot, opts.Target); err != nil {
			appendStep(journal, journalPath, "release-sh-check", "fail", err.Error(), now())
			res.StepsFailed = append(res.StepsFailed, "release-sh-check")
			return res, fmt.Errorf("%w: release.sh consistency: %v", ErrPrePublishFailed, err)
		}
		appendStep(journal, journalPath, "release-sh-check", "ok", "", now())
		res.StepsCompleted = append(res.StepsCompleted, "release-sh-check")
	}

	// Step 5: ship.sh --class release.
	commitMsg := "release: v" + opts.Target
	if opts.DryRun {
		logf("step: ship.sh (DRY-RUN — would commit & push & gh release create)")
		logf("  commit msg: %s", commitMsg)
		appendStep(journal, journalPath, "ship", "skipped-dry-run", "", now())
		logf("")
		logf("DRY RUN COMPLETE — no mutations were made.")
		return res, nil
	}
	releaseNotes := extractReleaseNotes(opts.RepoRoot, opts.Target)
	logf("step: ship.sh (--class release)")
	newSHA, err := steps.Ship(opts.RepoRoot, commitMsg, releaseNotes)
	if err != nil {
		appendStep(journal, journalPath, "ship", "fail", err.Error(), now())
		res.StepsFailed = append(res.StepsFailed, "ship")
		return res, fmt.Errorf("%w: %v", ErrShipFailed, err)
	}
	appendStep(journal, journalPath, "ship", "ok", "", now())
	res.StepsCompleted = append(res.StepsCompleted, "ship")
	res.NewCommitSHA = newSHA
	setJournalField(journal, journalPath, "commit_sha", newSHA)

	// Step 6: marketplace-poll (with auto-rollback).
	logf("step: marketplace-poll (max_wait=%s)", opts.MaxPollWait)
	if err := steps.MarketplacePoll(opts.RepoRoot, opts.Target, opts.MaxPollWait); err != nil {
		return failPostPublish(&res, journal, journalPath, opts, steps, logf, now,
			"marketplace-poll", "marketplace propagation failed", err)
	}
	appendStep(journal, journalPath, "marketplace-poll", "ok", "", now())
	res.StepsCompleted = append(res.StepsCompleted, "marketplace-poll")

	// Step 7: release-verify — the terminal self-consistency proof (binary
	// committed + pinned + version-stamped, local tag present). A release
	// that cannot prove itself must not stand: same post-publish rollback
	// semantics as a failed propagation.
	logf("step: release-verify")
	if err := steps.ReleaseVerify(opts.RepoRoot, opts.Target, res.NewCommitSHA); err != nil {
		return failPostPublish(&res, journal, journalPath, opts, steps, logf, now,
			"release-verify", "release self-consistency verification failed", err)
	}
	appendStep(journal, journalPath, "release-verify", "ok", "", now())
	res.StepsCompleted = append(res.StepsCompleted, "release-verify")

	setJournalField(journal, journalPath, "completed_at", now().UTC().Format(time.RFC3339))
	logf("DONE: v%s shipped, propagated, and verified", opts.Target)
	logf("journal: %s", journalPath)
	return res, nil
}

// failPostPublish records a failed post-publish step (the commit is already
// pushed), runs the auto-rollback unless --no-rollback, and returns the
// ErrPostPublishFailed result. Shared by marketplace-poll and release-verify
// so the rollback semantics cannot drift between them.
func failPostPublish(res *Result, journal *Journal, journalPath string, opts Options, steps Steps,
	logf func(string, ...any), now func() time.Time, stepName, reasonPrefix string, err error) (Result, error) {
	appendStep(journal, journalPath, stepName, "fail", err.Error(), now())
	res.StepsFailed = append(res.StepsFailed, stepName)
	logf("FAIL: %s: %v", stepName, err)
	wrapped := fmt.Errorf("%w: %s: %v", ErrPostPublishFailed, stepName, err)
	if opts.NoRollback {
		logf("WARN: --no-rollback set; not rolling back. Manual remediation required.")
		return *res, wrapped
	}
	logf("auto-rolling back v%s...", opts.Target)
	setJournalField(journal, journalPath, "completed_at", now().UTC().Format(time.RFC3339))
	reason := fmt.Sprintf("%s: %v", reasonPrefix, err)
	if rbErr := steps.Rollback(opts.RepoRoot, journalPath, reason); rbErr != nil {
		logf("WARN: rollback failed: %v", rbErr)
		res.RollbackErr = rbErr
	} else {
		logf("rollback complete")
	}
	res.RollbackTriggered = true
	return *res, wrapped
}

// --- Journal helpers ------------------------------------------------------

func initJournal(opts Options, fromTag string, startedAt time.Time) (*Journal, string, error) {
	branch, _ := currentBranch(opts.RepoRoot)
	j := &Journal{
		Version:   opts.Target,
		Tag:       "v" + opts.Target,
		Branch:    branch,
		StartedAt: startedAt.UTC().Format(time.RFC3339),
		Steps:     []StepRecord{},
	}
	var path string
	if opts.DryRun {
		path = filepath.Join(os.TempDir(), fmt.Sprintf("release-pipeline-dryrun-%d.json", os.Getpid()))
	} else {
		dir := opts.JournalDir
		if dir == "" {
			dir = filepath.Join(opts.RepoRoot, ".evolve", "release-journal")
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return j, "", err
		}
		ts := startedAt.UTC().Format("20060102T150405Z")
		path = filepath.Join(dir, fmt.Sprintf("%s-%s.json", opts.Target, ts))
	}
	if err := writeJournal(j, path); err != nil {
		return j, path, err
	}
	return j, path, nil
}

func writeJournal(j *Journal, path string) error {
	body, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func appendStep(j *Journal, path, step, status, note string, ts time.Time) {
	j.Steps = append(j.Steps, StepRecord{
		Step:      step,
		Status:    status,
		Note:      note,
		Timestamp: ts.UTC().Format(time.RFC3339),
	})
	_ = writeJournal(j, path)
}

func setJournalField(j *Journal, path, field, value string) {
	switch field {
	case "commit_sha":
		j.CommitSHA = value
	case "release_url":
		j.ReleaseURL = value
	case "completed_at":
		j.CompletedAt = value
	case "tag":
		j.Tag = value
	case "branch":
		j.Branch = value
	}
	_ = writeJournal(j, path)
}

// --- git helpers -----------------------------------------------------------

func resolvePrevTag(repoRoot string) (string, error) {
	out, err := exec.Command("git", "-C", repoRoot, "describe", "--tags", "--abbrev=0").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func resolveInitCommit(repoRoot string) (string, error) {
	out, err := exec.Command("git", "-C", repoRoot, "rev-list", "--max-parents=0", "HEAD").Output()
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 {
		return "", errors.New("no init commit")
	}
	return lines[0], nil
}

func currentBranch(repoRoot string) (string, error) {
	out, err := exec.Command("git", "-C", repoRoot, "symbolic-ref", "--short", "HEAD").Output()
	if err != nil {
		return "unknown", nil
	}
	return strings.TrimSpace(string(out)), nil
}

// --- CHANGELOG extraction --------------------------------------------------

// extractReleaseNotes reads everything between `## [<target>]` and the next
// `## [` heading. Used to populate EVOLVE_SHIP_RELEASE_NOTES for ship.sh.
// Empty result is acceptable (ship still proceeds, just without notes).
func extractReleaseNotes(repoRoot, target string) string {
	body, err := os.ReadFile(filepath.Join(repoRoot, "CHANGELOG.md"))
	if err != nil {
		return ""
	}
	lines := strings.Split(string(body), "\n")
	header := "## [" + target + "]"
	inBlock := false
	var out []string
	for _, line := range lines {
		if strings.HasPrefix(line, "## [") {
			if inBlock {
				break // next entry — stop
			}
			if strings.HasPrefix(line, header) {
				inBlock = true
				continue
			}
		}
		if inBlock {
			out = append(out, line)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// --- Default step implementations ------------------------------------------

// defaultFullDryRunPreflight shells out to legacy/scripts/release/full-dry-run.sh.
func defaultFullDryRunPreflight(repoRoot, target string) error {
	script := filepath.Join(repoRoot, "legacy", "scripts", "release", "full-dry-run.sh")
	if info, err := os.Stat(script); err != nil || (info.Mode()&0o111) == 0 {
		return fmt.Errorf("%s missing or not executable", script)
	}
	cmd := exec.Command("bash", script, "--version", target)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("full-dry-run.sh: %v (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// defaultPreflight calls releasepreflight.Run.
func defaultPreflight(repoRoot, target string, dryRun, skipTests bool) error {
	return runPreflightLib(repoRoot, target, dryRun, skipTests)
}

// defaultChangelogGen calls changeloggen.WriteEntry.
func defaultChangelogGen(repoRoot, fromRef, toRef, target string, dryRun bool) error {
	return runChangelogGenLib(repoRoot, fromRef, toRef, target, dryRun)
}

// defaultVersionBump calls versionbump.Run.
func defaultVersionBump(repoRoot, target string, dryRun bool) error {
	return runVersionBumpLib(repoRoot, target, dryRun)
}

// defaultRebuildBinary runs `go build -o go/evolve ./cmd/evolve` from
// <repoRoot>/go with the Makefile-equivalent ldflags (pkg/version.version =
// target, .commit = short HEAD, .builtAt = now) so the rebuilt binary
// self-reports the release it belongs to — release-verify asserts exactly
// that. The output path go/evolve is the marketplace-tracked binary
// location (matches the find-expression in skills/loop/SKILL.md).
// Returns nil for dryRun (the orchestration layer also skips, but defense
// in depth in case it's called directly). Test seam: callers in tests
// can pass a fake function via Steps.RebuildBinary.
func defaultRebuildBinary(repoRoot, target string, dryRun bool) error {
	if dryRun {
		return nil
	}
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("go toolchain not on PATH: %w", err)
	}
	const versionPkg = "github.com/mickeyyaya/evolve-loop/go/pkg/version"
	commit := "unknown"
	if out, err := exec.Command("git", "-C", repoRoot, "rev-parse", "--short=12", "HEAD").Output(); err == nil {
		commit = strings.TrimSpace(string(out))
	}
	ldflags := fmt.Sprintf("-X %s.version=%s -X %s.commit=%s -X %s.builtAt=%s",
		versionPkg, target, versionPkg, commit, versionPkg, time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	cmd := exec.Command("go", "build", "-ldflags", ldflags, "-o", "evolve", "./cmd/evolve")
	cmd.Dir = filepath.Join(repoRoot, "go")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}
	// Sanity check that the binary actually exists where we asked.
	binPath := filepath.Join(repoRoot, "go", "evolve")
	if _, err := os.Stat(binPath); err != nil {
		return fmt.Errorf("post-build stat %s: %w", binPath, err)
	}
	return nil
}

// defaultReleaseSh calls the releaseconsistency Go library directly
// (v11.8.2+; prior versions shelled out to legacy/scripts/utility/release.sh).
// The cache-refresh half of the bash release.sh is intentionally not
// reproduced here — that's environment-specific and removed entirely in
// v12.0.0; the in-pipeline cache flow is handled by marketplace-poll.
func defaultReleaseSh(repoRoot, target string) error {
	return runReleaseConsistencyLib(repoRoot, target)
}

// defaultShip invokes the native evolve binary's ship subcommand
// (v11.8.3+; prior versions shelled out to legacy/scripts/lifecycle/ship.sh).
// Resolves the binary path via EVOLVE_GO_BIN, then <repoRoot>/go/bin/evolve,
// then <repoRoot>/go/evolve (what rebuild-binary produces), then `evolve` on PATH.
// Returns the new HEAD SHA after the commit lands.
func defaultShip(repoRoot, msg, releaseNotes string) (string, error) {
	binPath := resolveEvolveBin(repoRoot)
	if binPath == "" {
		return "", fmt.Errorf("evolve binary not found (set EVOLVE_GO_BIN, or place at %s/go/bin/evolve or %s/go/evolve); v12.0.0+ requires the native binary",
			repoRoot, repoRoot)
	}
	cmd := exec.Command(binPath, "ship", "--class", "release", msg)
	cmd.Env = append(os.Environ(),
		"EVOLVE_SHIP_RELEASE_NOTES="+releaseNotes,
		"EVOLVE_SHIP_AUTO_CONFIRM=1", // releasepipeline is non-interactive
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("evolve ship: %v (output: %s)", err, strings.TrimSpace(string(out)))
	}
	headOut, err := exec.Command("git", "-C", repoRoot, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(headOut)), nil
}

// resolveEvolveBin returns a path to the native evolve binary, or "" if none is
// callable. Resolution order:
//
//  1. EVOLVE_GO_BIN (if set + executable)
//  2. <repoRoot>/go/bin/evolve (the gitignored local build — make build / runtime hooks)
//  3. <repoRoot>/go/evolve (the tracked binary defaultRebuildBinary produces)
//  4. `evolve` on PATH
//
// Step 3 is essential: the release's rebuild-binary step builds to
// <repoRoot>/go/evolve (`go build -o evolve`, cwd go/), so the very next step
// (ship) must resolve it there. Without it, a release on a host with no prior
// `make build` failed "binary not found" one step after building the binary.
func resolveEvolveBin(repoRoot string) string {
	if p := os.Getenv("EVOLVE_GO_BIN"); p != "" && isExecutableFile(p) {
		return p
	}
	if c := filepath.Join(repoRoot, "go", "bin", "evolve"); isExecutableFile(c) {
		return c
	}
	if c := filepath.Join(repoRoot, "go", "evolve"); isExecutableFile(c) {
		return c
	}
	if found, err := exec.LookPath("evolve"); err == nil {
		return found
	}
	return ""
}

// isExecutableFile reports whether path exists and has any execute bit set.
func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode()&0o111 != 0
}

// defaultMarketplacePoll calls marketplacepoll.Run with the default
// EVOLVE_MARKETPLACE_DIR or ~/.claude/plugins/marketplaces/evolve-loop.
func defaultMarketplacePoll(repoRoot, target string, maxWait time.Duration) error {
	return runMarketplacePollLib(repoRoot, target, maxWait)
}

// defaultRollback calls rollback.Run.
func defaultRollback(repoRoot, journalPath, reason string) error {
	return runRollbackLib(repoRoot, journalPath, reason)
}

// defaultReleaseVerify is the terminal release self-consistency proof
// (inbox release-rebuild-binary-not-committed acceptance):
//
//  1. sha256(disk go/evolve) == sha256(blob <commitSHA>:go/evolve) — the
//     binary the release built is the binary the release committed. This is
//     the structural check that failed silently in v18.3.0 and v18.5.0.
//  2. state.json:expected_ship_sha == that sha. Releases never pass through
//     repinPostCycle (cycle-class-only), so a stale pin here is expected on
//     every release — re-pin to the committed blob and log, don't fail.
//  3. `go/evolve --version` contains the target (the ldflags stamp).
//  4. Local tag v<target> exists; `gh release create` tags remote-only, so
//     create the local tag at the release commit when absent.
func defaultReleaseVerify(repoRoot, target, commitSHA string) error {
	// Guard: binAbs is EXECUTED below; a relative repoRoot would make that a
	// CWD-dependent execution (review MEDIUM-1).
	if !filepath.IsAbs(repoRoot) {
		return fmt.Errorf("release-verify: repoRoot must be absolute, got %q", repoRoot)
	}
	binRel := "go/evolve"
	binAbs := filepath.Join(repoRoot, "go", "evolve")

	diskBytes, err := os.ReadFile(binAbs)
	if err != nil {
		return fmt.Errorf("release-verify: tracked binary missing on disk: %w", err)
	}
	diskSHA := fmt.Sprintf("%x", sha256.Sum256(diskBytes))

	blobBytes, err := exec.Command("git", "-C", repoRoot, "cat-file", "blob", commitSHA+":"+binRel).Output()
	if err != nil {
		return fmt.Errorf("release-verify: %s not committed in release %s (the v18.5.0 defect): %w", binRel, commitSHA, err)
	}
	blobSHA := fmt.Sprintf("%x", sha256.Sum256(blobBytes))
	if diskSHA != blobSHA {
		return fmt.Errorf("release-verify: disk %s (%.12s…) != committed blob (%.12s…) — the released binary is not what was committed", binRel, diskSHA, blobSHA)
	}

	// Re-pin expected_ship_sha to the committed blob (best-effort: a missing
	// state.json is not a release defect, e.g. fresh clones).
	statePath := filepath.Join(repoRoot, ".evolve", "state.json")
	if raw, rerr := os.ReadFile(statePath); rerr == nil {
		var st map[string]any
		if jerr := json.Unmarshal(raw, &st); jerr == nil {
			if cur, _ := st["expected_ship_sha"].(string); cur != blobSHA {
				st["expected_ship_sha"] = blobSHA
				st["expected_ship_version"] = target
				if body, merr := json.MarshalIndent(st, "", "  "); merr == nil {
					tmp := statePath + ".tmp"
					if werr := os.WriteFile(tmp, body, 0o644); werr == nil {
						_ = os.Rename(tmp, statePath)
					}
				}
			}
		}
	}

	verOut, err := exec.Command(binAbs, "--version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("release-verify: %s --version failed: %v (output: %s)", binRel, err, strings.TrimSpace(string(verOut)))
	}
	if !strings.Contains(string(verOut), target) {
		return fmt.Errorf("release-verify: %s --version = %q does not report target %s (ldflags stamp missing)", binRel, strings.TrimSpace(string(verOut)), target)
	}

	tag := "v" + target
	tagOut, _ := exec.Command("git", "-C", repoRoot, "tag", "-l", tag).Output()
	if strings.TrimSpace(string(tagOut)) == "" {
		if out, terr := exec.Command("git", "-C", repoRoot, "tag", tag, commitSHA).CombinedOutput(); terr != nil {
			return fmt.Errorf("release-verify: local tag %s absent and creation failed: %v (%s)", tag, terr, strings.TrimSpace(string(out)))
		}
	}
	return nil
}
