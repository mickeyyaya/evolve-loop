package runner

// cli_chain.go — dispatch-log helpers for the per-phase CLI fallback chain.
//
// The chain RESOLUTION (primary + fallback + triggers) and the capability
// PROBE now live in internal/llmroute (llmroute.Resolve / llmroute.Probe /
// Plan.TriggersFallback), so the runner makes a single unified call for CLI +
// model. What remains here are the two presentation helpers the runner uses
// when logging that chain's execution.

// sameCandidates reports whether two ordered candidate lists are
// element-for-element equal. Used to suppress the "probe reordered" log line
// when the capability probe left the chain unchanged.
func sameCandidates(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// joinAttempts formats the per-attempt dispatch log — one "cli=exit" token per
// attempt, separated by " -> " arrows so the chain reads left-to-right in the
// order candidates were tried. Used only when fallback actually fired
// (>1 attempt) so single-CLI phases stay quiet.
func joinAttempts(attempts []string) string {
	if len(attempts) == 0 {
		return ""
	}
	out := attempts[0]
	for _, a := range attempts[1:] {
		out += " -> " + a
	}
	return out
}
