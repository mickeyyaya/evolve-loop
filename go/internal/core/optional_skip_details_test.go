package core

import (
	"fmt"
	"strings"
	"testing"
)

func TestOptionalSkipDetails_SplitsByErrorClass(t *testing.T) {
	for _, tc := range []struct {
		name     string
		err      error
		wantKind string
		wantIn   string
	}{
		{"infra timeout keeps the original key", ErrArtifactTimeout, "optional_infra_skip", "exhausted infra retries"},
		{"transient bridge keeps the original key", ErrTransientBridgeFailure, "optional_infra_skip", "exhausted infra retries"},
		{"missing persona gets its own key", fmt.Errorf("phase x: load agent: %w", ErrAgentDocMissing),
			"optional_missing_persona_skip", "persona doc missing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kind, msg, diags := optionalSkipDetails(Phase("x"), tc.err)
			if kind != tc.wantKind {
				t.Errorf("kind = %q, want %q", kind, tc.wantKind)
			}
			if !strings.Contains(msg, tc.wantIn) {
				t.Errorf("msg %q does not name the class (%q)", msg, tc.wantIn)
			}
			if len(diags) != 1 || diags[0].Severity != "warn" || diags[0].Message != msg {
				t.Errorf("diagnostic must carry the same message at warn severity, got %+v", diags)
			}
		})
	}
}
