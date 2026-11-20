package landcmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

const (
	fakeSHA       = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	watchGreen    = `echo '[{"status":"completed","conclusion":"success","url":"https://ci/run/1","databaseId":1}]'`
	watchRed      = `echo '[{"status":"completed","conclusion":"failure","url":"https://ci/run/8","databaseId":8}]'`
	watchBroken   = `exit 1`
	fakeGHPattern = `#!/bin/sh
D=$(dirname "$0")
case "$*" in
"api repos/{owner}/{repo}/commits/"*) echo '{"sha":"SHA","parents":[{"sha":"BASE"}]}';;
"api repos/{owner}/{repo}/compare/"*) echo '{"status":"ahead","merge_base_commit":{"sha":"BASE"},"files":[{"filename":"a.go"}]}';;
"pr view 5 --json headRefOid") echo '{"headRefOid":"SHA"}';;
*"--json status,conclusion,url,databaseId") WATCH;;
"run list "*"--commit BASE"*) echo '[{"status":"completed","conclusion":"failure","databaseId":20,"headSha":"BASE","url":"u","attempt":2,"createdAt":"2026-01-01T00:00:00Z"}]';;
"run list "*) echo '[]';;
"run rerun 8 --failed") touch "$D/rerun";;
"run view 7 --json "*) echo '{"status":"completed","conclusion":"success","databaseId":7,"headSha":"SHA","url":"u","attempt":1,"createdAt":"2026-01-01T00:00:00Z"}';;
"run view 9 --json "*) echo '{"status":"in_progress","conclusion":"","databaseId":9,"headSha":"SHA","url":"u","attempt":1,"createdAt":"2026-01-01T00:00:00Z"}';;
"run view 8 --json "*) if [ -f "$D/rerun" ]; then A=3; else A=2; fi; echo '{"status":"completed","conclusion":"failure","databaseId":8,"headSha":"SHA","url":"u","attempt":'$A',"createdAt":"2026-01-01T00:00:00Z"}';;
"run view 10 --json "*) echo '{"status":"completed","conclusion":"failure","databaseId":10,"headSha":"SHA","url":"u","attempt":2,"createdAt":"2026-01-01T00:00:00Z"}';;
"run view 10 --log-failed"*) printf 'build\ttest\t--- FAIL: TestOther (0.00s)\nbuild\ttest\tFAIL\texample.com/m/pkg\t0.1s\n';;
"run view "[28]*" --log-failed"*) printf 'build\ttest\t--- FAIL: TestRed (0.00s)\nbuild\ttest\tFAIL\texample.com/m/pkg\t0.1s\n';;
*) exit 1;;
esac
`
)

func installFakeGH(t *testing.T, watch string) {
	t.Helper()
	t.Setenv(ipcenv.PhaseKey, "")
	t.Setenv(ipcenv.DispatchDepthKey, "")
	bin := t.TempDir()
	script := strings.NewReplacer("SHA", fakeSHA, "BASE", strings.Repeat("b", 40), "WATCH", watch).Replace(fakeGHPattern)
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestRunCI_ClassifyExitCodes(t *testing.T) {
	installFakeGH(t, watchGreen)
	root := e3PolicyRoot(t, "")
	cases := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{"a green run is retry safe and exits zero", []string{"classify", "run:7", "--project-root", root}, 0, "7", ""},
		{"a green run prints JSON with --json", []string{"classify", "run:7", "--json", "--project-root", root}, 0, `"retry_safe"`, ""},
		{"a run still going is refused", []string{"classify", "run:9", "--project-root", root}, exitRefused, "", "not completed"},
		{"a red run whose failure is already red on the base is retry safe", []string{"classify", "run:8", "--project-root", root}, 0, "retry-safe: yes", ""},
		{"a red run with a new failure is refused", []string{"classify", "run:10", "--project-root", root}, exitRefused, "retry-safe: no", ""},
		{"a run the host cannot read is an IO error", []string{"classify", "run:99", "--project-root", root}, exitIO, "", ciClassifyPrefix},
		{"a missing target is a usage error", []string{"classify"}, exitUsage, "", "want exactly one target"},
		{"a malformed policy is an IO error", []string{"classify", "run:7", "--project-root", e3PolicyRoot(t, `{"ci_watch":`)}, exitIO, "", ciClassifyPrefix},
		{"a non-positive poll interval is an IO error", []string{"classify", "run:7", "--project-root", e3PolicyRoot(t, `{"ci_watch":{"poll_s":0}}`)}, exitIO, "", "poll_s"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := runCICmd(tc.args, nil, &stdout, &stderr)
			if code != tc.wantCode {
				t.Fatalf("exit = %d, want %d; stdout %q stderr %q", code, tc.wantCode, stdout.String(), stderr.String())
			}
			if !strings.Contains(stdout.String(), tc.wantStdout) || !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Errorf("stdout %q stderr %q, want %q and %q", stdout.String(), stderr.String(), tc.wantStdout, tc.wantStderr)
			}
		})
	}
}

func TestRunCI_ClassifyFilesABaseRedItemAndReportsAnUnreadableInbox(t *testing.T) {
	installFakeGH(t, watchGreen)
	root := e3PolicyRoot(t, "")
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := runCICmd([]string{"classify", "run:8", "--rerun", "--project-root", root}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, want 0; stderr %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "filed fix-first item") {
		t.Errorf("stderr %q does not name the filed item", stderr.String())
	}
	if err := os.RemoveAll(filepath.Join(root, ".evolve", "inbox")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "inbox"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	installFakeGH(t, watchGreen)
	if code := runCICmd([]string{"classify", "run:8", "--rerun", "--project-root", root}, nil, &stdout, &stderr); code != exitIO {
		t.Errorf("exit = %d, want %d for an unreadable inbox; stderr %q", code, exitIO, stderr.String())
	}
}

func TestRunCI_ClassifyReportsAnUnwritableReport(t *testing.T) {
	installFakeGH(t, watchGreen)
	var stderr bytes.Buffer
	code := runCICmd([]string{"classify", "run:7", "--project-root", e3PolicyRoot(t, "")}, nil, &failingWriter{}, &stderr)
	if code != exitIO || !strings.Contains(stderr.String(), "broken pipe") {
		t.Errorf("exit = %d, stderr %q, want %d naming the broken pipe", code, stderr.String(), exitIO)
	}
}

func TestRunCI_ClassifyWithoutAWorkingDirectoryIsAnIOError(t *testing.T) {
	installFakeGH(t, watchGreen)
	breakWorkingDirectory(t)
	var stdout, stderr bytes.Buffer
	if code := runCICmd([]string{"classify", "run:7"}, nil, &stdout, &stderr); code != exitIO {
		t.Fatalf("exit = %d, want %d; stderr %q", code, exitIO, stderr.String())
	}
	if !strings.Contains(stderr.String(), "getwd") {
		t.Errorf("stderr %q does not name the failed working directory", stderr.String())
	}
}

func TestRunCI_ClassifyWarnsWhenTheModuleFileIsMissing(t *testing.T) {
	installFakeGH(t, watchGreen)
	var stdout, stderr bytes.Buffer
	if code := runCICmd([]string{"classify", "run:7", "--project-root", e3PolicyRoot(t, "")}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "every touched fact is unknown") {
		t.Errorf("stderr %q lacks the missing-module warning", stderr.String())
	}
}

func TestRunCI_DarwinSetRunsTheWiredHostVerb(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runCICmd([]string{"darwin-set", "check"}, nil, &stdout, &stderr); code != exitUsage {
		t.Errorf("exit = %d, want the host verb's %d", code, exitUsage)
	}
	if ciSubverbRunner("classify") != nil || ciSubverbRunner("watch") == nil || ciSubverbRunner("telemetry") == nil {
		t.Error("ciSubverbRunner must route watch and telemetry and leave classify to the dispatcher")
	}
}

func TestRunCI_WatchExitCodes(t *testing.T) {
	cases := []struct {
		name       string
		watch      string
		args       []string
		policy     string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{"a green sha exits zero and prints the row", watchGreen, []string{"--sha", "abcdef1"}, "", 0, "success", ""},
		{"a green tag watches both workflows", watchGreen, []string{"--tag", "v1"}, "", 0, "release", ""},
		{"a red PR is classified and refused", watchRed, []string{"--pr", "5"}, "", exitRefused, "failure", ""},
		{"a watch the host cannot fetch is an IO error", watchBroken, []string{"--sha", "abcdef1"}, "", exitIO, "", ciWatchPrefix},
		{"a malformed policy is an IO error", watchGreen, []string{"--sha", "abcdef1"}, `{"ci_watch":`, exitIO, "", ciWatchPrefix},
		{"a non-positive timeout is an IO error", watchGreen, []string{"--sha", "abcdef1"}, `{"ci_watch":{"timeout_s":0}}`, exitIO, "", "timeout_s"},
		{"a head that cannot be resolved is an IO error", watchGreen, []string{"--pr", "6"}, "", exitIO, "", ciWatchPrefix},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			installFakeGH(t, tc.watch)
			var stdout, stderr bytes.Buffer
			args := append(append([]string{"watch"}, tc.args...), "--project-root", e3PolicyRoot(t, tc.policy))
			code := runCICmd(args, nil, &stdout, &stderr)
			if code != tc.wantCode {
				t.Fatalf("exit = %d, want %d; stdout %q stderr %q", code, tc.wantCode, stdout.String(), stderr.String())
			}
			if !strings.Contains(stdout.String(), tc.wantStdout) || !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Errorf("stdout %q stderr %q, want %q and %q", stdout.String(), stderr.String(), tc.wantStdout, tc.wantStderr)
			}
		})
	}
}

func TestRunCI_WatchWithoutAWorkingDirectoryIsAnIOError(t *testing.T) {
	installFakeGH(t, watchGreen)
	breakWorkingDirectory(t)
	var stdout, stderr bytes.Buffer
	if code := runCICmd([]string{"watch", "--sha", "abcdef1"}, nil, &stdout, &stderr); code != exitIO {
		t.Fatalf("exit = %d, want %d; stderr %q", code, exitIO, stderr.String())
	}
}

func TestClassifyRedRuns_AClassifyFailureIsNamedPerRun(t *testing.T) {
	installFakeGH(t, watchGreen)
	var out, errb bytes.Buffer
	classifyRedRuns(context.Background(), e3PolicyRoot(t, ""), []redWorkflow{{workflow: "required.yml", runID: 99}}, &out, &errb)
	if !strings.Contains(errb.String(), "classify required.yml run 99") {
		t.Errorf("stderr %q does not name the workflow and run that failed to classify", errb.String())
	}
}

func TestUnwiredDeps_FallBackToSafeDefaults(t *testing.T) {
	prev := current.Load()
	current.Store(nil)
	t.Cleanup(func() { current.Store(prev) })

	if _, err := resolveRoot("", nil); err == nil {
		t.Error("resolveRoot must refuse when its dependency is not wired")
	}
	var stdout, stderr bytes.Buffer
	if code := printVerbJSON("pr", map[string]int{"a": 1}, &stdout, &stderr); code != exitIO || !strings.Contains(stderr.String(), "PrintVerbJSON dependency is not wired") {
		t.Errorf("printVerbJSON = %d, stderr %q", code, stderr.String())
	}
	center, closeCenter := newSignalCenter("ci", "", "", &stderr)
	if center != nil || closeCenter == nil {
		t.Fatalf("newSignalCenter = (%v, %v), want a nil center and a callable close", center, closeCenter != nil)
	}
	closeCenter()
	root := t.TempDir()
	if got := refreshWorktree(root); got.Base != "HEAD" || got.Run == nil || got.Git == nil {
		t.Errorf("refreshWorktree = base %q, run set %v, git set %v; want the core default", got.Base, got.Run != nil, got.Git != nil)
	}
	if host().ResolveRoot != nil {
		t.Error("an unset host must carry no wired dependency")
	}
}

func TestWiredDeps_ReachTheHostFunctions(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := printVerbJSON("pr", map[string]int{"a": 1}, &stdout, &stderr); code != exitOK || !strings.Contains(stdout.String(), `"a":1`) {
		t.Errorf("printVerbJSON = %d, stdout %q", code, stdout.String())
	}
	center, closeCenter := newSignalCenter("ci", "", "", &stderr)
	if center == nil {
		t.Error("a wired NewSignalCenter must return its center")
	}
	closeCenter()
}
