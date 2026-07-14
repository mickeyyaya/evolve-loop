package core

import (
	"errors"
	"fmt"
	"testing"
)

// TestIsInfraTeardownError pins the single-source reconcile trigger: an infra
// teardown is an artifact-wait timeout OR a transient bridge failure (quota /
// liveness), including wrapped wire shapes. A substantive launch/boot/safety
// error, a generic error, and nil are NOT infra teardowns — those hard-fail
// without consulting the on-disk deliverable (anti-gaming boundary).
func TestIsInfraTeardownError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"artifact-timeout sentinel", ErrArtifactTimeout, true},
		{"artifact-timeout wrapped (exit 81 wire shape)", fmt.Errorf("bridge: launch exit=81: %w", ErrArtifactTimeout), true},
		{"transient sentinel", ErrTransientBridgeFailure, true},
		{"transient wrapped (exit 85 quota wire shape)", fmt.Errorf("bridge: launch exit=85: %w", ErrTransientBridgeFailure), true},
		{"transient wrapped (exit 80)", fmt.Errorf("bridge: launch exit=80: %w", ErrTransientBridgeFailure), true},
		{"substantive launch error (exit 2 safety-gate)", errors.New("bridge: launch exit=2"), false},
		{"all-families-exhausted (batch-level, not per-phase reconcile)", ErrAllFamiliesExhausted, false},
		{"generic error", errors.New("boom"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsInfraTeardownError(tc.err); got != tc.want {
				t.Errorf("IsInfraTeardownError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
