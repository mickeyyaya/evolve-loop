package phasecontract

import (
	"os"
	"path/filepath"
	"strings"
)

// ChallengeToken returns the trimmed per-cycle token from <workspace>/challenge-token.txt; only a non-empty token counts.
func ChallengeToken(workspace string) (string, bool) {
	raw, err := os.ReadFile(filepath.Join(workspace, "challenge-token.txt"))
	if err != nil {
		return "", false
	}
	tok := strings.TrimSpace(string(raw))
	return tok, tok != ""
}
