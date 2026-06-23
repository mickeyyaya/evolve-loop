// shiperror_testhelpers_test.go — shared assertion helpers for the structured
// ship-error protocol (core.ShipError). Replaces the legacy `var ie
// *IntegrityError; errors.As(err, &ie)` pattern: most ship-refusal sites are
// now NON-integrity classes (precondition/config/transient), so they return a
// bare *core.ShipError that does not match *IntegrityError. These helpers
// recover the structured error and assert Code/Class while preserving the
// original tests' message-containment intent.
package ship

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// mustShipErr recovers a *core.ShipError from err or fails the test.
func mustShipErr(t *testing.T, err error) *core.ShipError {
	t.Helper()
	se, ok := core.AsShipError(err)
	if !ok {
		t.Fatalf("expected a recoverable *core.ShipError; got %T: %v", err, err)
	}
	return se
}

// wantShipErr recovers the ShipError, asserts its Code + Class, and (when
// msgSubstr != "") asserts the message contains msgSubstr. Returns it for
// any further assertions.
func wantShipErr(t *testing.T, err error, code core.ShipErrorCode, class core.ShipErrorClass, msgSubstr string) *core.ShipError {
	t.Helper()
	se := mustShipErr(t, err)
	if se.Code != code {
		t.Fatalf("want Code=%s, got %s (%v)", code, se.Code, err)
	}
	if se.Class != class {
		t.Fatalf("want Class=%s, got %s (%v)", class, se.Class, err)
	}
	if msgSubstr != "" && !strings.Contains(se.Message, msgSubstr) {
		t.Errorf("message should contain %q; got %q", msgSubstr, se.Message)
	}
	return se
}
