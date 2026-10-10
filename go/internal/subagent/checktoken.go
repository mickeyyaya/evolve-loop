package subagent

import (
	"bytes"
	"fmt"
	"os"
)

type CheckTokenResult struct {
	OK     bool
	Reason string
}

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
