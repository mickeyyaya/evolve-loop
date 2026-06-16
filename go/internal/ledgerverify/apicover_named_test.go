package ledgerverify

import (
	"os"
	"path/filepath"
	"testing"
)

// TestVerifyContext_LoadResolvesBothFields names the ledgerverify.VerifyContext
// type (returned by LoadVerifyContext but never named in a test) and pins the
// full struct contract: both fields are resolved together from disk —
// intent_required from cycle-state.json and the trimmed .cycle-verdict.
func TestVerifyContext_LoadResolvesBothFields(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "cycle-state.json"), []byte(`{"intent_required":true}`), 0o644); err != nil {
		t.Fatalf("write cycle-state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, ".cycle-verdict"), []byte(" PASS \n"), 0o644); err != nil {
		t.Fatalf("write verdict: %v", err)
	}

	want := VerifyContext{IntentRequired: true, CycleVerdict: "PASS"}
	got := LoadVerifyContext(ws, "")
	if got != want {
		t.Errorf("LoadVerifyContext = %+v, want %+v", got, want)
	}
}
