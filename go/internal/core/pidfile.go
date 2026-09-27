package core

import "strings"

// BridgePIDFile derives the agent-PID file path from a phase stdout-log path
// (<ws>/<phase>-stdout.log → <ws>/<phase>.bridge-pid), or "" when stdoutLog is
// off-convention. It is the single source of this convention, shared by the
// bridge (which writes the PID) and the observer's liveness probe (which
// reads it), so a rename here moves both sides at once.
func BridgePIDFile(stdoutLog string) string {
	const suffix = "-stdout.log"
	if !strings.HasSuffix(stdoutLog, suffix) {
		return ""
	}
	return strings.TrimSuffix(stdoutLog, suffix) + ".bridge-pid"
}
