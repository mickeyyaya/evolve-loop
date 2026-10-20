package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/mergewindow"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func windowPlane(t *testing.T, policyJSON string) string {
	t.Helper()
	root := t.TempDir()
	workflow, err := os.ReadFile(filepath.Join("..", "..", "..", requiredWorkflowPath))
	if err != nil {
		t.Fatal(err)
	}
	writeWindowFile(t, filepath.Join(root, requiredWorkflowPath), string(workflow))
	if policyJSON != "" {
		writeWindowFile(t, filepath.Join(root, ".evolve", "policy.json"), policyJSON)
	}
	return root
}

func writeWindowFile(t *testing.T, path, body string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const windowOn = `{"pr":{"merge_window":{"stage":"on"}},"fleet":{"integration":{"stage":"enforce"}}}`

func runWindow(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runPR(append([]string{"window"}, args...), nil, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestPRWindow_AtStageOnADocsAndCIPRIsAWindowAndExitsZero(t *testing.T) {
	t.Parallel()
	root := windowPlane(t, windowOn)
	list := writeWindowFile(t, filepath.Join(t.TempDir(), "files"), "docs/plans/a.md\nREADME.md\n\n.github/workflows/go.yml\n")
	code, stdout, stderr := runWindow(t, "--files-from", list, "--json", "--project-root", root)
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	var got prWindowDecision
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode %q: %v", stdout, err)
	}
	want := prWindowDecision{Stage: "on", Integration: "enforce", Verdict: mergewindow.Window, Rule: mergewindow.Window, Reason: reasonWindow, BoundaryPaths: []string{}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decision = %+v, want %+v", got, want)
	}
	if !strings.Contains(stdout, `"boundary_paths":[]`) || strings.Contains(stdout, `"pr"`) {
		t.Errorf("the JSON must list an empty boundary_paths and omit pr for a file list: %s", stdout)
	}
}

func TestPRWindow_AtStageOnACodePathIsABoundaryAndExitsThree(t *testing.T) {
	t.Parallel()
	root := windowPlane(t, windowOn)
	list := writeWindowFile(t, filepath.Join(t.TempDir(), "files"), "docs/plans/a.md\r\ngo/x.go\n.evolve/policy.json\ndocs/a.md .github/x\n")
	code, stdout, _ := runWindow(t, "--files-from", list, "--project-root", root)
	want := "boundary: 4 paths need a boundary: docs/plans/a.md, go/x.go, .evolve/policy.json, docs/a.md .github/x (stage on, integration enforce, paths boundary)\n"
	if code != exitBoundary || stdout != want {
		t.Fatalf("exit %d, stdout %q; want %d, %q", code, stdout, exitBoundary, want)
	}
}

func TestPRWindow_UntilIntegrationEnforceEveryPRIsABoundaryAndTheRuleStillShows(t *testing.T) {
	t.Parallel()
	if reasonIntegration != "merge windows wait for fleet.integration.stage enforce (ADR-0147)" {
		t.Fatalf("the reason must name the integration stage of ADR-0147: %q", reasonIntegration)
	}
	cases := []struct{ policyJSON, want string }{
		{"", "boundary: " + reasonIntegration + " (stage off, integration shadow, paths window)\n"},
		{`{"pr":{"merge_window":{"stage":"on"}}}`, "boundary: " + reasonIntegration + " (stage on, integration shadow, paths window)\n"},
		{`{"pr":{"merge_window":{"stage":"on"}},"fleet":{"integration":{"stage":"shadow"}}}`, "boundary: " + reasonIntegration + " (stage on, integration shadow, paths window)\n"},
		{`{"fleet":{"integration":{"stage":"enforce"}}}`, "boundary: " + reasonWindowStageOff + " (stage off, integration enforce, paths window)\n"},
	}
	for _, c := range cases {
		root := windowPlane(t, c.policyJSON)
		list := writeWindowFile(t, filepath.Join(t.TempDir(), "files"), "docs/plans/a.md\n")
		code, stdout, _ := runWindow(t, "--files-from", list, "--project-root", root)
		if code != exitBoundary || stdout != c.want {
			t.Errorf("policy %q: exit %d, stdout %q; want %d, %q", c.policyJSON, code, stdout, exitBoundary, c.want)
		}
	}
}

func TestWindowDecision_AListAtTheGitHubFileLimitIsABoundary(t *testing.T) {
	t.Parallel()
	open := windowGate{stage: policy.MergeWindowOn, integration: policy.FleetIntegrationEnforce}
	d := windowDecision("7", open, mergewindow.Verdict{Class: mergewindow.Window}, ghFileLimit)
	if d.Verdict != mergewindow.Boundary || d.Rule != mergewindow.Boundary || !strings.Contains(d.Reason, "3000 files") {
		t.Fatalf("decision = %+v, want a boundary that names the limit", d)
	}
	if d := windowDecision("7", open, mergewindow.Verdict{Class: mergewindow.Window}, ghFileLimit-1); d.Verdict != mergewindow.Window {
		t.Fatalf("below the limit: %+v, want window", d)
	}
}

func TestPRWindow_AnEmptyChangeIsABoundary(t *testing.T) {
	t.Parallel()
	root := windowPlane(t, windowOn)
	list := writeWindowFile(t, filepath.Join(t.TempDir(), "files"), "\n")
	code, stdout, _ := runWindow(t, "--files-from", list, "--project-root", root)
	if code != exitBoundary || !strings.HasPrefix(stdout, "boundary: "+reasonNoPaths) {
		t.Fatalf("exit %d, stdout %q", code, stdout)
	}
}

func TestDecidePRWindow_ReadsThePRFilesWithTheirOldNamesThroughGH(t *testing.T) {
	t.Parallel()
	root := windowPlane(t, windowOn)
	var calls []string
	gh := func(_ context.Context, args ...string) ([]byte, error) {
		calls = append(calls, strings.Join(args, " "))
		return []byte("\"docs/moved.md\"\n\"go/old.go\"\n"), nil
	}
	d, err := decidePRWindow(context.Background(), root, prWindowArgs{pr: "1009"}, gh)
	if err != nil {
		t.Fatal(err)
	}
	if d.PR != "1009" || d.Verdict != mergewindow.Boundary || !reflect.DeepEqual(d.BoundaryPaths, []string{"docs/moved.md", "go/old.go"}) {
		t.Fatalf("decision = %+v", d)
	}
	want := "api --paginate repos/{owner}/{repo}/pulls/1009/files --jq .[] | .filename, (.previous_filename // empty) | @json"
	if len(calls) != 1 || calls[0] != want {
		t.Fatalf("gh calls = %q, want [%q]", calls, want)
	}
}

func TestDecidePRWindow_EachUnreadableInputIsAnError(t *testing.T) {
	t.Parallel()
	ghDown := func(context.Context, ...string) ([]byte, error) { return nil, errors.New("gh: HTTP 502") }
	missing := filepath.Join(t.TempDir(), "absent")
	cases := map[string]struct {
		root func(t *testing.T) string
		args prWindowArgs
		want string
	}{
		"policy JSON": {func(t *testing.T) string { return windowPlane(t, "{") }, prWindowArgs{pr: "1"}, "policy"},
		"stage word":  {func(t *testing.T) string { return windowPlane(t, `{"pr":{"merge_window":{"stage":"enforce"}}}`) }, prWindowArgs{pr: "1"}, "pr.merge_window.stage"},
		"no workflow": {func(t *testing.T) string { return t.TempDir() }, prWindowArgs{pr: "1"}, "required.yml"},
		"bad workflow": {func(t *testing.T) string {
			r := t.TempDir()
			writeWindowFile(t, filepath.Join(r, requiredWorkflowPath), "jobs: {}\n")
			return r
		}, prWindowArgs{pr: "1"}, "no job"},
		"no file list":     {func(t *testing.T) string { return windowPlane(t, windowOn) }, prWindowArgs{filesFrom: missing}, "absent"},
		"integration word": {func(t *testing.T) string { return windowPlane(t, `{"fleet":{"integration":{"stage":"on"}}}`) }, prWindowArgs{pr: "1"}, "fleet.integration.stage"},
		"gh fails":         {func(t *testing.T) string { return windowPlane(t, windowOn) }, prWindowArgs{pr: "1"}, "HTTP 502"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := decidePRWindow(context.Background(), c.root(t), c.args, ghDown)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("decidePRWindow = %v, want an error with %q", err, c.want)
			}
		})
	}
}

func TestPRWindow_IOFailuresExitTwo(t *testing.T) {
	t.Parallel()
	code, _, stderr := runWindow(t, "--files-from", "f", "--project-root", t.TempDir())
	if code != exitIO || !strings.Contains(stderr, prWindowPrefix) {
		t.Fatalf("a plane with no required.yml: exit %d, stderr %q", code, stderr)
	}
	root := windowPlane(t, windowOn)
	list := writeWindowFile(t, filepath.Join(t.TempDir(), "files"), "docs/a.md")
	var stderrBuf bytes.Buffer
	if code := runPRWindow([]string{"--files-from", list, "--project-root", root}, &failingWriter{}, &stderrBuf); code != exitIO || !strings.Contains(stderrBuf.String(), "broken pipe") {
		t.Fatalf("a broken stdout: exit %d, stderr %q", code, stderrBuf.String())
	}
}

func TestPRWindow_AnUnreadableWorkingDirectoryExitsTwo(t *testing.T) {
	breakWorkingDirectory(t)
	code, _, stderr := runWindow(t, "--files-from", "f")
	if code != exitIO || !strings.Contains(stderr, "getwd") {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
}

func TestPRWindow_UsageErrorsExitTen(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{nil, {"1", "2"}, {"1", "--files-from", "f"}, {"abc"}, {"0"}, {"--bogus"}, {"--files-from"}} {
		code, stdout, stderr := runWindow(t, args...)
		if code != exitUsage || stdout != "" || !strings.Contains(stderr, prWindowUsage) {
			t.Errorf("window %q: exit %d, stdout %q, stderr %q", args, code, stdout, stderr)
		}
	}
	var stderr bytes.Buffer
	if code := runPR(nil, nil, &bytes.Buffer{}, &stderr); code != exitUsage || !strings.Contains(stderr.String(), prWindowUsage) {
		t.Errorf("bare pr must name both sub-verbs: %q", stderr.String())
	}
}

func TestParsePRWindowArgs_TakesOnePROrAFileList(t *testing.T) {
	t.Parallel()
	a, err := parsePRWindowArgs([]string{"--json", "1009", "--project-root=/p"})
	if err != nil || a != (prWindowArgs{pr: "1009", projectRoot: "/p", asJSON: true}) {
		t.Fatalf("parsePRWindowArgs = %+v, %v", a, err)
	}
	a, err = parsePRWindowArgs([]string{"--files-from", "list"})
	if err != nil || a != (prWindowArgs{filesFrom: "list"}) {
		t.Fatalf("parsePRWindowArgs = %+v, %v", a, err)
	}
}

func TestDecidePRWindow_AFileNameThatIsNotJSONIsAnError(t *testing.T) {
	t.Parallel()
	gh := func(context.Context, ...string) ([]byte, error) { return []byte("go/x.go\n"), nil }
	if _, err := decidePRWindow(context.Background(), windowPlane(t, windowOn), prWindowArgs{pr: "1"}, gh); err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("decidePRWindow = %v, want a decode error", err)
	}
}
