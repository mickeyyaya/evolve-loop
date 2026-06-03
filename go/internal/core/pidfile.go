package core

import "strings"

// BridgePIDFile derives the agent-PID file path from a phase stdout-log path
// (<ws>/<phase>-stdout.log → <ws>/<phase>.bridge-pid). It is the SINGLE source
// of this convention, shared by the bridge (which WRITES the agent PID at
// launch) and the auto-spawn observer's CPU liveness probe (which READS it), so
// the two cannot drift — a rename here moves both sides at once. Returns "" when
// stdoutLog is off-convention, in which case the bridge skips the write and the
// probe no-ops (best-effort).
func BridgePIDFile(stdoutLog string) string {
	const suffix = "-stdout.log"
	if !strings.HasSuffix(stdoutLog, suffix) {
		return ""
	}
	return strings.TrimSuffix(stdoutLog, suffix) + ".bridge-pid"
}
