package evalqualitycheck

import (
	"os"
	"path/filepath"
	"testing"
)

func writeEval(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "eval.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestCheck_NonTrivialCommand_PASS — a real workspace-touching command
// classifies as PASS.
func TestCheck_NonTrivialCommand_PASS(t *testing.T) {
	path := writeEval(t, "```bash\ngo test ./...\n```\n")
	r, err := Check(Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if r.Overall != LevelPass {
		t.Errorf("Overall=%d, want PASS(0)", r.Overall)
	}
}

// TestCheck_TautologyExact_HALT — :, true, exit 0 → HALT.
func TestCheck_TautologyExact_HALT(t *testing.T) {
	cases := []string{":", "true", "exit 0", "/bin/true"}
	for _, c := range cases {
		path := writeEval(t, "```bash\n"+c+"\n```\n")
		r, err := Check(Options{Path: path})
		if err != nil {
			t.Fatal(err)
		}
		if r.Overall != LevelHalt {
			t.Errorf("%q: Overall=%d, want HALT(2)", c, r.Overall)
		}
	}
}

// TestCheck_TautologyBracket_HALT — [ true ], [ 1 -eq 1 ] → HALT.
func TestCheck_TautologyBracket_HALT(t *testing.T) {
	cases := []string{"[ true ]", "[ 1 -eq 1 ]", `[ "a" = "a" ]`}
	for _, c := range cases {
		path := writeEval(t, "```bash\n"+c+"\n```\n")
		r, err := Check(Options{Path: path})
		if err != nil {
			t.Fatal(err)
		}
		if r.Overall != LevelHalt {
			t.Errorf("%q: Overall=%d, want HALT(2)", c, r.Overall)
		}
	}
}

// TestCheck_EchoOnly_WARN — echo doesn't inspect anything.
func TestCheck_EchoOnly_WARN(t *testing.T) {
	path := writeEval(t, "```bash\necho \"hello\"\n```\n")
	r, err := Check(Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if r.Overall != LevelWarn {
		t.Errorf("Overall=%d, want WARN(1)", r.Overall)
	}
}

// TestCheck_GrepInlineConstant_WARN — grep against a literal in its
// own args is a weak signal.
func TestCheck_GrepInlineConstant_WARN(t *testing.T) {
	path := writeEval(t, "```bash\ngrep \"foo\" \"foobar\"\n```\n")
	r, err := Check(Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if r.Overall != LevelWarn {
		t.Errorf("Overall=%d, want WARN(1)", r.Overall)
	}
}

// TestCheck_WorstOf_HALTBeatsPASS — the overall verdict reflects the
// most severe classification, not the average.
func TestCheck_WorstOf_HALTBeatsPASS(t *testing.T) {
	path := writeEval(t, "```bash\ngo test ./...\ntrue\n```\n")
	r, err := Check(Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if r.Overall != LevelHalt {
		t.Errorf("Overall=%d, want HALT(2) (worst-of)", r.Overall)
	}
	if len(r.Commands) != 2 {
		t.Errorf("Commands len=%d, want 2", len(r.Commands))
	}
}

// TestCheck_NonBashFencedBlock_Ignored — only bash fences are parsed.
func TestCheck_NonBashFencedBlock_Ignored(t *testing.T) {
	path := writeEval(t, "```python\nexit(0)\n```\n")
	r, err := Check(Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Commands) != 0 {
		t.Errorf("non-bash block should be ignored; got %+v", r.Commands)
	}
}

// TestCheck_CommentsAndBlanksIgnored — # comments and blank lines
// inside bash blocks are skipped.
func TestCheck_CommentsAndBlanksIgnored(t *testing.T) {
	path := writeEval(t, "```bash\n# this is a comment\n\ngo build ./...\n```\n")
	r, err := Check(Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Commands) != 1 {
		t.Errorf("expected 1 command after stripping comments; got %+v", r.Commands)
	}
}

// TestCheck_MissingFile_Error — file-not-found surfaces as error.
func TestCheck_MissingFile_Error(t *testing.T) {
	_, err := Check(Options{Path: "/no/such/file.md"})
	if err == nil {
		t.Error("Check on missing file: want error")
	}
}

// TestCheck_EmptyPath_Error — required-field validation.
func TestCheck_EmptyPath_Error(t *testing.T) {
	_, err := Check(Options{})
	if err == nil {
		t.Error("Check with empty Path: want error")
	}
}

// TestCheck_MultipleBashBlocks_Concatenated — all bash blocks
// contribute commands.
func TestCheck_MultipleBashBlocks_Concatenated(t *testing.T) {
	path := writeEval(t, "## section\n```bash\ngo build\n```\n\n## second\n```bash\ngo test\n```\n")
	r, err := Check(Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Commands) != 2 {
		t.Errorf("Commands len=%d, want 2 (one per block)", len(r.Commands))
	}
}

// TestClassify_DirectUnit_AllLevels — direct helper test pinning
// each classification branch.
func TestClassify_DirectUnit_AllLevels(t *testing.T) {
	cases := []struct {
		cmd  string
		want Level
	}{
		{"go test ./...", LevelPass},
		{":", LevelHalt},
		{"true", LevelHalt},
		{"exit 0", LevelHalt},
		{"[ true ]", LevelHalt},
		{"echo hi", LevelWarn},
		{`grep "x" "xx"`, LevelWarn},
		{"ls -la", LevelPass},
	}
	for _, c := range cases {
		got := classify(c.cmd).Level
		if got != c.want {
			t.Errorf("classify(%q) = %d, want %d", c.cmd, got, c.want)
		}
	}
}
