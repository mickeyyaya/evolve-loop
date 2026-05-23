package ledgerverify

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRoleCount_UnknownRole covers the defensive default branch in
// roleCount. VerifyCycle never passes an unknown role through, but if
// the required set is ever extended without updating roleCount, this
// asserts the defensive behavior is `return 0`.
func TestRoleCount_UnknownRole(t *testing.T) {
	t.Parallel()
	r := Result{Scout: 5, Builder: 4}
	if got := roleCount(r, "operator"); got != 0 {
		t.Fatalf("unknown role count=%d want 0", got)
	}
}

// TestLoadVerifyContext_MissingIntentKey covers the
// `_, ok := raw["intent_required"]; !ok` branch — JSON file is valid
// but doesn't include the intent_required key.
func TestLoadVerifyContext_MissingIntentKey(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "cycle-state.json"), []byte(`{"other_field":42}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	vc := LoadVerifyContext(ws, "")
	if vc.IntentRequired {
		t.Fatalf("missing key should default to false")
	}
}
