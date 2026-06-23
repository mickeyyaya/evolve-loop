package opscmd

import (
	"errors"
	"fmt"
	"github.com/mickeyyaya/evolveloop/go/cmd/evolve/cmdutil"
	"io"

	"github.com/mickeyyaya/evolveloop/go/internal/rollback"
)

// runRollback is `evolve rollback <journal.json> [--reason "..."] [--dry-run]`.
// Mirrors legacy/scripts/release/rollback.sh exit codes:
//
//	0  — rollback complete (all 3 steps OK or skipped)
//	1  — rollback partial (some step failed)
//	2  — journal not found / malformed
//	10 — invalid arguments
func RunRollback(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var (
		journalPath string
		reason      string
		dryRun      bool
	)

	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--help" || a == "-h":
			fmt.Fprintln(stdout, "Usage: evolve rollback <journal.json> [--reason \"...\"] [--dry-run]")
			fmt.Fprintln(stdout, "Auto-revert a failed release: gh release delete + remote tag delete + git revert + ship.")
			return 0
		case a == "--dry-run":
			dryRun = true
		case a == "--reason":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "[rollback] --reason missing value")
				return 10
			}
			reason = args[i]
		case len(a) >= 2 && a[:2] == "--":
			fmt.Fprintf(stderr, "[rollback] unknown flag: %s\n", a)
			return 10
		default:
			if journalPath == "" {
				journalPath = a
			} else {
				fmt.Fprintf(stderr, "[rollback] extra positional arg: %s\n", a)
				return 10
			}
		}
		i++
	}

	if journalPath == "" {
		fmt.Fprintln(stderr, "[rollback] usage: rollback <journal.json> [--reason \"...\"] [--dry-run]")
		return 10
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
