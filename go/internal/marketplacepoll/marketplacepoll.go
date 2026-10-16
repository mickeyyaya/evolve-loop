// Package marketplacepoll polls the local marketplace checkout until its plugin version matches the release target.
// See docs/architecture/packages/internal-marketplacepoll.md.
package marketplacepoll

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/semvercheck"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

var (
	ErrTimeout = errors.New("marketplacepoll: timeout waiting for marketplace convergence")
	ErrRuntime = errors.New("marketplacepoll: runtime error")
)

type Options struct {
	Target         string
	MarketplaceDir string
	MaxWait        time.Duration
	PollInterval   time.Duration
	DryRun         bool
	RepoRoot       string
	Stderr         io.Writer

	Now       func() time.Time
	Sleep     func(time.Duration)
	Pull      func(dir string) error
	ReleaseSh func(repoRoot, target string) error
}

type Result struct {
	Converged      bool
	Attempts       int
	Elapsed        time.Duration
	FinalVersion   string
	ReleaseShRunOK bool
}

func pluginJSONPath(marketDir string) string {
	return filepath.Join(marketDir, ".claude-plugin", "plugin.json")
}

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

func DefaultPull(dir string) error {
	gitDir := filepath.Join(dir, ".git")
	if info, err := os.Stat(gitDir); err != nil || !info.IsDir() {
		return nil
	}
	_ = sysexec.Command(context.Background(), "git", "-C", dir, "fetch", "origin", "main", "--quiet").Run()
	_ = sysexec.Command(context.Background(), "git", "-C", dir, "reset", "--hard", "origin/main", "--quiet").Run()
	return nil
}

func DefaultReleaseSh(repoRoot, target string) error {
	return nil
}

func Run(opts Options) (Result, error) {
	res := Result{}
	if err := validateMarketplacePollOptions(opts); err != nil {
		return res, err
	}

	now, sleep, pull, releaseSh, logf := resolveMarketplacePollSeams(opts)

	if opts.DryRun {
		logDryRunPoll(opts, logf)
		return res, nil
	}

	if err := validateMarketplaceDir(opts.MarketplaceDir); err != nil {
		return res, err
	}

	res, err := pollForConvergence(opts, now, sleep, pull, logf)
	if err != nil {
		return res, err
	}

	if err := refreshInstalledPlugins(&res, opts, releaseSh, logf); err != nil {
		return res, err
	}
	return res, nil
}

func validateMarketplacePollOptions(opts Options) error {
	if !semvercheck.IsSemver(opts.Target) {
		return fmt.Errorf("%w: target version not semver: %s", ErrRuntime, opts.Target)
	}
	if opts.MaxWait <= 0 {
		return fmt.Errorf("%w: MaxWait must be > 0", ErrRuntime)
	}
	if opts.PollInterval <= 0 {
		return fmt.Errorf("%w: PollInterval must be > 0", ErrRuntime)
	}
	return nil
}

func resolveMarketplacePollSeams(opts Options) (func() time.Time, func(time.Duration), func(string) error, func(string, string) error, func(string, ...any)) {
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
	return now, sleep, pull, releaseSh, logf
}

func logDryRunPoll(opts Options, logf func(string, ...any)) {
	logf("DRY-RUN: would poll %s for version=%s", opts.MarketplaceDir, opts.Target)
	logf("DRY-RUN: max_wait=%s poll_interval=%s", opts.MaxWait, opts.PollInterval)
	logf("DRY-RUN: on success would invoke release.sh %s", opts.Target)
}

func validateMarketplaceDir(dir string) error {
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("%w: marketplace dir not found: %s", ErrRuntime, dir)
	}
	if _, err := os.Stat(pluginJSONPath(dir)); err != nil {
		return fmt.Errorf("%w: marketplace dir has no .claude-plugin/plugin.json: %s",
			ErrRuntime, dir)
	}
	return nil
}

func pollForConvergence(opts Options, now func() time.Time, sleep func(time.Duration), pull func(string) error, logf func(string, ...any)) (Result, error) {
	res := Result{}
	start := now()
	deadline := start.Add(opts.MaxWait)
	logf("polling %s for version=%s (max_wait=%s, interval=%s)",
		opts.MarketplaceDir, opts.Target, opts.MaxWait, opts.PollInterval)

	for {
		res.Attempts++
		_ = pull(opts.MarketplaceDir)

		current, _ := ReadMarketplaceVersion(opts.MarketplaceDir)
		res.FinalVersion = current
		if current == opts.Target {
			res.Converged = true
			res.Elapsed = now().Sub(start)
			logf("OK: marketplace converged to v%s after %s (attempt %d)",
				opts.Target, res.Elapsed.Round(time.Second), res.Attempts)
			return res, nil
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
}

func refreshInstalledPlugins(res *Result, opts Options, releaseSh func(string, string) error, logf func(string, ...any)) error {
	logf("running release.sh %s to refresh installed_plugins.json...", opts.Target)
	if err := releaseSh(opts.RepoRoot, opts.Target); err != nil {
		logf("WARN: release.sh exited non-zero — manually verify installed_plugins.json")
		logf("  bash legacy/scripts/utility/release.sh %s", opts.Target)
		return fmt.Errorf("%w: %v", ErrRuntime, err)
	}
	res.ReleaseShRunOK = true
	logf("DONE: marketplace + installed_plugins.json refreshed to v%s", opts.Target)
	return nil
}
