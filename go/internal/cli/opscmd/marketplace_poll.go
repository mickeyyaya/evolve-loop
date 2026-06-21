package opscmd

import (
	"errors"
	"fmt"
	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/marketplacepoll"
)

// runMarketplacePoll is `evolve marketplace-poll <target> [--max-wait-s N]
// [--poll-interval-s N] [--marketplace-dir DIR] [--dry-run]`.
//
// Mirrors legacy/scripts/release/marketplace-poll.sh exit codes:
//
//	0  — converged + release.sh refresh OK
//	1  — timeout
//	2  — runtime error (missing dir, bad plugin.json, semver, release.sh fail)
//	10 — invalid arguments
func RunMarketplacePoll(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var (
		target         string
		maxWaitS       = 300
		pollIntervalS  = 15
		marketplaceDir = os.Getenv("EVOLVE_MARKETPLACE_DIR")
		dryRun         bool
	)
	if marketplaceDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			marketplaceDir = home + "/.claude/plugins/marketplaces/evolve-loop"
		}
	}

	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--help" || a == "-h":
			fmt.Fprintln(stdout, "Usage: evolve marketplace-poll <target-version> [flags]")
			fmt.Fprintln(stdout, "  --max-wait-s N           (default 300)")
			fmt.Fprintln(stdout, "  --poll-interval-s N      (default 15)")
			fmt.Fprintln(stdout, "  --marketplace-dir DIR    (default $EVOLVE_MARKETPLACE_DIR or ~/.claude/plugins/marketplaces/evolve-loop)")
			fmt.Fprintln(stdout, "  --dry-run")
			return 0
		case a == "--dry-run":
			dryRun = true
		case a == "--max-wait-s":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "[marketplace-poll] --max-wait-s missing value")
				return 10
			}
			n, err := strconv.Atoi(args[i])
			if err != nil || n <= 0 {
				fmt.Fprintln(stderr, "[marketplace-poll] --max-wait-s must be integer > 0")
				return 10
			}
			maxWaitS = n
		case a == "--poll-interval-s":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "[marketplace-poll] --poll-interval-s missing value")
				return 10
			}
			n, err := strconv.Atoi(args[i])
			if err != nil || n <= 0 {
				fmt.Fprintln(stderr, "[marketplace-poll] --poll-interval-s must be integer > 0")
				return 10
			}
			pollIntervalS = n
		case a == "--marketplace-dir":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "[marketplace-poll] --marketplace-dir missing value")
				return 10
			}
			marketplaceDir = args[i]
		case len(a) >= 2 && a[:2] == "--":
			fmt.Fprintf(stderr, "[marketplace-poll] unknown flag: %s\n", a)
			return 10
		default:
			if target == "" {
				target = a
			} else {
				fmt.Fprintf(stderr, "[marketplace-poll] extra positional arg: %s\n", a)
				return 10
			}
		}
		i++
	}

	if target == "" {
		fmt.Fprintln(stderr, "[marketplace-poll] usage: marketplace-poll <target-version> [flags]")
		return 10
	}

	repoRoot := cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT")
	opts := marketplacepoll.Options{
		Target:         target,
		MarketplaceDir: marketplaceDir,
		MaxWait:        time.Duration(maxWaitS) * time.Second,
		PollInterval:   time.Duration(pollIntervalS) * time.Second,
		DryRun:         dryRun,
		RepoRoot:       repoRoot,
		Stderr:         stderr,
	}
	_, err := marketplacepoll.Run(opts)
	if err == nil {
		return 0
	}
	if errors.Is(err, marketplacepoll.ErrTimeout) {
		return 1
	}
	if errors.Is(err, marketplacepoll.ErrRuntime) {
		fmt.Fprintf(stderr, "[marketplace-poll] FAIL: %v\n", err)
		return 2
	}
	fmt.Fprintf(stderr, "[marketplace-poll] FAIL: %v\n", err)
	return 2
}
