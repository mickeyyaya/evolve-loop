// Package cmdutil holds tiny helpers shared by the `evolve` CLI's
// per-subcommand handlers. Deliberately minimal — this is not a CLI
// framework. The existing manual flag parsing in each cmd_*.go file is
// already idiomatic Go; cmdutil only extracts the 1-2 patterns that
// were verbatim-duplicated across 10+ files.
package cmdutil

// HasHelp reports whether args contains the conventional help flags
// ("-h" or "--help"). Use as an early-exit gate at the top of a
// subcommand's Run function:
//
//	if cmdutil.HasHelp(args) {
//	    fmt.Fprintln(stdout, "Usage: ...")
//	    return 0
//	}
//
// Replaces the verbatim 6-line `for _, a := range args { if a == "-h"
// || a == "--help" { ... } }` block that lived in 13 cmd_*.go files.
func HasHelp(args []string) bool {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			return true
		}
	}
	return false
}
