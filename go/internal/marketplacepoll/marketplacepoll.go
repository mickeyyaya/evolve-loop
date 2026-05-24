// Package marketplacepoll ports legacy/scripts/release/marketplace-poll.sh.
//
// Post-publish marketplace propagation verifier (v8.13.2). Polls the local
// marketplace checkout — the path Claude Code reads at session startup —
// until the plugin.json there matches the target version, OR until the
// deadline. On success, re-invokes release.sh to refresh installed_plugins.json
// (the **cache-refresh ordering bug fix**: poll first, then refresh, so the
// refresh check runs only when version-match is already true).
//
// Exit codes (mirrored by the cmd-layer wrapper):
//
//	0 — marketplace converged + release.sh refresh succeeded
//	1 — timeout: polled until MaxWait without matching version
//	2 — runtime error (missing dir, malformed plugin.json, release.sh failed)
//
// Testing seams: callers may inject Now/Sleep/Pull/ReleaseSh to bypass real
// time, real git operations, and real shell invocations. Defaults preserve
// the bash script's external behavior.
package marketplacepoll

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Sentinel errors. The cmd layer maps these to bash exit codes.
var (
	// ErrTimeout is returned when MaxWait elapses without convergence.
	// Maps to exit code 1.
	ErrTimeout = errors.New("marketplacepoll: timeout waiting for marketplace convergence")
	// ErrRuntime wraps runtime failures (missing dir, missing plugin.json,
	// invalid semver target, release.sh failure). Maps to exit code 2.
	ErrRuntime = errors.New("marketplacepoll: runtime error")
)

// Options drives a Run() invocation. Zero values for seam fields cause the
// real implementations to be used; tests override them.
type Options struct {
	Target         string        // required; semver X.Y.Z
	MarketplaceDir string        // required
	MaxWait        time.Duration // required; > 0
	PollInterval   time.Duration // required; > 0
	DryRun         bool
	RepoRoot       string // for default ReleaseSh; used only if ReleaseSh nil
	Stderr         io.Writer

	// Seams; nil = use the real implementations.
	Now       func() time.Time
	Sleep     func(time.Duration)
	Pull      func(dir string) error
	ReleaseSh func(repoRoot, target string) error
}

// Result captures what happened during a Run() call. Always populated even
// on error, so callers can log diagnostics.
type Result struct {
	Converged      bool
	Attempts       int
	Elapsed        time.Duration
	FinalVersion   string
	ReleaseShRunOK bool // true iff release.sh was called and exited 0
}

// IsSemver matches X.Y.Z with numeric components only.
var semverRE = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

func IsSemver(s string) bool { return semverRE.MatchString(s) }

// pluginJSONPath returns the canonical plugin.json location under a
// marketplace directory.
func pluginJSONPath(marketDir string) string {
	return filepath.Join(marketDir, ".claude-plugin", "plugin.json")
}

// ReadMarketplaceVersion reads the plugin.json at dir/.claude-plugin/plugin.json
// and returns the "version" string. Mirrors the bash sed pipeline:
//
//	sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1
//
// Returns ("", error) if the file is missing or unreadable.
// Returns ("", nil) if the file exists but no version field matches (rare —
// in practice the bash script also returns empty here).
func ReadMarketplaceVersion(dir string) (string, error) {
	path := pluginJSONPath(dir)
	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return parseVersionField(string(body)), nil
}

var versionFieldRE = regexp.MustCompile(`"version"[[:space:]]*:[[:space:]]*"([^"]*)"`)

func parseVersionField(s string) string {
	m := versionFieldRE.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// DefaultPull is the production Pull implementation: git fetch origin main
// + git reset --hard origin/main, silently no-op'd if dir is not a git
// checkout (matches bash).
func DefaultPull(dir string) error {
	gitDir := filepath.Join(dir, ".git")
	if info, err := os.Stat(gitDir); err != nil || !info.IsDir() {
		return nil // not a git checkout — silent no-op
	}
	// Both errors are intentionally swallowed: the bash script does the
	// same. Convergence is detected by the version check, not by git
	// exit codes.
	_ = exec.Command("git", "-C", dir, "fetch", "origin", "main", "--quiet").Run()
	_ = exec.Command("git", "-C", dir, "reset", "--hard", "origin/main", "--quiet").Run()
	return nil
}

// DefaultReleaseSh runs the bash release.sh if still present (cache-refresh
// side-effects: marketplace pull + installed_plugins.json registry update).
// In v12.0.0 the bash script is removed; this becomes a graceful no-op that
// logs the skip but never errors. The pure-consistency-check half is
// already covered by go/internal/releaseconsistency.
func DefaultReleaseSh(repoRoot, target string) error {
	script := filepath.Join(repoRoot, "legacy", "scripts", "utility", "release.sh")
	if _, err := os.Stat(script); err != nil {
		// v12.0.0+: legacy/scripts/utility/release.sh removed. Skip cache
		// refresh; consistency is already covered by releaseconsistency.
		return nil
	}
	cmd := exec.Command("bash", script, target)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("release.sh exited non-zero: %w (output: %s)",
			err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Run executes the poll loop. Returns Result + error mapped to the bash
// exit-code scheme:
//
//	nil               → exit 0
//	ErrTimeout        → exit 1
//	ErrRuntime (wrap) → exit 2
//
// Caller must ensure Target/MarketplaceDir/MaxWait/PollInterval are set;
// validation of those happens here and surfaces as ErrRuntime.
func Run(opts Options) (Result, error) {
	res := Result{}

	// Validate semver target.
	if !IsSemver(opts.Target) {
		return res, fmt.Errorf("%w: target version not semver: %s", ErrRuntime, opts.Target)
	}
	if opts.MaxWait <= 0 {
		return res, fmt.Errorf("%w: MaxWait must be > 0", ErrRuntime)
	}
	if opts.PollInterval <= 0 {
		return res, fmt.Errorf("%w: PollInterval must be > 0", ErrRuntime)
	}

	// Seam defaults.
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	sleep := opts.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}
	pull := opts.Pull
	if pull == nil {
		pull = DefaultPull
	}
	releaseSh := opts.ReleaseSh
	if releaseSh == nil {
		releaseSh = DefaultReleaseSh
	}
	logw := opts.Stderr
	if logw == nil {
		logw = io.Discard
	}

	logf := func(format string, args ...any) {
		fmt.Fprintf(logw, "[marketplace-poll] "+format+"\n", args...)
	}

	// Dry-run: announce intent and return.
	if opts.DryRun {
		logf("DRY-RUN: would poll %s for version=%s", opts.MarketplaceDir, opts.Target)
		logf("DRY-RUN: max_wait=%s poll_interval=%s", opts.MaxWait, opts.PollInterval)
		logf("DRY-RUN: on success would invoke release.sh %s", opts.Target)
		return res, nil
	}

	// Validate marketplace dir exists.
	if info, err := os.Stat(opts.MarketplaceDir); err != nil || !info.IsDir() {
		return res, fmt.Errorf("%w: marketplace dir not found: %s", ErrRuntime, opts.MarketplaceDir)
	}
	if _, err := os.Stat(pluginJSONPath(opts.MarketplaceDir)); err != nil {
		return res, fmt.Errorf("%w: marketplace dir has no .claude-plugin/plugin.json: %s",
			ErrRuntime, opts.MarketplaceDir)
	}

	start := now()
	deadline := start.Add(opts.MaxWait)
	logf("polling %s for version=%s (max_wait=%s, interval=%s)",
		opts.MarketplaceDir, opts.Target, opts.MaxWait, opts.PollInterval)

	for {
		res.Attempts++
		_ = pull(opts.MarketplaceDir) // bash swallows pull errors

		current, _ := ReadMarketplaceVersion(opts.MarketplaceDir)
		res.FinalVersion = current
		if current == opts.Target {
			res.Converged = true
			res.Elapsed = now().Sub(start)
			logf("OK: marketplace converged to v%s after %s (attempt %d)",
				opts.Target, res.Elapsed.Round(time.Second), res.Attempts)
			break
		}

		if !now().Before(deadline) {
			res.Elapsed = now().Sub(start)
			displayed := current
			if displayed == "" {
				displayed = "<unreadable>"
			}
			logf("TIMEOUT: marketplace still at v%s after %s; expected v%s",
				displayed, opts.MaxWait, opts.Target)
			return res, ErrTimeout
		}
		displayed := current
		if displayed == "" {
			displayed = "<empty>"
		}
		logf("  attempt %d: marketplace at v%s; waiting %s...",
			res.Attempts, displayed, opts.PollInterval)
		sleep(opts.PollInterval)
	}

	// Cache-refresh ordering fix: invoke release.sh now that we know
	// the marketplace is converged.
	logf("running release.sh %s to refresh installed_plugins.json...", opts.Target)
	if err := releaseSh(opts.RepoRoot, opts.Target); err != nil {
		logf("WARN: release.sh exited non-zero — manually verify installed_plugins.json")
		logf("  bash legacy/scripts/utility/release.sh %s", opts.Target)
		return res, fmt.Errorf("%w: %v", ErrRuntime, err)
	}
	res.ReleaseShRunOK = true
	logf("DONE: marketplace + installed_plugins.json refreshed to v%s", opts.Target)
	return res, nil
}
