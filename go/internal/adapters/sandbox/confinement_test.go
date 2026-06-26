package sandbox

import "testing"

// SSOT tests for the unified sandbox-confinement decision (DetectNested +
// ShouldWrap). Before this, "is this a nested LLM CLI sandbox session?" was detected by
// two different heuristics (preflight read CLAUDECODE; the bridge read
// CLAUDE_CODE_ENTRYPOINT/SESSION_ID), and "should the inner OS sandbox wrap?"
// was decided in two places — the bridge (auto-mode-only nested skip, which
// hung EVOLVE_SANDBOX=on under nested macOS) and preflight (InnerSandbox).
// These functions are the single source both consumers now call.

func TestDetectNested(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"empty", map[string]string{}, false},
		{"claudecode only", map[string]string{"CLAUDECODE": "1"}, true},
		{"entrypoint only (bridge's old signal)", map[string]string{"CLAUDE_CODE_ENTRYPOINT": "cli"}, true},
		{"session id only (bridge's old signal)", map[string]string{"CLAUDE_CODE_SESSION_ID": "abc"}, true},
		{"all signals (our real env)", map[string]string{
			"CLAUDECODE": "1", "CLAUDE_CODE_ENTRYPOINT": "cli", "CLAUDE_CODE_SESSION_ID": "abc",
		}, true},
		{"codex managed sandbox bootstrap path", map[string]string{
			"PATH": "/usr/bin:/var/run/com.apple.security.cryptexd/codex.system/bootstrap/usr/bin:/bin",
		}, true},
		{"codex arg0 path", map[string]string{
			"PATH": "/usr/bin:/Users/test/.codex/tmp/arg0/codex-arg0abc:/bin",
		}, true},
		// CLAUDECODE_TYPE=host marks the top-level host process, which is NOT
		// nested-under-another-sandbox — it overrides the other signals
		// (preserves preflight's original semantics).
		{"host type overrides", map[string]string{
			"CLAUDECODE": "1", "CLAUDE_CODE_ENTRYPOINT": "cli", "CLAUDECODE_TYPE": "host",
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			get := func(k string) string { return tt.env[k] }
			if got := DetectNested(get); got != tt.want {
				t.Errorf("DetectNested(%v) = %v, want %v", tt.env, got, tt.want)
			}
		})
	}
}

func TestShouldWrap(t *testing.T) {
	avail := func(os string) ProbeResult { return ProbeResult{OS: os, Available: true, BinaryPath: "/usr/bin/" + os} }
	unavail := func(os string) ProbeResult { return ProbeResult{OS: os, Available: false, Reason: "not on PATH"} }

	tests := []struct {
		name     string
		nested   bool
		probe    ProbeResult
		wantWrap bool
	}{
		// The core regression: nested managed LLM CLIs must NOT wrap regardless of binary
		// availability — on darwin the inner sandbox-exec hangs the REPL boot,
		// and on every OS the outer Claude session already confines. This is
		// the cell the bridge's auto-only skip missed for EVOLVE_SANDBOX=on.
		{"darwin nested + available → skip", true, avail("darwin"), false},
		{"linux nested + available → skip", true, avail("linux"), false},
		// Not nested + binary available → wrap (the normal confined path).
		{"darwin standalone + available → wrap", false, avail("darwin"), true},
		{"linux standalone + available → wrap", false, avail("linux"), true},
		// Not nested but no binary → can't wrap (degrade).
		{"darwin standalone + unavailable → skip", false, unavail("darwin"), false},
		{"linux standalone + unavailable → skip", false, unavail("linux"), false},
		// Unsupported OS → never wrap.
		{"unsupported os → skip", false, ProbeResult{OS: "windows", Available: true}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotWrap, reason := ShouldWrap(tt.nested, tt.probe)
			if gotWrap != tt.wantWrap {
				t.Errorf("ShouldWrap(nested=%v, %s avail=%v) wrap=%v, want %v (reason: %q)",
					tt.nested, tt.probe.OS, tt.probe.Available, gotWrap, tt.wantWrap, reason)
			}
			if reason == "" {
				t.Errorf("ShouldWrap must always return a non-empty reason (got wrap=%v)", gotWrap)
			}
		})
	}
}
