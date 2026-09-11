//go:build integration

package guardcmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunEval_VerifyIndentsMultilineScript(t *testing.T) {
	dir := t.TempDir()
	path := writeEval(t, dir, "multiline.md", fenceCmd("name=present\ntest \"$name\" = present"))
	var stdout, stderr bytes.Buffer

	rc := RunEval([]string{"verify", path, dir}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0; stderr=%s", rc, stderr.String())
	}
	if line := "\n         test \"$name\" = present\n"; !strings.Contains(stdout.String(), line) {
		t.Fatalf("stdout does not indent the continued script line:\n%s", stdout.String())
	}
}
