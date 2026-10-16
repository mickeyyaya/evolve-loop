package ledgerverify

import (
	"os"
	"path/filepath"
	"testing"
)

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
