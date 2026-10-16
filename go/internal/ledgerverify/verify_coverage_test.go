package ledgerverify

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRoleCount_UnknownRole(t *testing.T) {
	t.Parallel()
	r := Result{Scout: 5, Builder: 4}
	if got := roleCount(r, "operator"); got != 0 {
		t.Fatalf("unknown role count=%d want 0", got)
	}
}

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
