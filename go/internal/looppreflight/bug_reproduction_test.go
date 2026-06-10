package looppreflight

import (
	"strconv"
	"strings"
	"testing"
)

// TestBootRCName_DefaultBranch_IncludesExitCode reproduces the diagnostic gap:
// bootRCName() returns "boot failure" for unrecognized exit codes, discarding the
// numeric value. Operators cannot distinguish rc=42 from rc=99 from any other
// unrecognized code in preflight output — the diagnostic is semantically empty.
//
// Reproducer for cycle-270 fault-localization Rank 2 (boot.go bootRCName):
// RED on the pre-fix tree (bare "boot failure", no numeric code); GREEN once
// the default branch carries the exit code ("boot failure (exit=%d)"). The
// assertion is format-agnostic on purpose — it requires the number, not the
// exact phrasing.
func TestBootRCName_DefaultBranch_IncludesExitCode(t *testing.T) {
	unknownCodes := []struct {
		rc   int
		name string
	}{
		{1, "generic-error"},
		{42, "arbitrary"},
		{99, "high-unrecognized"},
		{128, "signal-base"},
	}
	for _, tc := range unknownCodes {
		t.Run(tc.name, func(t *testing.T) {
			got := bootRCName(tc.rc)
			if !strings.Contains(got, strconv.Itoa(tc.rc)) {
				t.Errorf("bootRCName(%d) = %q: numeric exit code absent from diagnostic string; operators cannot identify the actual failure mode without it", tc.rc, got)
			}
		})
	}
}
