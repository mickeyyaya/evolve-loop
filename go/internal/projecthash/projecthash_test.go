package projecthash

import (
	"os/exec"
	"strings"
	"testing"
)

// Goldens captured from:
//
//	printf '%s' "$INPUT" | shasum -a 256 | head -c 8
//
// on macOS (the canonical preflight-environment.sh path; sha256sum on
// Linux produces the same bytes for the same input).
//
// Source of the bash logic: scripts/dispatch/preflight-environment.sh:260-264
//
// These vectors guard against any silent encoding/length drift between
// the bash and Go ports — multi-project worktree isolation depends on
// byte-exact equivalence (see feedback_multi_project_isolation.md).
func TestCompute_GoldenVectors(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"empty_string", "", "e3b0c442"},
		{"evolve_repo_root", "/Users/danleemh/ai/claude/evolve-loop", "21f9f7ae"},
		{"tmp_test_project", "/tmp/test-project", "b71e47fd"},
		{"short_home", "/Users/danleemh", "200d7b19"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Compute(tc.input)
			if got != tc.want {
				t.Errorf("Compute(%q)=%q, want %q (bash golden)", tc.input, got, tc.want)
			}
		})
	}
}

func TestCompute_LengthIs8(t *testing.T) {
	for _, in := range []string{"", "x", "a longer path with spaces and /slashes", strings.Repeat("a", 1024)} {
		got := Compute(in)
		if len(got) != 8 {
			t.Errorf("Compute(%q) length=%d, want 8", in, len(got))
		}
		for _, r := range got {
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
				t.Errorf("Compute(%q)=%q contains non-hex-lower char %q", in, got, r)
			}
		}
	}
}

func TestCompute_Deterministic(t *testing.T) {
	in := "/some/path"
	a := Compute(in)
	b := Compute(in)
	if a != b {
		t.Errorf("non-deterministic: a=%q, b=%q", a, b)
	}
}

// TestCompute_MatchesBash spot-checks the live bash pipeline. Skipped
// when shasum is unavailable (e.g. sparse CI image). This is the
// safety net that catches any regression in either side.
func TestCompute_MatchesBash(t *testing.T) {
	if _, err := exec.LookPath("shasum"); err != nil {
		t.Skip("shasum not available")
	}
	for _, in := range []string{
		"",
		"/Users/danleemh/ai/claude/evolve-loop",
		"/tmp",
		"path with unicode 日本語",
		strings.Repeat("a/", 64),
	} {
		t.Run(in, func(t *testing.T) {
			cmd := exec.Command("bash", "-c",
				`printf '%s' "$0" | shasum -a 256 | head -c 8`, in)
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("bash: %v", err)
			}
			bashHash := strings.TrimSpace(string(out))
			goHash := Compute(in)
			if bashHash != goHash {
				t.Errorf("input=%q bash=%q go=%q", in, bashHash, goHash)
			}
		})
	}
}

func TestForProjectRoot_FallsBackOnEmpty(t *testing.T) {
	// preflight-environment.sh:262-264 falls back to literal "default"
	// when EVOLVE_PROJECT_ROOT is empty. The Go port must match.
	if got := ForProjectRoot(""); got != "default" {
		t.Errorf("ForProjectRoot('')=%q, want 'default'", got)
	}
}

func TestForProjectRoot_HappyPath(t *testing.T) {
	got := ForProjectRoot("/Users/danleemh/ai/claude/evolve-loop")
	if got != "21f9f7ae" {
		t.Errorf("ForProjectRoot=%q, want 21f9f7ae", got)
	}
}
