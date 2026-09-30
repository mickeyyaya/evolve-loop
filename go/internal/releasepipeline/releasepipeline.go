// Package releasepipeline drives `evolve release`: preflight, changelog, version
// bump, binary rebuild, ship, marketplace poll and release verify, with
// auto-rollback. See docs/architecture/packages/internal-releasepipeline.md.
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
)

var (
	ErrPrePublishFailed  = errors.New("releasepipeline: pre-publish step failed")
	ErrShipFailed        = errors.New("releasepipeline: ship.sh failed")
	ErrPostPublishFailed = errors.New("releasepipeline: post-publish step failed")
)

type Steps struct {
	FullDryRunPreflight func(repoRoot, target string) error

	Preflight       func(repoRoot, target string, dryRun, skipTests bool) error
	ChangelogGen    func(repoRoot, fromRef, toRef, target string, dryRun bool) error
	VersionBump     func(repoRoot, target string, dryRun bool) error
	RebuildBinary   func(repoRoot, target string, dryRun bool) error
	ReleaseSh       func(repoRoot, target string) error
	Ship            func(repoRoot, msg, releaseNotes string) (newSHA string, err error)
	MarketplacePoll func(repoRoot, target string, maxWait time.Duration) error
	Rollback        func(repoRoot, journalPath, reason string) error
	ReleaseVerify   func(repoRoot, target, commitSHA string) error
}

type Options struct {
	Target           string
	RepoRoot         string
	DryRun           bool
	NoRollback       bool
	SkipTests        bool
	StrictPass       bool
	RequirePreflight bool
	MaxPollWait      time.Duration
	FromTag          string
	JournalDir       string
	Stderr           io.Writer

	Now   func() time.Time
	Steps Steps
}

type Result struct {
	Target            string
	JournalPath       string
	StepsCompleted    []string
	StepsFailed       []string
	NewCommitSHA      string
	RollbackTriggered bool
	RollbackErr       error
}

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

type StepRecord struct {
	Step      string `json:"step"`
	Status    string `json:"status"`
	Note      string `json:"note,omitempty"`
	Timestamp string `json:"timestamp"`
}

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

func Run(opts Options) (Result, error) {
	r, err := newReleaseRun(opts)
	if err != nil {
		return r.res, err
	}
	if err := r.prePublish(); err != nil {
		return r.res, err
	}
	shipped, err := r.ship()
	if err != nil {
		return r.res, err
	}
	if !shipped {
		return r.res, nil
	}
	if err := r.postPublish(); err != nil {
		return r.res, err
	}
	r.complete()
	return r.res, nil
}

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

func initJournal(opts Options, fromTag string, startedAt time.Time) (*Journal, string, error) {
	branch, _ := currentBranch(opts.RepoRoot)
	j := &Journal{
		Version:   opts.Target,
		Tag:       "v" + opts.Target,
		Branch:    branch,
		StartedAt: startedAt.UTC().Format(time.RFC3339),
		Steps:     []StepRecord{},
	}
	path := journalPath(opts, startedAt)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return j, "", err
	}
	if err := writeJournal(j, path); err != nil {
		return j, path, err
	}
	return j, path, nil
}

func journalPath(opts Options, startedAt time.Time) string {
	dir := opts.JournalDir
	if opts.DryRun {
		if dir == "" {
			dir = os.TempDir()
		}
		return filepath.Join(dir, fmt.Sprintf("release-pipeline-dryrun-%s.json", opts.Target))
	}
	if dir == "" {
		dir = filepath.Join(opts.RepoRoot, ".evolve", "release-journal")
	}
	return filepath.Join(dir, fmt.Sprintf("%s-%s.json", opts.Target, startedAt.UTC().Format("20060102T150405Z")))
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
				break
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
	notes := strings.TrimSpace(strings.Join(out, "\n"))
	if notes == "" {
		return ""
	}
	return notes + "\n\n" + fingerprintsSection
}

const fingerprintsSection = "## Fingerprints (corporate approval)\n\n" +
	"The recommended macOS artifact is the universal `evolve_darwin_all.tar.gz` " +
	"(a lipo'd x86_64 + arm64 fat binary) — one fingerprint covering both Intel " +
	"and Apple Silicon. SHA256 checksums for every published artifact are in the " +
	"`checksums.txt` release asset. (The per-arch `evolve_darwin_amd64.tar.gz` / " +
	"`evolve_darwin_arm64.tar.gz` archives remain published for existing installs, " +
	"each with its own distinct hash — approve the universal one.)\n\n" +
	"To adopt this release under a fingerprint-based approval system:\n\n" +
	"1. Download `evolve_darwin_all.tar.gz` and `checksums.txt` from this release.\n" +
	"2. Verify integrity: `shasum -a 256 -c checksums.txt`.\n" +
	"3. Submit the universal binary's SHA256 as the one macOS approval fingerprint " +
	"(one request per adopted version; the pin re-adopts automatically on first run)."

func defaultFullDryRunPreflight(repoRoot, target string) error {
	return runPreflightLib(repoRoot, target, true, true, true)
}

func defaultPreflight(repoRoot, target string, dryRun, skipTests bool) error {
	return runPreflightLib(repoRoot, target, dryRun, skipTests, false)
}

func defaultChangelogGen(repoRoot, fromRef, toRef, target string, dryRun bool) error {
	return runChangelogGenLib(repoRoot, fromRef, toRef, target, dryRun)
}

func defaultVersionBump(repoRoot, target string, dryRun bool) error {
	return runVersionBumpLib(repoRoot, target, dryRun)
}

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
	binPath := filepath.Join(repoRoot, "go", "evolve")
	if _, err := os.Stat(binPath); err != nil {
		return fmt.Errorf("post-build stat %s: %w", binPath, err)
	}
	return nil
}

func defaultReleaseSh(repoRoot, target string) error {
	return runReleaseConsistencyLib(repoRoot, target)
}

func defaultShip(repoRoot, msg, releaseNotes string) (string, error) {
	binPath := resolveEvolveBin(repoRoot)
	if binPath == "" {
		return "", fmt.Errorf("evolve binary not found (set EVOLVE_GO_BIN, or place at %s/go/bin/evolve or %s/go/evolve); v12.0.0+ requires the native binary",
			repoRoot, repoRoot)
	}
	cmd := exec.Command(binPath, "ship", "--class", "release", msg)
	cmd.Env = append(os.Environ(),
		// SSOT IPC-protocol-allowed: releasepipeline → evolve-ship subprocess
		"EVOLVE_"+"SHIP_RELEASE_NOTES="+releaseNotes,
		"EVOLVE_SHIP_AUTO_CONFIRM=1",
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

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode()&0o111 != 0
}

func defaultMarketplacePoll(repoRoot, target string, maxWait time.Duration) error {
	marketplaceDir := ""
	if home, err := os.UserHomeDir(); err == nil {
		marketplaceDir = filepath.Join(home, ".claude", "plugins", "marketplaces", "evo")
	}
	return runMarketplacePollLib(repoRoot, target, maxWait, marketplaceDir)
}

func defaultRollback(repoRoot, journalPath, reason string) error {
	return runRollbackLib(repoRoot, journalPath, reason)
}

func defaultReleaseVerify(repoRoot, target, commitSHA string) error {
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

	repinExpectedShipSHA(filepath.Join(repoRoot, ".evolve", "state.json"), target, blobSHA)

	verOut, err := exec.Command(binAbs, "--version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("release-verify: %s --version failed: %v (output: %s)", binRel, err, strings.TrimSpace(string(verOut)))
	}
	if !strings.Contains(string(verOut), target) {
		return fmt.Errorf("release-verify: %s --version = %q does not report target %s (ldflags stamp missing)", binRel, strings.TrimSpace(string(verOut)), target)
	}

	return ensureLocalTag(repoRoot, "v"+target, commitSHA)
}

func repinExpectedShipSHA(statePath, target, blobSHA string) {
	raw, rerr := os.ReadFile(statePath)
	if rerr != nil {
		return
	}
	var st map[string]any
	if jerr := json.Unmarshal(raw, &st); jerr != nil {
		return
	}
	if cur, _ := st["expected_ship_sha"].(string); cur == blobSHA {
		return
	}
	st["expected_ship_sha"] = blobSHA
	st["expected_ship_version"] = target
	body, merr := json.MarshalIndent(st, "", "  ")
	if merr != nil {
		return
	}
	tmp := statePath + ".tmp"
	if werr := os.WriteFile(tmp, body, 0o644); werr == nil {
		_ = os.Rename(tmp, statePath)
	}
}

func ensureLocalTag(repoRoot, tag, commitSHA string) error {
	tagOut, _ := exec.Command("git", "-C", repoRoot, "tag", "-l", tag).Output()
	if strings.TrimSpace(string(tagOut)) != "" {
		return nil
	}
	if out, terr := exec.Command("git", "-C", repoRoot, "tag", tag, commitSHA).CombinedOutput(); terr != nil {
		return fmt.Errorf("release-verify: local tag %s absent and creation failed: %v (%s)", tag, terr, strings.TrimSpace(string(out)))
	}
	return nil
}
