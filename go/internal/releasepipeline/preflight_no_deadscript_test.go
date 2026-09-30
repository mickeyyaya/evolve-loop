package releasepipeline

import (
	"strings"
	"testing"
)

func TestDefaultFullDryRunPreflight_NoDeadScript(t *testing.T) {
	err := defaultFullDryRunPreflight(t.TempDir(), "99.0.0")
	if err == nil {
		return
	}
	msg := err.Error()
	if strings.Contains(msg, "full-dry-run.sh") || strings.Contains(msg, "legacy/scripts") {
		t.Errorf("step-0 preflight still depends on the deleted legacy script: %v", err)
	}
}
