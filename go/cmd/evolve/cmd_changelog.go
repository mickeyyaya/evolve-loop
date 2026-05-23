package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/changeloggen"
)

// runChangelogGen is `evolve changelog-gen <from-ref> <to-ref> <version> [--dry-run]`.
// Mirrors legacy/scripts/release/changelog-gen.sh.
func runChangelogGen(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var (
		fromRef, toRef, version string
		dryRun                  bool
	)
	for _, a := range args {
		switch {
		case a == "--help" || a == "-h":
			fmt.Fprintln(stdout, "Usage: evolve changelog-gen <from-ref> <to-ref> <target-version> [--dry-run]")
			fmt.Fprintln(stdout, "Conventional-commits parser → Keep-a-Changelog entry prepended to CHANGELOG.md.")
			return 0
		case a == "--dry-run":
			dryRun = true
		case len(a) >= 2 && a[:2] == "--":
			fmt.Fprintf(stderr, "[changelog-gen] unknown flag: %s\n", a)
			return 10
		default:
			switch {
			case fromRef == "":
				fromRef = a
			case toRef == "":
				toRef = a
			case version == "":
				version = a
			default:
				fmt.Fprintf(stderr, "[changelog-gen] extra positional arg: %s\n", a)
				return 10
			}
		}
	}
	if fromRef == "" || toRef == "" || version == "" {
		fmt.Fprintln(stderr, "[changelog-gen] usage: changelog-gen <from-ref> <to-ref> <target-version> [--dry-run]")
		return 10
	}
	if !changeloggen.IsSemver(version) {
		fmt.Fprintf(stderr, "[changelog-gen] FAIL: target version not semver: %s\n", version)
		return 1
	}

	repoRoot := envOrCwd("EVOLVE_PROJECT_ROOT")
	clPath := changeloggen.ResolveChangelogPath(repoRoot)

	// Idempotency pre-check: matches bash behavior of returning OK before
	// running git log if the entry already exists.
	if body, err := os.ReadFile(clPath); err == nil {
		if changeloggen.HasEntry(string(body), version) {
			fmt.Fprintf(stderr, "[changelog-gen] CHANGELOG.md already has [%s] entry — preserving (idempotent skip)\n", version)
			return 0
		}
	}

	if err := changeloggen.VerifyRef(repoRoot, fromRef); err != nil {
		fmt.Fprintf(stderr, "[changelog-gen] FAIL: %v\n", err)
		return 1
	}
	if err := changeloggen.VerifyRef(repoRoot, toRef); err != nil {
		fmt.Fprintf(stderr, "[changelog-gen] FAIL: %v\n", err)
		return 1
	}

	commits, err := changeloggen.ReadGitLog(repoRoot, fromRef, toRef)
	if err != nil && !errors.Is(err, changeloggen.ErrNoCommits) {
		fmt.Fprintf(stderr, "[changelog-gen] FAIL: %v\n", err)
		return 1
	}
	if errors.Is(err, changeloggen.ErrNoCommits) {
		fmt.Fprintf(stderr, "[changelog-gen] WARN: no commits between %s..%s\n", fromRef, toRef)
		fmt.Fprintln(stderr, "[changelog-gen] writing minimal placeholder entry")
	}

	buckets := changeloggen.ClassifyAll(commits)
	entry := changeloggen.RenderEntry(version, fromRef, toRef, time.Now(), buckets)

	if dryRun {
		fmt.Fprintf(stderr, "[changelog-gen] DRY-RUN: would prepend the following block to %s:\n", clPath)
		fmt.Fprintln(stdout, "----- BEGIN GENERATED -----")
		fmt.Fprint(stdout, entry)
		fmt.Fprintln(stdout, "----- END GENERATED -----")
		return 0
	}

	wrote, skipped, err := changeloggen.WriteEntry(clPath, version, entry)
	if err != nil {
		fmt.Fprintf(stderr, "[changelog-gen] FAIL: %v\n", err)
		return 1
	}
	if skipped {
		fmt.Fprintf(stderr, "[changelog-gen] CHANGELOG.md already has [%s] entry — preserving (idempotent skip)\n", version)
		return 0
	}
	if wrote {
		fmt.Fprintf(stderr, "[changelog-gen] OK: prepended [%s] entry to %s\n", version, clPath)
	}
	return 0
}
