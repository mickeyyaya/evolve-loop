package phasecontract

import (
	"os"
	"path/filepath"
	"testing"
)

// TestChallengeToken_TrimsAndRejectsEmpty — ChallengeToken is the reader of
// <workspace>/challenge-token.txt the runner's prompt preparation and the
// verdict engine's ACS floor share (ADR-0103 unit 11, review fold F1): trimmed,
// and only a non-empty token counts. Moved verbatim from the verdict leaf's
// test 31a. Kills `TrimSpace dropped`, `empty token accepted`.
func TestChallengeToken_TrimsAndRejectsEmpty(t *testing.T) {
	ws := t.TempDir()
	if _, ok := ChallengeToken(ws); ok {
		t.Error("no file ⇒ no token")
	}
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(ws, "challenge-token.txt"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("  tok \n")
	if tok, ok := ChallengeToken(ws); !ok || tok != "tok" {
		t.Errorf("trimmed: %q %v", tok, ok)
	}
	write("   \n\n")
	if tok, ok := ChallengeToken(ws); ok || tok != "" {
		t.Errorf("whitespace only ⇒ no token: %q %v", tok, ok)
	}
	write("")
	if _, ok := ChallengeToken(ws); ok {
		t.Error("empty ⇒ no token")
	}
}
