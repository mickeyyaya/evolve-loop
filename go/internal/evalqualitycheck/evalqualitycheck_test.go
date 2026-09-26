package evalqualitycheck

import (
	"os"
	"path/filepath"
	"strings"
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

func TestCheck_NonBashFencedBlock_Ignored(t *testing.T) {
	path := writeEval(t, "```python\nexit(0)\n```\n")
	r, err := Check(Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range r.Commands {
		if strings.Contains(c.Line, "exit(0)") {
			t.Errorf("non-bash block content was parsed as a command: %+v", c)
		}
	}
	if r.Overall != LevelWarn {
		t.Errorf("Overall = %v, want LevelWarn — a python-fence-only eval has zero real bash graders", r.Overall)
	}
}

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

func TestCheck_MissingFile_Error(t *testing.T) {
	_, err := Check(Options{Path: "/no/such/file.md"})
	if err == nil {
		t.Error("Check on missing file: want error")
	}
}

func TestCheck_EmptyPath_Error(t *testing.T) {
	_, err := Check(Options{})
	if err == nil {
		t.Error("Check with empty Path: want error")
	}
}

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

func TestCheck_CommitPresenceRange_HALT(t *testing.T) {
	cases := []string{
		"git log --oneline 81d2c2f..HEAD",
		`test "$(git rev-list --count main..HEAD)" -ge 6`,
		"git rev-list v1.0..HEAD",
		`git log main..feature | grep -q "fix(core)"`,
		"git log --oneline origin/main...HEAD",
		"git rev-list --count main..",
	}
	for _, c := range cases {
		path := writeEval(t, "```bash\n"+c+"\n```\n")
		r, err := Check(Options{Path: path})
		if err != nil {
			t.Fatal(err)
		}
		if r.Overall != LevelHalt {
			t.Errorf("%q: Overall=%d, want HALT(2)", c, r.Overall)
		}
		if len(r.Commands) != 1 || !strings.Contains(r.Commands[0].Reason, "content parity") {
			t.Errorf("%q: Reason=%q, want pointer to the content-parity pattern", c, r.Commands[0].Reason)
		}
	}
}

func TestCheck_ContentParityAndPlainGit_PASS(t *testing.T) {
	cases := []string{
		"git diff 81d2c2f..HEAD --quiet -- go/",
		"git diff --quiet origin/main -- .evolve/",
		"git log -1 --format=%H",
		"git rev-parse HEAD",
		"git log -p dir/../file",
		"git log --follow -- docs/../README.md",
		"git log ..HEAD", // left-open range: a known gap of commitPresenceRE
	}
	for _, c := range cases {
		path := writeEval(t, "```bash\n"+c+"\n```\n")
		r, err := Check(Options{Path: path})
		if err != nil {
			t.Fatal(err)
		}
		if r.Overall != LevelPass {
			t.Errorf("%q: Overall=%d, want PASS(0)", c, r.Overall)
		}
	}
}
