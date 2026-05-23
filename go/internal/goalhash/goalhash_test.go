package goalhash

import (
	"os/exec"
	"strings"
	"testing"
)

// TestNormalize_KnownTransforms covers each layer of the bash pipeline
// (lowercase, whitespace squeeze, trim) independently and combined.
func TestNormalize_KnownTransforms(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"lower_only", "Hello World", "hello world"},
		{"upper_collapse_ws", "FIX   the   BUG", "fix the bug"},
		{"leading_trailing_space", "  goal text  ", "goal text"},
		{"tab_to_space", "a\tb", "a b"},
		{"newline_to_space", "a\nb", "a b"},
		{"mixed_whitespace_run", "a \t\n b", "a b"},
		{"unicode_lower", "Improve UI 日本語", "improve ui 日本語"},
		{"only_whitespace", "   \t\n  ", ""},
		{"single_char", "X", "x"},
		{"contractions_preserved", "doesn't won't can't", "doesn't won't can't"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Normalize(tc.in)
			if got != tc.want {
				t.Errorf("Normalize(%q)=%q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestCompute_GoldenVectors guards against silent drift in either layer
// (Normalize or SHA256). Vectors captured from:
//
//	printf '%s' "$NORMALIZED" | sha256sum | awk '{print $1}'
func TestCompute_GoldenVectors(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		// echo -n "" | sha256sum
		{"empty", "", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		// echo -n "hello world" | sha256sum
		{"lower_no_ws", "hello world", "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"},
		// Hash should be invariant to leading/trailing whitespace + case
		{"trim_case_invariant_1", "  Hello World  ", "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"},
		{"trim_case_invariant_2", "HELLO\tWORLD", "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Compute(tc.in)
			if got != tc.want {
				t.Errorf("Compute(%q)=%q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestCompute_LengthIs64(t *testing.T) {
	for _, in := range []string{"", "x", "a longer goal description with many words", strings.Repeat("g ", 256)} {
		got := Compute(in)
		if len(got) != 64 {
			t.Errorf("Compute(%q) length=%d, want 64", in, len(got))
		}
		for _, r := range got {
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
				t.Errorf("Compute(%q)=%q contains non-hex-lower char %q", in, got, r)
				break
			}
		}
	}
}

func TestShort_Is8Chars(t *testing.T) {
	got := Short("anything")
	if len(got) != 8 {
		t.Errorf("Short length=%d, want 8", len(got))
	}
	if Short("anything") != Compute("anything")[:8] {
		t.Errorf("Short should equal Compute()[:8]")
	}
}

// TestCompute_MatchesBash is the canonical safety net: run the exact
// bash pipeline (Normalize + sha256sum) and assert byte-equivalence.
// Skipped if sha256sum (or shasum) not available.
func TestCompute_MatchesBash(t *testing.T) {
	hasher := "sha256sum"
	if _, err := exec.LookPath(hasher); err != nil {
		hasher = "shasum"
		if _, err := exec.LookPath(hasher); err != nil {
			t.Skip("neither sha256sum nor shasum available")
		}
	}

	cases := []string{
		"",
		"simple goal",
		"  Mixed  CASE  with   gaps  ",
		"unicode 日本語 and tabs\there",
		"with\nnewline\nseparators",
		"goal with apostrophe doesn't break",
		strings.Repeat("x ", 32) + "end",
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			// Run the actual bash pipeline:
			//   printf '%s\n' "$IN" | tr '[:upper:]' '[:lower:]' \
			//     | tr -s '[:space:]' ' ' | sed 's/^ //; s/ $//'
			//   then pipe to sha256sum | awk '{print $1}'
			pipeline := `printf '%s\n' "$0" | tr '[:upper:]' '[:lower:]' | tr -s '[:space:]' ' ' | sed 's/^ //; s/ $//'`
			if hasher == "sha256sum" {
				pipeline = `(` + pipeline + `) | tr -d '\n' | sha256sum | awk '{print $1}'`
			} else {
				pipeline = `(` + pipeline + `) | tr -d '\n' | shasum -a 256 | awk '{print $1}'`
			}
			// Note: we strip the trailing newline from the normalize
			// pipeline (added by `printf '%s\n'`) before hashing —
			// equivalent to bash's `text="$(normalize_goal ...)"`
			// command substitution which strips trailing newlines.
			cmd := exec.Command("bash", "-c", pipeline, in)
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("bash pipeline: %v", err)
			}
			bashHash := strings.TrimSpace(string(out))
			goHash := Compute(in)
			if bashHash != goHash {
				t.Errorf("input=%q\n  bash=%q\n  go  =%q\n  bash-normalized=%q\n  go-normalized=%q",
					in, bashHash, goHash, runBashNormalize(t, in), Normalize(in))
			}
		})
	}
}

func runBashNormalize(t *testing.T, in string) string {
	t.Helper()
	cmd := exec.Command("bash", "-c",
		`printf '%s\n' "$0" | tr '[:upper:]' '[:lower:]' | tr -s '[:space:]' ' ' | sed 's/^ //; s/ $//' | tr -d '\n'`, in)
	out, _ := cmd.Output()
	return string(out)
}

func TestCompute_Deterministic(t *testing.T) {
	in := "any goal"
	if Compute(in) != Compute(in) {
		t.Errorf("non-deterministic")
	}
}
