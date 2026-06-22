package core

import (
	"strconv"
	"strings"
)

// AdvisorDepthExceeded reports whether the dispatch env marks this as a NESTED
// advisor invocation (EVOLVE_ADVISOR_DEPTH≥1). A malformed/non-numeric value is
// treated as 0 — the stamp is defense-in-depth, never a reason to break a valid
// advisor call. EVOLVE_ADVISOR_DEPTH is registered in flagregistry (WS1-S2).
// Moved here from phase_advisor.go so that file no longer reads the env key.
func AdvisorDepthExceeded(env map[string]string) bool {
	n, err := strconv.Atoi(strings.TrimSpace(env["EVOLVE_ADVISOR_DEPTH"]))
	return err == nil && n >= 1
}
