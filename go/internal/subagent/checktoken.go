package subagent

import (
	"bytes"
	"fmt"
	"os"
)

// CheckTokenResult is either OK or INTEGRITY_FAIL. Reason carries a
// human-readable message; callers can format it however they like.
type CheckTokenResult struct {
	OK     bool
	Reason string
}

// CheckToken validates that the artifact at path exists and contains the
// challenge token. Returns a struct so the CLI shim can map to exit codes
// without re-deciding.
func CheckToken(artifactPath, token string) CheckTokenResult {
	body, err := os.ReadFile(artifactPath)
	if err != nil {
		if os.IsNotExist(err) {
			return CheckTokenResult{OK: false, Reason: fmt.Sprintf("artifact missing: %s", artifactPath)}
		}
		return CheckTokenResult{OK: false, Reason: fmt.Sprintf("artifact unreadable %s: %v", artifactPath, err)}
	}
	if !bytes.Contains(body, []byte(token)) {
		return CheckTokenResult{OK: false, Reason: fmt.Sprintf("token absent from %s", artifactPath)}
	}
	return CheckTokenResult{OK: true, Reason: fmt.Sprintf("OK: token present in %s", artifactPath)}
}
