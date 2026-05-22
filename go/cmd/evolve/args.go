package main

import "strings"

// reorderArgs moves all flag tokens (starting with "-") ahead of
// positional tokens so Go's stdlib flag.Parse — which stops at the
// first positional — accepts flag-after-positional invocations like
// the bash predicates do (e.g. `probe foo --json`).
//
// Limitations: assumes flags are bool or use `--flag=value` form. The
// only callers in Phase 1 (cmd_doctor) only define bool flags, so this
// is safe. Future callers passing `--flag value` (space-separated)
// must declare them with `=` or be reordered manually.
func reorderArgs(args []string) []string {
	flags := make([]string, 0, len(args))
	pos := make([]string, 0, len(args))
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
		} else {
			pos = append(pos, a)
		}
	}
	return append(flags, pos...)
}
