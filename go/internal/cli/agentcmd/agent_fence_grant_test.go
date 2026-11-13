package agentcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

var seatBase = map[string]string{
	"go/x.go":        "package x // base\n",
	"go/other.go":    "package x // other base\n",
	"go/gen/keep.go": "package gen // base\n",
}

func writeSeatFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func seatWorktree(t *testing.T) string {
	t.Helper()
	repo := gittest.Fixture(t)
	for rel, content := range seatBase {
		writeSeatFile(t, repo.Dir, rel, content)
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	wt := filepath.Join(t.TempDir(), "seat")
	repo.Git("worktree", "add", "-q", "-b", "seat", wt)
	return wt
}

func seatWrites(t *testing.T, writes map[string]string) func(core.BridgeRequest) {
	return func(req core.BridgeRequest) {
		for rel, content := range writes {
			writeSeatFile(t, req.Worktree, rel, content)
		}
	}
}

func readSeatFile(wt, rel string) (string, bool) {
	raw, err := os.ReadFile(filepath.Join(wt, filepath.FromSlash(rel)))
	return string(raw), err == nil
}

func errorDiagnosticsNaming(env Envelope, needles ...string) bool {
	for _, d := range env.Diagnostics {
		if d.Severity != "error" {
			continue
		}
		named := true
		for _, n := range needles {
			named = named && strings.Contains(d.Message, n)
		}
		if named {
			return true
		}
	}
	return false
}

func hasErrorDiagnostic(env Envelope) bool {
	for _, d := range env.Diagnostics {
		if d.Severity == "error" {
			return true
		}
	}
	return false
}

func TestAgentRun_TheBuildSeatThatCreatesAFileOutsideItsGrantGetsAnErrorThatNamesThePathAndTheGrant(t *testing.T) {
	root := project(t)
	wt := seatWorktree(t)
	fb := &fakeBridge{artifact: builderReport, onLaunch: seatWrites(t, map[string]string{
		"go/x.go":          "package x // seat edit\n",
		"go/stray_test.go": "package x // stray\n",
	})}

	_, env, stderr := runJSON(t, depsWith(fb), builderArgs(root, "--worktree", wt, "--writable", "go/x.go"), "build item X")

	if len(fb.requests) != 1 {
		t.Fatalf("launches %d stderr=%s, want 1", len(fb.requests), stderr)
	}
	if _, kept := readSeatFile(wt, "go/stray_test.go"); kept {
		t.Error("the fence must still remove the file that the seat created outside its grant")
	}
	if got, _ := readSeatFile(wt, "go/x.go"); got != "package x // seat edit\n" {
		t.Errorf("the granted file lost the seat edit: %q", got)
	}
	if !errorDiagnosticsNaming(env, "go/stray_test.go", "--writable") {
		t.Errorf("no error diagnostic names the removed path and the --writable next step: %+v", env.Diagnostics)
	}
}

func TestAgentRun_TheBuildSeatKeepsANewFileInAWritableDirAndRestoresAnExistingFileOutsideIt(t *testing.T) {
	root := project(t)
	wt := seatWorktree(t)
	fb := &fakeBridge{artifact: builderReport, onLaunch: seatWrites(t, map[string]string{
		"go/x.go":       "package x // seat edit\n",
		"go/gen/new.go": "package gen // new\n",
		"go/other.go":   "package x // seat tampered\n",
	})}
	args := builderArgs(root, "--worktree", wt, "--writable", "go/x.go", "--writable-dir", "go/gen")

	_, env, stderr := runJSON(t, depsWith(fb), args, "build item X")

	if len(fb.requests) != 1 {
		t.Fatalf("launches %d stderr=%s, want 1", len(fb.requests), stderr)
	}
	if got, kept := readSeatFile(wt, "go/gen/new.go"); !kept || got != "package gen // new\n" {
		t.Errorf("the new file in the granted directory did not survive: kept=%v content=%q", kept, got)
	}
	if got, _ := readSeatFile(wt, "go/other.go"); got != seatBase["go/other.go"] {
		t.Errorf("an existing file outside the grant was not restored: %q", got)
	}
	if got, _ := readSeatFile(wt, "go/x.go"); got != "package x // seat edit\n" {
		t.Errorf("the granted file lost the seat edit: %q", got)
	}
	if env.Verdict != core.VerdictPASS || errorDiagnosticsNaming(env, "go/gen/new.go") {
		t.Errorf("verdict %s diagnostics %+v: a new file inside the grant is no fault", env.Verdict, env.Diagnostics)
	}
}

func TestAgentRun_TheBuildSeatAcceptsAWritableDirAsItsOnlyGrant(t *testing.T) {
	root := project(t)
	wt := seatWorktree(t)
	fb := &fakeBridge{artifact: builderReport, onLaunch: seatWrites(t, map[string]string{"go/gen/new.go": "package gen // new\n"})}

	code, env, stderr := runJSON(t, depsWith(fb), builderArgs(root, "--worktree", wt, "--writable-dir", "go/gen"), "build item X")

	if code != 0 || env.Verdict != core.VerdictPASS || hasErrorDiagnostic(env) {
		t.Fatalf("exit %d verdict %s stderr=%s diagnostics=%+v, want 0, PASS and no error", code, env.Verdict, stderr, env.Diagnostics)
	}
	if _, kept := readSeatFile(wt, "go/gen/new.go"); !kept {
		t.Error("the new file in the only granted directory did not survive")
	}
}

func TestAgentRun_TheBuildSeatRefusesAWritableDirThatIsNotACleanLocalDirectory(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		reason string
	}{
		{"an escaping path", []string{"--writable", "go/x.go", "--writable-dir", "../up"}, "../up"},
		{"an absolute path", []string{"--writable", "go/x.go", "--writable-dir", "/etc"}, "/etc"},
		{"a path that is not clean", []string{"--writable", "go/x.go", "--writable-dir", "go/./gen"}, "go/./gen"},
		{"an existing file", []string{"--writable", "go/x.go", "--writable-dir", "go/other.go"}, "go/other.go"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := project(t)
			wt := seatWorktree(t)
			fb := &fakeBridge{artifact: builderReport}
			var stdout, stderr bytes.Buffer

			code := New(depsWith(fb))(builderArgs(root, append([]string{"--worktree", wt}, tc.args...)...), strings.NewReader("x"), &stdout, &stderr)

			if code != exitUsage || !strings.Contains(stderr.String(), tc.reason) || len(fb.requests) != 0 {
				t.Errorf("exit %d stderr %q launches %d, want %d, a refusal that names %q and no launch", code, stderr.String(), len(fb.requests), exitUsage, tc.reason)
			}
		})
	}
}

func TestAgentRun_AReaderRefusesAWritableDir(t *testing.T) {
	root := project(t)
	fb := &fakeBridge{artifact: passReport}
	var stdout, stderr bytes.Buffer
	args := []string{"run", "adversarial-review", "--project-root", root, "--worktree", seatWorktree(t), "--writable-dir", "go/gen"}

	code := New(depsWith(fb))(args, strings.NewReader("x"), &stdout, &stderr)

	if code != exitUsage || !strings.Contains(stderr.String(), "read-only") || len(fb.requests) != 0 {
		t.Errorf("exit %d stderr %q launches %d, want %d, a read-only refusal and no launch", code, stderr.String(), len(fb.requests), exitUsage)
	}
}

func TestAgentRun_TheUsageStatesTheWritableDirGrant(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := New(depsWith(&fakeBridge{}))([]string{"help"}, strings.NewReader(""), &stdout, &stderr)

	stated := false
	for _, line := range strings.Split(stdout.String(), "\n") {
		stated = stated || strings.Contains(line, "--writable-dir") && strings.Contains(line, "new file")
	}
	if code != 0 || !stated {
		t.Errorf("exit %d, usage states the new-file directory grant: %v; want 0 and a usage line that names --writable-dir and new file", code, stated)
	}
}

func TestAgentRun_TheBuildSeatThatCreatesAFileOutsideItsGrantEndsWithAVerdictThatIsNotPASS(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		writes  map[string]string
		removed string
	}{
		{"a file grant only", []string{"--writable", "go/x.go"}, map[string]string{"go/x.go": "package x // seat edit\n", "go/stray_test.go": "package x // stray\n"}, "go/stray_test.go"},
		{"a sibling that shares the directory name prefix", []string{"--writable", "go/x.go", "--writable-dir", "go/gen"}, map[string]string{"go/x.go": "package x // seat edit\n", "go/gen2/x.go": "package gen2\n"}, "go/gen2/x.go"},
		{"a file beside the granted directory", []string{"--writable-dir", "go/gen"}, map[string]string{"go/gen/new.go": "package gen // new\n", "go/stray.go": "package x\n"}, "go/stray.go"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := project(t)
			wt := seatWorktree(t)
			fb := &fakeBridge{artifact: builderReport, onLaunch: seatWrites(t, tc.writes)}

			_, env, stderr := runJSON(t, depsWith(fb), builderArgs(root, append([]string{"--worktree", wt}, tc.args...)...), "build item X")

			if len(fb.requests) != 1 {
				t.Fatalf("launches %d stderr=%s, want 1", len(fb.requests), stderr)
			}
			if env.Verdict == core.VerdictPASS || env.Verdict == "" {
				t.Errorf("verdict %q: a writer seat that created %s outside its grant must not end with PASS", env.Verdict, tc.removed)
			}
			if !errorDiagnosticsNaming(env, tc.removed, "--writable") {
				t.Errorf("no error diagnostic names %s and the --writable next step: %+v", tc.removed, env.Diagnostics)
			}
		})
	}
}

func TestAgentRun_TheJourneyOfASeatThatCreatesAFileOutsideItsGrantExitsNonZeroAndRecordsTheVerdict(t *testing.T) {
	root := project(t)
	wt := seatWorktree(t)
	fb := &fakeBridge{artifact: builderReport, onLaunch: seatWrites(t, map[string]string{
		"go/x.go":          "package x // seat edit\n",
		"go/stray_test.go": "package x // stray\n",
	})}

	code, env, stderr := runJSON(t, depsWith(fb), builderArgs(root, "--worktree", wt, "--writable", "go/x.go"), "build item X")

	if code != exitFail {
		t.Errorf("exit %d stderr=%s verdict %q, want %d: a script that gates on the exit code must see the seat fail", code, stderr, env.Verdict, exitFail)
	}
	if rec := readRecord(t, root, env.RunLabel); rec.Verdict == core.VerdictPASS || rec.Verdict != env.Verdict {
		t.Errorf("launch record verdict %q, envelope verdict %q: the record must carry the same non-PASS verdict", rec.Verdict, env.Verdict)
	}
	if _, kept := readSeatFile(wt, "go/stray_test.go"); kept {
		t.Error("the fence must still remove the file that the seat created outside its grant")
	}
}
