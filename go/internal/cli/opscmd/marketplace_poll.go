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

func printMarketplacePollUsage(stdout io.Writer) {
	fmt.Fprintln(stdout, "Usage: evolve marketplace-poll <target-version> [flags]")
	fmt.Fprintln(stdout, "  --max-wait-s N           (default 300)")
	fmt.Fprintln(stdout, "  --poll-interval-s N      (default 15)")
	fmt.Fprintln(stdout, "  --marketplace-dir DIR    (default ~/.claude/plugins/marketplaces/evo)")
	fmt.Fprintln(stdout, "  --dry-run")
}

func marketplacePollIntFlag(args []string, i int, name string, stderr io.Writer) (value, nextIndex int, ok bool) {
	if i+1 >= len(args) {
		fmt.Fprintf(stderr, "[marketplace-poll] %s missing value\n", name)
		return 0, i, false
	}
	n, err := strconv.Atoi(args[i+1])
	if err != nil || n <= 0 {
		fmt.Fprintf(stderr, "[marketplace-poll] %s must be integer > 0\n", name)
		return 0, i, false
	}
	return n, i + 1, true
}

type marketplacePollFlags struct {
	target         string
	maxWaitS       int
	pollIntervalS  int
	marketplaceDir string
	dryRun         bool
}

func marketplacePollFlagCase(f *marketplacePollFlags, a string, args []string, i *int, stderr io.Writer) (handled, ok bool) {
	switch {
	case a == "--dry-run":
		f.dryRun = true
	case a == "--max-wait-s":
		n, ni, ok := marketplacePollIntFlag(args, *i, "--max-wait-s", stderr)
		if !ok {
			return true, false
		}
		f.maxWaitS, *i = n, ni
	case a == "--poll-interval-s":
		n, ni, ok := marketplacePollIntFlag(args, *i, "--poll-interval-s", stderr)
		if !ok {
			return true, false
		}
		f.pollIntervalS, *i = n, ni
	case a == "--marketplace-dir":
		*i++
		if *i >= len(args) {
			fmt.Fprintln(stderr, "[marketplace-poll] --marketplace-dir missing value")
			return true, false
		}
		f.marketplaceDir = args[*i]
	case len(a) >= 2 && a[:2] == "--":
		fmt.Fprintf(stderr, "[marketplace-poll] unknown flag: %s\n", a)
		return true, false
	default:
		return false, true
	}
	return true, true
}

func parseMarketplacePollArgs(args []string, stdout, stderr io.Writer) (f marketplacePollFlags, code int, done bool) {
	f.maxWaitS = 300
	f.pollIntervalS = 15
	if home, err := os.UserHomeDir(); err == nil {
		f.marketplaceDir = home + "/.claude/plugins/marketplaces/evo"
	}

	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--help" || a == "-h" {
			printMarketplacePollUsage(stdout)
			return f, 0, true
		}
		if handled, ok := marketplacePollFlagCase(&f, a, args, &i, stderr); handled {
			if !ok {
				return f, 10, true
			}
			continue
		}
		if f.target == "" {
			f.target = a
		} else {
			fmt.Fprintf(stderr, "[marketplace-poll] extra positional arg: %s\n", a)
			return f, 10, true
		}
	}

	if f.target == "" {
		fmt.Fprintln(stderr, "[marketplace-poll] usage: marketplace-poll <target-version> [flags]")
		return f, 10, true
	}
	return f, 0, false
}

func marketplacePollExitCode(err error, stderr io.Writer) int {
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
	f, code, done := parseMarketplacePollArgs(args, stdout, stderr)
	if done {
		return code
	}

	repoRoot := cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT")
	opts := marketplacepoll.Options{
		Target:         f.target,
		MarketplaceDir: f.marketplaceDir,
		MaxWait:        time.Duration(f.maxWaitS) * time.Second,
		PollInterval:   time.Duration(f.pollIntervalS) * time.Second,
		DryRun:         f.dryRun,
		RepoRoot:       repoRoot,
		Stderr:         stderr,
	}
	_, err := marketplacepoll.Run(opts)
	return marketplacePollExitCode(err, stderr)
}
