package guardcmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunEvalQualityCheck_AnUnknownFlagIsAUsageErrorBeforeAnyEvalIsRead(t *testing.T) {
	var stdout, stderr bytes.Buffer

	rc := runEvalQualityCheck([]string{"--no-such-flag", "eval.md"}, &stdout, &stderr)

	if rc != 10 {
		t.Fatalf("rc = %d, want the usage exit 10", rc)
	}
	if stdout.Len() != 0 {
		t.Errorf("no verdict may be printed for a usage error: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "no-such-flag") {
		t.Errorf("the flag error must name the flag: %q", stderr.String())
	}
}
