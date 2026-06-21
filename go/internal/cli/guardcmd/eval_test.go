package guardcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// writeEval writes a single eval markdown file into dir and returns its path.
func writeEval(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

func fenceCmd(cmd string) string { return "```bash\n" + cmd + "\n```\n" }

// TestRunEval_DiversityCheck_ExitCodes exercises the CLI dispatch + exit-code
// contract (0 PASS, 1 WARN, 2 HALT, 10 bad args) for diversity-check.
func TestRunEval_DiversityCheck_ExitCodes(t *testing.T) {
	cases := []struct {
		name   string
		files  map[string]string
		wantRC int
	}{
		{
			name:   "three all-positive evals → HALT(2)",
			files:  map[string]string{"a.md": fenceCmd(`grep -q x f`), "b.md": fenceCmd(`grep -q y f`), "c.md": fenceCmd(`grep -q z f`)},
			wantRC: 2,
		},
		{
			name:   "mixed with negative case → PASS(0)",
			files:  map[string]string{"a.md": fenceCmd(`grep -q x f`), "b.md": fenceCmd(`! grep -q y f`), "c.md": fenceCmd(`grep -q z f`)},
			wantRC: 0,
		},
		{
			name:   "single positive eval → WARN(1)",
			files:  map[string]string{"a.md": fenceCmd(`grep -q x f`)},
			wantRC: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for n, b := range tc.files {
				writeEval(t, dir, n, b)
			}
			var stdout, stderr bytes.Buffer
			rc := RunEval([]string{"diversity-check", dir}, nil, &stdout, &stderr)
			if rc != tc.wantRC {
				t.Errorf("rc=%d, want %d (stdout=%s stderr=%s)", rc, tc.wantRC, stdout.String(), stderr.String())
			}
		})
	}
}

// TestRunEval_DiversityCheck_BadArgs covers the missing-path and unknown-dir paths.
func TestRunEval_DiversityCheck_BadArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if rc := RunEval([]string{"diversity-check"}, nil, &stdout, &stderr); rc != 10 {
		t.Errorf("missing path: rc=%d, want 10", rc)
	}
	stdout.Reset()
	stderr.Reset()
	if rc := RunEval([]string{"diversity-check", "/no/such/dir/xyz"}, nil, &stdout, &stderr); rc != 1 {
		t.Errorf("missing dir: rc=%d, want 1 (internal error)", rc)
	}
}

// TestRunEval_UnknownSubcommand covers the dispatch default arm.
func TestRunEval_UnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if rc := RunEval([]string{"bogus"}, nil, &stdout, &stderr); rc != 10 {
		t.Errorf("unknown subcommand: rc=%d, want 10", rc)
	}
	stdout.Reset()
	stderr.Reset()
	if rc := RunEval(nil, nil, &stdout, &stderr); rc != 10 {
		t.Errorf("no subcommand: rc=%d, want 10", rc)
	}
}
