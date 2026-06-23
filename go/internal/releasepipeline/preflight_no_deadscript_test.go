package releasepipeline

import (
	"strings"
	"testing"
)

// TestDefaultFullDryRunPreflight_NoDeadScript guards the T1.3 fix: the step-0
// "full dry-run preflight" (`evolve release --require-preflight`) must run the
// Go-native preflight, not shell out to legacy/scripts/release/full-dry-run.sh —
// which the 2026-06-18 script→Go migration deleted, making the flag a guaranteed
// hard-fail. Against a throwaway dir the Go preflight errors for a REAL reason
// (no git / no version files); it must never fail by referencing the dead script.
func TestDefaultFullDryRunPreflight_NoDeadScript(t *testing.T) {
	err := defaultFullDryRunPreflight(t.TempDir(), "99.0.0")
	if err == nil {
		return // acceptable — the Go preflight found nothing to object to
	}
	msg := err.Error()
	if strings.Contains(msg, "full-dry-run.sh") || strings.Contains(msg, "legacy/scripts") {
		t.Errorf("step-0 preflight still depends on the deleted legacy script: %v", err)
	}
}
