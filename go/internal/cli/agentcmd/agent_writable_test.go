package agentcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const builderReport = "## Changes\nx.go\n## Tests\n1/1 PASS\n## Verdict\nVerdict: PASS\n"

func worktreeWithDirs(t *testing.T, dirs ...string) string {
	t.Helper()
	wt := linkedWorktree(t)
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(wt, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return wt
}

func TestAgentRun_TheBuildSeatRefusesAWritableThatNamesADirectory(t *testing.T) {
	cases := []struct {
		name     string
		writable string
	}{
		{"a top directory", "go"},
		{"a nested directory", "go/internal"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := project(t)
			wt := worktreeWithDirs(t, "go/internal")
			fb := &fakeBridge{artifact: builderReport}
			var stdout, stderr bytes.Buffer

			code := New(depsWith(fb))(builderArgs(root, "--worktree", wt, "--writable", tc.writable), strings.NewReader("x"), &stdout, &stderr)

			if code != exitUsage || !strings.Contains(stderr.String(), tc.writable) || !strings.Contains(stderr.String(), "directory") || len(fb.requests) != 0 {
				t.Errorf("exit %d stderr %q launches %d, want %d, a refusal that names %q and a directory, and no launch", code, stderr.String(), len(fb.requests), exitUsage, tc.writable)
			}
		})
	}
}

func TestAgentRun_TheBuildSeatAcceptsAWritableThatIsAFileOrNotYetThere(t *testing.T) {
	cases := []struct {
		name     string
		writable string
	}{
		{"a file that does not exist yet", "go/new_file.go"},
		{"a file that exists", "go/existing.go"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := project(t)
			wt := worktreeWithDirs(t, "go")
			if err := os.WriteFile(filepath.Join(wt, "go", "existing.go"), []byte("package x\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			fb := &fakeBridge{artifact: builderReport}
			trees := &treeStates{values: []string{"before", "after"}}

			code, _, stderr := runJSON(t, seatDeps(fb, trees), builderArgs(root, "--worktree", wt, "--writable", tc.writable), "build item X")

			if code != 0 || len(fb.requests) != 1 {
				t.Errorf("exit %d launches %d stderr %q, want 0 and one launch", code, len(fb.requests), stderr)
			}
		})
	}
}

func TestAgentRun_TheUsageStatesThatWritableNamesAFileNeverADirectory(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := New(depsWith(&fakeBridge{}))([]string{"help"}, strings.NewReader(""), &stdout, &stderr)

	stated := false
	for _, line := range strings.Split(stdout.String(), "\n") {
		if strings.Contains(line, "--writable") && strings.Contains(line, "directory") {
			stated = true
		}
	}
	if code != 0 || !stated {
		t.Errorf("exit %d, usage states the directory rule: %v; want 0 and a usage line that names --writable and a directory", code, stated)
	}
}
