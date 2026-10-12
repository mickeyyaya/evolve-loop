//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciwatch"
)

const (
	ciFakeHead = "0123456789abcdef0123456789abcdef01234567"
	ciFakeBase = "fedcba9876543210fedcba9876543210fedcba98"
)

const ciFakeGHScript = `#!/bin/sh
D='%DIR%'
printf '%s\n' "$*" >> "$D/calls.log"
reply() { if [ -f "$D/$1" ]; then cat "$D/$1"; exit 0; fi; echo "fake gh: no $1" >&2; exit 1; }
case "$*" in
  "api repos/{owner}/{repo}/commits/"*) reply commit.json ;;
  "api repos/{owner}/{repo}/compare/"*) reply compare.json ;;
  "pr view "*"headRefOid,baseRefName") reply pr-full.json ;;
  "pr view "*) reply pr.json ;;
  "run list --workflow "*" status,conclusion,url,databaseId") reply "watch-$4.json" ;;
  "run list "*) reply runs.json ;;
  "run view "*"--log-failed"*) reply log.txt ;;
  "run view "*) reply run.json ;;
esac
echo "fake gh: unexpected $*" >&2
exit 2
`

type ciFakeGH struct {
	t   *testing.T
	dir string
}

func newCIFakeGH(t *testing.T, replies map[string]string) ciFakeGH {
	t.Helper()
	dir := t.TempDir()
	for name, body := range replies {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	bin := t.TempDir()
	script := strings.ReplaceAll(ciFakeGHScript, "%DIR%", dir)
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return ciFakeGH{t: t, dir: dir}
}

func (g ciFakeGH) calls() string {
	g.t.Helper()
	raw, _ := os.ReadFile(filepath.Join(g.dir, "calls.log"))
	return string(raw)
}

func ciProjectRoot(t *testing.T, policyBody string, withGoMod bool) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if policyBody != "" {
		if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(policyBody), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if withGoMod {
		if err := os.MkdirAll(filepath.Join(root, "go"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "go", "go.mod"), []byte("module example.com/fixture\n\ngo 1.23\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func ciRunDoc(id int, status, conclusion string) string {
	n := strconv.Itoa(id)
	return `{"attempt":1,"conclusion":"` + conclusion + `","createdAt":"2026-10-01T00:00:00Z","databaseId":` + n +
		`,"headSha":"` + ciFakeHead + `","status":"` + status + `","url":"https://ci/runs/` + n + `"}`
}

var ciCompareDoc = `{"status":"ahead","merge_base_commit":{"sha":"` + ciFakeBase + `"},"files":[{"filename":"go/internal/x/x.go"}]}`

const ciFailedLog = "test (ubuntu)\tRun tests\t--- FAIL: TestBroken (0.01s)\ntest (ubuntu)\tRun tests\tFAIL\tgithub.com/x/y/internal/x\t0.1s\n"

func runCIIn(args ...string) (string, string, int) {
	var out, errb bytes.Buffer
	code := runCI(args, nil, &out, &errb)
	return out.String(), errb.String(), code
}

func TestRunCIClassify_AGreenRunIsRetrySafe(t *testing.T) {
	gh := newCIFakeGH(t, map[string]string{"run.json": ciRunDoc(501, "completed", "success"), "compare.json": ciCompareDoc})
	root := ciProjectRoot(t, "", true)

	out, errOut, code := runCIIn("classify", "run:501", "--project-root", root)

	if code != 0 || errOut != "" {
		t.Fatalf("code=%d stderr=%q, want 0 and silence", code, errOut)
	}
	if !strings.HasPrefix(out, "run 501 https://ci/runs/501 head=") || !strings.HasSuffix(out, "retry-safe: yes\n") {
		t.Errorf("stdout = %q, want the run header and retry-safe: yes", out)
	}
	if !strings.Contains(gh.calls(), "run view 501 --json attempt,conclusion,createdAt,databaseId,headSha,status,url") {
		t.Errorf("gh calls = %q, want the run view", gh.calls())
	}
}

func TestRunCIClassify_ARedRunIsRefusedAndAMissingGoModWarns(t *testing.T) {
	newCIFakeGH(t, map[string]string{"run.json": ciRunDoc(502, "completed", "failure"), "compare.json": ciCompareDoc, "log.txt": ciFailedLog, "runs.json": "[]"})
	root := ciProjectRoot(t, `{"ci_watch":{"timeout_s":5,"poll_s":1}}`, false)

	out, errOut, code := runCIIn("classify", "run:502", "--json", "--project-root", root)

	if code != exitRefused {
		t.Fatalf("code=%d stderr=%q, want %d", code, errOut, exitRefused)
	}
	if !strings.Contains(errOut, ciClassifyPrefix+"WARN: open "+filepath.Join(root, "go", "go.mod")) {
		t.Errorf("stderr = %q, want the go.mod WARN", errOut)
	}
	var rep ciwatch.ClassifyReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("stdout %q: %v", out, err)
	}
	if rep.RunID != 502 || rep.RetrySafe || len(rep.Failures) != 1 || rep.Failures[0].Test != "TestBroken" {
		t.Errorf("report = %+v, want one TestBroken failure that is not retry-safe", rep)
	}
}

func TestRunCIClassify_RefusalsAndIOErrors(t *testing.T) {
	cases := []struct {
		name    string
		replies map[string]string
		policy  string
		target  string
		code    int
		want    string
	}{
		{"no run on the commit", map[string]string{"commit.json": `{"sha":"` + ciFakeHead + `"}`, "runs.json": "[]", "compare.json": ciCompareDoc}, "",
			"sha:" + ciFakeHead[:7], exitRefused, ciClassifyPrefix + "ciwatch: no CI run"},
		{"the run is still in progress", map[string]string{"run.json": ciRunDoc(503, "in_progress", ""), "compare.json": ciCompareDoc}, "",
			"run:503", exitRefused, "run 503 is in_progress"},
		{"gh fails", nil, "", "run:504", exitIO, ciClassifyPrefix + "gh run view 504"},
		{"a malformed policy", nil, `{"ci_watch":`, "run:504", exitIO, ciClassifyPrefix + "policy: parse "},
		{"an invalid ci_watch block", nil, `{"ci_watch":{"timeout_s":0}}`, "run:504", exitIO, ciClassifyPrefix + "policy: ci_watch.timeout_s must be > 0, got 0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			newCIFakeGH(t, tc.replies)
			out, errOut, code := runCIIn("classify", tc.target, "--project-root", ciProjectRoot(t, tc.policy, true))
			if code != tc.code || out != "" || !strings.Contains(errOut, tc.want) {
				t.Errorf("code=%d stdout=%q stderr=%q, want %d and %q", code, out, errOut, tc.code, tc.want)
			}
		})
	}
}

func TestRunCIClassify_AnUnwritableStdoutIsAnIOError(t *testing.T) {
	newCIFakeGH(t, map[string]string{"run.json": ciRunDoc(501, "completed", "success"), "compare.json": ciCompareDoc})
	var errb bytes.Buffer

	code := runCI([]string{"classify", "run:501", "--project-root", ciProjectRoot(t, "", true)}, nil, &failingWriter{}, &errb)

	if code != exitIO || errb.String() != ciClassifyPrefix+"broken pipe\n" {
		t.Errorf("code=%d stderr=%q, want exitIO and the write error", code, errb.String())
	}
}

func TestRunCI_AnUnreadableWorkingDirectoryIsAnIOErrorForBothVerbs(t *testing.T) {
	breakWorkingDirectory(t)
	for _, args := range [][]string{{"classify", "run:1"}, {"watch", "--sha", ciFakeHead}} {
		out, errOut, code := runCIIn(args...)
		if code != exitIO || out != "" || !strings.Contains(errOut, "getwd: no such file or directory") {
			t.Errorf("%v: code=%d stdout=%q stderr=%q, want exitIO and the getwd error", args, code, out, errOut)
		}
	}
}

func watchReply(conclusion string, id int) string {
	return `[{"status":"completed","conclusion":"` + conclusion + `","url":"https://ci/runs/` + strconv.Itoa(id) + `","databaseId":` + strconv.Itoa(id) + `}]`
}

func TestRunCIWatch_AGreenSHAExits0(t *testing.T) {
	gh := newCIFakeGH(t, map[string]string{"commit.json": `{"sha":"` + ciFakeHead + `"}`, "watch-required.yml.json": watchReply("success", 9)})
	root := ciProjectRoot(t, "", true)

	out, errOut, code := runCIIn("watch", "--sha", ciFakeHead[:7], "--project-root", root)

	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	if out != "required.yml\tsuccess\t"+shortSHA(ciFakeHead)+"\thttps://ci/runs/9\n" {
		t.Errorf("stdout = %q", out)
	}
	if !strings.Contains(gh.calls(), "api repos/{owner}/{repo}/commits/"+ciFakeHead[:7]) {
		t.Errorf("calls = %q, want the SHA resolved through the commits API", gh.calls())
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "inbox")); !os.IsNotExist(err) {
		t.Errorf("a green watch filed an inbox item (stat err=%v)", err)
	}
}

func TestRunCIWatch_ARedTagWatchesBothWorkflowsAndClassifiesTheRedRun(t *testing.T) {
	gh := newCIFakeGH(t, map[string]string{
		"commit.json":             `{"sha":"` + ciFakeHead + `"}`,
		"watch-required.yml.json": watchReply("success", 9),
		"watch-release.yml.json":  watchReply("failure", 77),
		"log.txt":                 ciFailedLog,
		"run.json":                ciRunDoc(77, "completed", "failure"),
		"compare.json":            ciCompareDoc,
		"runs.json":               "[]",
	})
	root := ciProjectRoot(t, `{"ci_watch":{"timeout_s":5,"poll_s":1}}`, true)

	out, errOut, code := runCIIn("watch", "--tag", "v1.2.3", "--project-root", root)

	if code != exitRefused {
		t.Fatalf("code=%d stdout=%q stderr=%q, want %d", code, out, errOut, exitRefused)
	}
	if !strings.Contains(out, "required.yml\tsuccess\t") || !strings.Contains(out, "release.yml\tfailure\t") {
		t.Errorf("stdout = %q, want one line per watched workflow", out)
	}
	if !strings.Contains(out, "run 77 https://ci/runs/77") || !strings.Contains(out, "retry-safe: no") {
		t.Errorf("stdout = %q, want the red run's classification", out)
	}
	if !strings.Contains(gh.calls(), "commits/v1.2.3") {
		t.Errorf("calls = %q, want the tag resolved", gh.calls())
	}
	if entries, _ := os.ReadDir(filepath.Join(root, ".evolve", "inbox")); len(entries) == 0 {
		t.Error("a red tag watch files a fix-forward inbox item")
	}
}

func TestRunCIWatch_ARedPRFilesNoItemAndAFailedClassifyIsReported(t *testing.T) {
	newCIFakeGH(t, map[string]string{
		"pr.json":                 `{"headRefOid":"` + ciFakeHead + `"}`,
		"watch-required.yml.json": watchReply("failure", 78),
	})
	root := ciProjectRoot(t, "", true)

	out, errOut, code := runCIIn("watch", "--pr", "12", "--project-root", root)

	if code != exitRefused {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	if !strings.HasPrefix(out, "required.yml\tfailure\t") {
		t.Errorf("stdout = %q", out)
	}
	if !strings.Contains(errOut, ciWatchPrefix+"classify required.yml run 78: gh run view 78") {
		t.Errorf("stderr = %q, want the classify failure named with its run", errOut)
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "inbox")); !os.IsNotExist(err) {
		t.Errorf("a PR watch filed an inbox item (stat err=%v)", err)
	}
}

func TestRunCIWatch_SetupAndFetchFailuresAreIOErrors(t *testing.T) {
	cases := []struct {
		name    string
		replies map[string]string
		policy  string
		args    []string
		want    string
	}{
		{"a malformed policy", nil, `{"ci_watch":`, []string{"--sha", ciFakeHead}, ciWatchPrefix + "policy: parse "},
		{"an invalid ci_watch block", nil, `{"ci_watch":{"poll_s":-1}}`, []string{"--sha", ciFakeHead}, ciWatchPrefix + "policy: ci_watch.poll_s must be > 0, got -1"},
		{"an unresolvable PR", map[string]string{"pr.json": `{"headRefOid":"nope"}`}, "", []string{"--pr", "3"}, ciWatchPrefix + `ciwatch: PR 3 resolved to "nope", not a commit SHA`},
		{"the run list fails", map[string]string{"commit.json": `{"sha":"` + ciFakeHead + `"}`}, "", []string{"--sha", ciFakeHead, "--workflow", "lint.yml"}, ciWatchPrefix + "lint.yml: ciwatch: fetch run status for "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			newCIFakeGH(t, tc.replies)
			out, errOut, code := runCIIn(append(append([]string{"watch"}, tc.args...), "--project-root", ciProjectRoot(t, tc.policy, true))...)
			if code != exitIO || out != "" || !strings.Contains(errOut, tc.want) {
				t.Errorf("code=%d stdout=%q stderr=%q, want exitIO and %q", code, out, errOut, tc.want)
			}
		})
	}
}

func TestClassifyRedRuns_AnUnloadablePolicyClassifiesNothing(t *testing.T) {
	gh := newCIFakeGH(t, nil)
	root := ciProjectRoot(t, `{"ci_watch":`, true)
	var out, errb bytes.Buffer

	classifyRedRuns(context.Background(), root, []redWorkflow{{workflow: "required.yml", runID: 5}}, &out, &errb)

	if out.Len() != 0 || !strings.HasPrefix(errb.String(), ciWatchPrefix+"classify: policy: parse ") {
		t.Errorf("stdout=%q stderr=%q, want only the classify policy error", out.String(), errb.String())
	}
	if gh.calls() != "" {
		t.Errorf("gh calls = %q, want none", gh.calls())
	}
}
