package releasepipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/changeloggen"
	"github.com/mickeyyaya/evolve-loop/go/internal/marketplacepoll"
	"github.com/mickeyyaya/evolve-loop/go/internal/releaseconsistency"
	"github.com/mickeyyaya/evolve-loop/go/internal/releasepreflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/rollback"
	"github.com/mickeyyaya/evolve-loop/go/internal/versionbump"
)

// runPreflightLib invokes the releasepreflight library directly (no shell-out
// to legacy/scripts/release/preflight.sh).
func runPreflightLib(repoRoot, target string, dryRun, skipTests bool) error {
	_, err := releasepreflight.Run(releasepreflight.Options{
		Target:     target,
		RepoRoot:   repoRoot,
		DryRun:     dryRun,
		SkipTests:  skipTests,
		StrictPass: os.Getenv("EVOLVE_RELEASE_STRICT_PASS") == "1",
		Stderr:     os.Stderr,
	})
	return err
}

// runChangelogGenLib invokes the changeloggen library directly.
func runChangelogGenLib(repoRoot, fromRef, toRef, target string, dryRun bool) error {
	if !changeloggen.IsSemver(target) {
		return fmt.Errorf("target version not semver: %s", target)
	}
	clPath := changeloggen.ResolveChangelogPath(repoRoot)
	if body, err := os.ReadFile(clPath); err == nil {
		if changeloggen.HasEntry(string(body), target) {
			fmt.Fprintf(os.Stderr, "[changelog-gen] CHANGELOG.md already has [%s] entry — preserving (idempotent skip)\n", target)
			return nil
		}
	}
	if err := changeloggen.VerifyRef(repoRoot, fromRef); err != nil {
		return err
	}
	if err := changeloggen.VerifyRef(repoRoot, toRef); err != nil {
		return err
	}
	commits, err := changeloggen.ReadGitLog(repoRoot, fromRef, toRef)
	if err != nil && err != changeloggen.ErrNoCommits {
		return err
	}
	buckets := changeloggen.ClassifyAll(commits)
	entry := changeloggen.RenderEntry(target, fromRef, toRef, time.Now(), buckets)
	if dryRun {
		fmt.Fprintf(os.Stderr, "[changelog-gen] DRY-RUN: would prepend to %s\n", clPath)
		return nil
	}
	_, _, err = changeloggen.WriteEntry(clPath, target, entry)
	return err
}

// runVersionBumpLib invokes the versionbump library directly.
func runVersionBumpLib(repoRoot, target string, dryRun bool) error {
	paths := versionbump.DefaultPaths(repoRoot)
	_, err := versionbump.Run(paths, target, dryRun, time.Now())
	return err
}

// runMarketplacePollLib invokes the marketplacepoll library with the env-var
// or default marketplace directory.
func runMarketplacePollLib(repoRoot, target string, maxWait time.Duration) error {
	marketplaceDir := os.Getenv("EVOLVE_MARKETPLACE_DIR")
	if marketplaceDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			marketplaceDir = filepath.Join(home, ".claude", "plugins", "marketplaces", "evolve-loop")
		}
	}
	_, err := marketplacepoll.Run(marketplacepoll.Options{
		Target:         target,
		MarketplaceDir: marketplaceDir,
		MaxWait:        maxWait,
		PollInterval:   15 * time.Second,
		RepoRoot:       repoRoot,
		Stderr:         os.Stderr,
	})
	return err
}

// runReleaseConsistencyLib invokes the releaseconsistency library directly.
func runReleaseConsistencyLib(repoRoot, target string) error {
	_, err := releaseconsistency.Run(releaseconsistency.Options{
		ProjectRoot: repoRoot,
		Target:      target,
		Stderr:      os.Stderr,
	})
	return err
}

// runRollbackLib invokes the rollback library directly.
func runRollbackLib(repoRoot, journalPath, reason string) error {
	_, err := rollback.Run(rollback.Options{
		JournalPath: journalPath,
		Reason:      reason,
		RepoRoot:    repoRoot,
		Stderr:      os.Stderr,
	})
	return err
}
