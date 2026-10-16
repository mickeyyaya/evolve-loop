package phasecontract

import (
	"os"
	"path/filepath"
	"strings"
)

func ChallengeToken(workspace string) (string, bool) {
	raw, err := os.ReadFile(filepath.Join(workspace, "challenge-token.txt"))
	if err != nil {
		return "", false
	}
	tok := strings.TrimSpace(string(raw))
	return tok, tok != ""
}
