package phasecontract

import (
	"os"
	"path/filepath"
	"testing"
)

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
