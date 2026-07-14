// Package apicover measures public-API coverage: it enumerates every exported
// symbol of a package (go/ast, stdlib only) and applies a two-signal check —
// named by a _test AST AND >0% in `go tool cover -func` — flagging uncovered
// symbols and named-but-0% false-greens. //apicover:ignore reason=... suppresses
// a symbol (reason mandatory). It is warning-only by default; -enforce makes it
// exit non-zero.
//
// It is importable so both the standalone cmd/apicover binary AND the evolve
// binary (the `evolve apicover` subcommand, and the audit's in-process API-gate)
// drive the SAME measurement code from one source — no second first-party
// executable rebuilt at runtime.
package apicover

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

// Main is the apicover CLI entry point, shared by the standalone cmd/apicover
// binary and the `evolve apicover` subcommand so both parse identical flags from
// one source (a per-call FlagSet, so it is safe to invoke in-process). It parses
// args (-cover, -require-doc, -enforce, positional package dirs), writes the
// report to stdout, and returns the process exit code: 0 warning-only (or clean
// under -enforce), 1 under -enforce with unresolved symbols, 2 on a flag or
// measurement error (message to stderr).
func Main(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("apicover", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cover := fs.String("cover", "", "path to `go tool cover -func` output")
	requireDoc := fs.Bool("require-doc", false, "flag exported decls missing a godoc comment")
	enforce := fs.Bool("enforce", false, "exit non-zero when uncovered/false-green symbols exist")
	if err := fs.Parse(args); err != nil {
		// -h/-help: FlagSet already printed usage to stderr. Exit 0 to match the
		// old flag.CommandLine (ExitOnError) behavior — a help request is not an
		// error, and the standalone binary's exit code must stay byte-parity.
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	code, err := Run(Config{
		Dirs:       fs.Args(),
		CoverPath:  *cover,
		RequireDoc: *requireDoc,
		Enforce:    *enforce,
	}, stdout)
	if err != nil {
		fmt.Fprintln(stderr, "apicover:", err)
		return 2
	}
	return code
}
