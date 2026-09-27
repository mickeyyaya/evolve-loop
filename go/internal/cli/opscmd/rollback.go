package opscmd

import (
	"errors"
	"fmt"
	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/rollback"
)

func parseRollbackArgs(args []string, stdout, stderr io.Writer) (journalPath, reason string, dryRun bool, code int, done bool) {
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--help" || a == "-h":
			fmt.Fprintln(stdout, "Usage: evolve rollback <journal.json> [--reason \"...\"] [--dry-run]")
			fmt.Fprintln(stdout, "Auto-revert a failed release: gh release delete + remote tag delete + git revert + ship.")
			return "", "", false, 0, true
		case a == "--dry-run":
			dryRun = true
		case a == "--reason":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "[rollback] --reason missing value")
				return "", "", false, 10, true
			}
			reason = args[i]
		case len(a) >= 2 && a[:2] == "--":
			fmt.Fprintf(stderr, "[rollback] unknown flag: %s\n", a)
			return "", "", false, 10, true
		default:
			if journalPath == "" {
				journalPath = a
			} else {
				fmt.Fprintf(stderr, "[rollback] extra positional arg: %s\n", a)
				return "", "", false, 10, true
			}
		}
		i++
	}

	if journalPath == "" {
		fmt.Fprintln(stderr, "[rollback] usage: rollback <journal.json> [--reason \"...\"] [--dry-run]")
		return "", "", false, 10, true
	}
	return journalPath, reason, dryRun, 0, false
}

func rollbackExitCode(err error, stderr io.Writer) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, rollback.ErrJournalNotFound) || errors.Is(err, rollback.ErrJournalMalformed) {
		fmt.Fprintf(stderr, "[rollback] FAIL: %v\n", err)
		return 2
	}
	if errors.Is(err, rollback.ErrPartial) {
		// Already logged via Stderr; partial = exit 1.
		return 1
	}
	fmt.Fprintf(stderr, "[rollback] FAIL: %v\n", err)
	return 1
}

// runRollback is `evolve rollback <journal.json> [--reason "..."] [--dry-run]`.
// Mirrors legacy/scripts/release/rollback.sh exit codes:
//
//	0  — rollback complete (all 3 steps OK or skipped)
//	1  — rollback partial (some step failed)
//	2  — journal not found / malformed
//	10 — invalid arguments
func RunRollback(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	journalPath, reason, dryRun, code, done := parseRollbackArgs(args, stdout, stderr)
	if done {
		return code
	}

	repoRoot := cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT")
	opts := rollback.Options{
		JournalPath: journalPath,
		Reason:      reason,
		DryRun:      dryRun,
		RepoRoot:    repoRoot,
		Stderr:      stderr,
	}
	_, err := rollback.Run(opts)
	return rollbackExitCode(err, stderr)
}
