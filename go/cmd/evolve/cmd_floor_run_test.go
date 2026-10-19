package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeFloorExec(results map[string]struct {
	out  string
	code int
	err  error
}) floorExecFunc {
	return func(_, command string) (string, int, error) {
		r := results[command]
		return r.out, r.code, r.err
	}
}

type floorResult = struct {
	out  string
	code int
	err  error
}

func floorRootWithLintPin(t *testing.T, version string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "jobs:\n  lint:\n    steps:\n      - uses: actions/setup-go@v5\n        with:\n          go-version: '" + version + "'\n"
	if err := os.WriteFile(filepath.Join(dir, "go.yml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestFloorRun_VerdictsPerTierAndExitCode(t *testing.T) {
	t.Parallel()
	root := floorRootWithLintPin(t, "1.23")
	cases := []struct {
		name    string
		results map[string]floorResult
		want    int
		lines   []string
	}{
		{"all green", nil, 0, []string{"PASS vet", "PASS ratchet"}},
		{"red then green", map[string]floorResult{"go vet ./...": {out: "a.go:1: bad\n", code: 1}}, 1, []string{"RED vet", "a.go:1: bad", "PASS ratchet"}},
		{"cannot run", map[string]floorResult{"go vet ./...": {code: 127}}, 2, []string{"ERROR vet", "PASS ratchet"}},
		{"red beats green", map[string]floorResult{"GOOS=darwin go vet ./...": {code: 1}}, 1, []string{"RED vet"}},
		{"error then red keeps error", map[string]floorResult{"go vet ./...": {code: 127}, "GOOS=darwin go vet ./...": {code: 1}}, 2, []string{"ERROR vet"}},
		{"start failure beats red", map[string]floorResult{"go vet ./...": {code: 1}, "GOOS=darwin go vet ./...": {err: errors.New("no sh")}}, 2, []string{"ERROR vet", "no sh"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var out, errOut bytes.Buffer
			code := runFloorWith(fakeFloorExec(tc.results), []string{"run", "--tier", "vet,ratchet", "--root", root}, &out, &errOut)
			if code != tc.want {
				t.Errorf("exit=%d, want %d\n%s", code, tc.want, out.String())
			}
			for _, line := range tc.lines {
				if !strings.Contains(out.String(), line) {
					t.Errorf("stdout lacks %q:\n%s", line, out.String())
				}
			}
		})
	}
}

func TestFloorRun_FmtTierErrorsWithoutAPinnedToolchain(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	code := runFloorWith(fakeFloorExec(nil), []string{"run", "--tier", "fmt,ratchet", "--root", t.TempDir()}, &out, &errOut)
	if code != 2 || !strings.Contains(out.String(), "ERROR fmt") || !strings.Contains(out.String(), "PASS ratchet") {
		t.Errorf("exit=%d\n%s", code, out.String())
	}
	var plan, planErr bytes.Buffer
	if code := runFloorWith(fakeFloorExec(nil), []string{"run", "--dry-run", "--tier", "fmt", "--root", t.TempDir()}, &plan, &planErr); code != 2 || !strings.Contains(planErr.String(), "tier fmt") {
		t.Errorf("dry-run exit=%d stderr=%s", code, planErr.String())
	}
}

func TestFloorRun_LintGoVersionRefusals(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cases := map[string]string{
		"not yaml":   "jobs: [\n",
		"no version": "jobs:\n  lint:\n    steps:\n      - run: go vet ./...\n",
	}
	for name, body := range cases {
		path := filepath.Join(dir, strings.ReplaceAll(name, " ", "-")+".yml")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if v, err := floorLintGoVersion(path); err == nil {
			t.Errorf("%s: version %q, want an error", name, v)
		}
	}
}

func TestFloorRun_FmtCommandNormalizesAMinorOnlyPin(t *testing.T) {
	t.Parallel()
	cmds, err := floorFmtCommands(floorRootWithLintPin(t, "1.23"))
	if err != nil || len(cmds) != 1 || !strings.Contains(cmds[0], "GOTOOLCHAIN=go1.23.0;") {
		t.Errorf("cmds=%q err=%v", cmds, err)
	}
	cmds, err = floorFmtCommands(floorRootWithLintPin(t, "1.23.4"))
	if err != nil || !strings.Contains(cmds[0], "GOTOOLCHAIN=go1.23.4;") {
		t.Errorf("cmds=%q err=%v", cmds, err)
	}
}

func TestFloorRun_RefusesExtraArgumentsAndBadFlags(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"run", "extra"}, {"run", "--nope"}, {"run", "--tier", ""}} {
		var out, errOut bytes.Buffer
		code := runFloorWith(fakeFloorExec(nil), append(args, "--dry-run", "--root", t.TempDir()), &out, &errOut)
		wantTwo := len(args) < 3 || args[1] != "--tier"
		if wantTwo && code != 2 {
			t.Errorf("%v exit=%d, want 2", args, code)
		}
	}
}

func TestFloorRun_TierSelectionDedupesAndTrims(t *testing.T) {
	t.Parallel()
	got, err := selectFloorTiers("vet, vet,ratchet")
	if err != nil || len(got) != 2 || got[0].name != "vet" || got[1].name != "ratchet" {
		t.Errorf("got %v err=%v", got, err)
	}
	if _, err := selectFloorTiers("vet,"); err == nil {
		t.Error("an empty tier name must be refused")
	}
}

func TestFloorRun_TailKeepsTheLastLines(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	for i := 0; i < floorTailLines+5; i++ {
		b.WriteString("line\n")
	}
	b.WriteString("last\n")
	got := floorTail(b.String())
	if len(got) != floorTailLines || got[len(got)-1] != "last" {
		t.Errorf("tail len=%d last=%q", len(got), got[len(got)-1])
	}
	if floorTail("") != nil {
		t.Error("empty output must have no tail")
	}
}

func TestFloorRun_ExecFloorCommandReportsExitCodesAndStartFailures(t *testing.T) {
	t.Parallel()
	out, code, err := execFloorCommand(t.TempDir(), "echo hi; exit 3")
	if err != nil || code != 3 || !strings.Contains(out, "hi") {
		t.Errorf("out=%q code=%d err=%v", out, code, err)
	}
	if _, _, err := execFloorCommand(filepath.Join(t.TempDir(), "missing"), "true"); err == nil {
		t.Error("a missing working directory must be a start failure")
	}
	if code := runFloor([]string{"run", "--dry-run", "--tier", "vet", "--root", t.TempDir()}, nil, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Errorf("runFloor dry-run exit=%d", code)
	}
}
