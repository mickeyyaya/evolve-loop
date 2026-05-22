// Package cycle68 ports the cycle-68 ACS predicates (2 bash files).
package cycle68

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC68_001_IntentMaxTurnsRaised ports cycle-68/001.
// .evolve/profiles/intent.json:max_turns is in [8, 32].
func TestC68_001_IntentMaxTurnsRaised(t *testing.T) {
	root := acsassert.RepoRoot(t)
	profile := filepath.Join(root, ".evolve", "profiles", "intent.json")
	if !acsassert.FileExists(t, profile) {
		t.Skip("intent.json missing — skip cycle-68-001")
	}
	// JSONFieldEquals only checks one value; assert max_turns is in range
	// via regex on the raw file. Accept any integer between 8 and 32.
	if !acsassert.FileMatchesRegex(t, profile, `"max_turns"\s*:\s*(8|9|1[0-9]|2[0-9]|3[0-2])`) {
		t.Errorf("%s: max_turns not in [8,32]", profile)
	}
}

// TestC68_002_OverrunLogFormat ports cycle-68/002.
func TestC68_002_OverrunLogFormat(t *testing.T) {
	root := acsassert.RepoRoot(t)
	script := filepath.Join(root, "scripts", "dispatch", "subagent-run.sh")
	if !acsassert.FileExists(t, script) {
		t.Skip("subagent-run.sh missing — skip cycle-68-002")
	}
	// Required: new format string.
	if !acsassert.FileContains(t, script,
		`(turns=${_actual_turns} vs ceiling=${_max_turns_profile})`) {
		return
	}
	// Forbidden: legacy misleading format.
	if acsassert.FileContainsAny(script, `(${_actual_turns}x ceiling)`) {
		t.Errorf("%s: legacy misleading format still present", script)
	}
	// Sanity: turn-overrun event-name token preserved.
	if !acsassert.FileContains(t, script, `"turn-overrun"`) {
		return
	}
}
