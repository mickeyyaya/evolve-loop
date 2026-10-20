package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
)

const (
	createdPullRequestURL  = "https://github.com/acme/widgets/pull/4217"
	existingPullRequestURL = "https://github.com/acme/widgets/pull/7731"
	existingPullRequest    = `[{"number":7731,"url":"` + existingPullRequestURL + `"}]`
	commitSubject          = "feat: add the widget"
)

type fakeGh struct {
	logPath string
}

func installFakeGh(t *testing.T, list, createURL string, createFails bool) fakeGh {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "gh-args.log")
	script := `#!/bin/sh
for a in "$@"; do printf '%s\n' "$a" >> "$FAKE_GH_LOG"; done
printf -- '--\n' >> "$FAKE_GH_LOG"
case "$1 $2" in
"pr list") printf '%s\n' "$FAKE_GH_LIST" ;;
"pr create")
  if [ -n "$FAKE_GH_CREATE_FAIL" ]; then echo "gh: HTTP 502 from the API" >&2; exit 1; fi
  printf '%s\n' "$FAKE_GH_CREATE_URL" ;;
*) echo "fake gh: unexpected call: $*" >&2; exit 99 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_GH_LOG", logPath)
	t.Setenv("FAKE_GH_LIST", list)
	t.Setenv("FAKE_GH_CREATE_URL", createURL)
	if createFails {
		t.Setenv("FAKE_GH_CREATE_FAIL", "1")
	} else {
		t.Setenv("FAKE_GH_CREATE_FAIL", "")
	}
	return fakeGh{logPath: logPath}
}

func (g fakeGh) calls(t *testing.T) [][]string {
	t.Helper()
	raw, err := os.ReadFile(g.logPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("read fake gh log: %v", err)
	}
	var calls [][]string
	var current []string
	for _, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		if line == "--" {
			calls = append(calls, current)
			current = nil
			continue
		}
		current = append(current, line)
	}
	return calls
}

func (g fakeGh) createCalls(t *testing.T) [][]string {
	t.Helper()
	var creates [][]string
	for _, call := range g.calls(t) {
		if len(call) >= 2 && call[0] == "pr" && call[1] == "create" {
			creates = append(creates, call)
		}
	}
	return creates
}

func argAfter(call []string, flag string) (string, bool) {
	i := slices.Index(call, flag)
	if i < 0 || i+1 >= len(call) {
		return "", false
	}
	return call[i+1], true
}

func repoOnBranch(t *testing.T, branch string) string {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Git("commit", "--allow-empty", "-m", "chore: seed")
	if branch != "main" {
		repo.Git("checkout", "-b", branch)
	}
	return repo.Dir
}

func runShipPr(t *testing.T, root string, extra ...string) (rc int, stdout, stderr string) {
	t.Helper()
	args := append([]string{"--class", "manual", "--pr", "--project-root", root}, extra...)
	args = append(args, commitSubject)
	var out, errBuf bytes.Buffer
	rc = runShipCmd(args, nil, &out, &errBuf)
	return rc, out.String(), errBuf.String()
}

func TestRunShipCmd_Pr_CreatesThePullRequestAndPrintsItsNumber(t *testing.T) {
	gh := installFakeGh(t, "[]", createdPullRequestURL, false)
	calls := stubShipRun(t, ship.RunResult{ExitCode: ship.ExitOK}, nil)
	root := repoOnBranch(t, "feature/widget")

	rc, stdout, stderr := runShipPr(t, root)

	if rc != 0 {
		t.Fatalf("rc = %d, want 0; stderr = %q", rc, stderr)
	}
	if len(*calls) != 1 || (*calls)[0].Class != ship.ClassManual {
		t.Fatalf("ship runs = %+v, want exactly one manual-class run", *calls)
	}
	if !strings.Contains(stdout, "4217") {
		t.Errorf("stdout = %q, want the new PR number 4217", stdout)
	}
	creates := gh.createCalls(t)
	if len(creates) != 1 {
		t.Fatalf("gh pr create ran %d times, want 1; calls = %v", len(creates), gh.calls(t))
	}
	if base, ok := argAfter(creates[0], "--base"); !ok || base != "main" {
		t.Errorf("gh pr create --base = %q (present %v), want main", base, ok)
	}
	if title, ok := argAfter(creates[0], "--title"); !ok || title != commitSubject {
		t.Errorf("gh pr create --title = %q (present %v), want %q", title, ok, commitSubject)
	}
	listed := false
	for _, call := range gh.calls(t) {
		if len(call) >= 2 && call[0] == "pr" && call[1] == "list" {
			listed = slices.Contains(call, "feature/widget")
		}
	}
	if !listed {
		t.Errorf("the open-PR lookup never named the branch feature/widget; calls = %v", gh.calls(t))
	}
}

func TestRunShipCmd_Pr_ReportsAnOpenPullRequestInsteadOfCreatingASecond(t *testing.T) {
	gh := installFakeGh(t, existingPullRequest, createdPullRequestURL, false)
	stubShipRun(t, ship.RunResult{ExitCode: ship.ExitOK}, nil)
	root := repoOnBranch(t, "feature/widget")

	rc, stdout, stderr := runShipPr(t, root)

	if rc != 0 {
		t.Fatalf("rc = %d, want 0; stderr = %q", rc, stderr)
	}
	if !strings.Contains(stdout, "7731") {
		t.Errorf("stdout = %q, want the existing PR number 7731", stdout)
	}
	if strings.Contains(stdout, "4217") {
		t.Errorf("stdout = %q names the would-be new PR 4217", stdout)
	}
	if creates := gh.createCalls(t); len(creates) != 0 {
		t.Errorf("gh pr create ran %d times for a branch with an open PR: %v", len(creates), creates)
	}
}

func TestRunShipCmd_Pr_OnMainExits10BeforeShipOrGhRun(t *testing.T) {
	gh := installFakeGh(t, "[]", createdPullRequestURL, false)
	calls := stubShipRun(t, ship.RunResult{ExitCode: ship.ExitOK}, nil)
	root := repoOnBranch(t, "main")

	rc, stdout, stderr := runShipPr(t, root)

	if rc != 10 {
		t.Errorf("rc = %d, want 10; stderr = %q", rc, stderr)
	}
	if len(*calls) != 0 {
		t.Errorf("ship ran %d times for --pr on main, want 0", len(*calls))
	}
	if gh.calls(t) != nil {
		t.Errorf("gh was called for --pr on main: %v", gh.calls(t))
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "main") {
		t.Errorf("stderr = %q, want a reason naming main", stderr)
	}
}

func TestRunShipCmd_Pr_AGhFailureExits2AfterShipKeptTheCommitPushed(t *testing.T) {
	installFakeGh(t, "[]", createdPullRequestURL, true)
	calls := stubShipRun(t, ship.RunResult{ExitCode: ship.ExitOK, Logs: []string{"ship: pushed feature/widget"}}, nil)
	root := repoOnBranch(t, "feature/widget")

	rc, stdout, stderr := runShipPr(t, root)

	if rc != 2 {
		t.Errorf("rc = %d, want 2; stderr = %q", rc, stderr)
	}
	if len(*calls) != 1 {
		t.Errorf("ship runs = %d, want 1 (the push is never undone)", len(*calls))
	}
	if !strings.Contains(stderr, "ship: pushed feature/widget") {
		t.Errorf("stderr = %q, want the push result kept", stderr)
	}
	if strings.Contains(stdout, "4217") {
		t.Errorf("stdout = %q reports a PR number that was never created", stdout)
	}
}

func TestRunShipCmd_Pr_ARefusedShipOpensNoPullRequest(t *testing.T) {
	gh := installFakeGh(t, "[]", createdPullRequestURL, false)
	stubShipRun(t, ship.RunResult{ExitCode: ship.ExitIntegrity}, nil)
	root := repoOnBranch(t, "feature/widget")

	rc, stdout, _ := runShipPr(t, root)

	if rc != int(ship.ExitIntegrity) {
		t.Errorf("rc = %d, want ship's own %d", rc, ship.ExitIntegrity)
	}
	if gh.calls(t) != nil {
		t.Errorf("gh was called after a refused ship: %v", gh.calls(t))
	}
	if strings.Contains(stdout, "4217") {
		t.Errorf("stdout = %q reports a PR for a refused ship", stdout)
	}
}

func TestRunShipCmd_Pr_ADryRunOpensNoPullRequest(t *testing.T) {
	gh := installFakeGh(t, "[]", createdPullRequestURL, false)
	stubShipRun(t, ship.RunResult{ExitCode: ship.ExitOK}, nil)
	root := repoOnBranch(t, "feature/widget")

	rc, _, stderr := runShipPr(t, root, "--dry-run")

	if rc != 0 {
		t.Errorf("rc = %d, want 0; stderr = %q", rc, stderr)
	}
	if gh.calls(t) != nil {
		t.Errorf("gh was called by a dry run: %v", gh.calls(t))
	}
}

func TestRunShipCmd_Pr_OnlyTheManualClassMayOpenAPullRequest(t *testing.T) {
	for _, class := range []string{"cycle", "release", "trivial"} {
		t.Run(class, func(t *testing.T) {
			gh := installFakeGh(t, "[]", createdPullRequestURL, false)
			calls := stubShipRun(t, ship.RunResult{ExitCode: ship.ExitOK}, nil)
			root := repoOnBranch(t, "feature/widget")
			var stderr bytes.Buffer

			rc := runShipCmd([]string{"--class", class, "--pr", "--project-root", root, commitSubject}, nil, new(bytes.Buffer), &stderr)

			if rc != 10 {
				t.Errorf("rc = %d, want 10; stderr = %q", rc, stderr.String())
			}
			if len(*calls) != 0 || gh.calls(t) != nil {
				t.Errorf("ship runs = %d, gh calls = %v, want neither", len(*calls), gh.calls(t))
			}
		})
	}
}

func TestRunShipCmd_Pr_BodyFileReachesGhAndRequiresPr(t *testing.T) {
	gh := installFakeGh(t, "[]", createdPullRequestURL, false)
	stubShipRun(t, ship.RunResult{ExitCode: ship.ExitOK}, nil)
	root := repoOnBranch(t, "feature/widget")
	body := filepath.Join(t.TempDir(), "body.md")
	if err := os.WriteFile(body, []byte("why this change\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rc, _, stderr := runShipPr(t, root, "--pr-body-file", body)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0; stderr = %q", rc, stderr)
	}
	creates := gh.createCalls(t)
	if len(creates) != 1 {
		t.Fatalf("gh pr create ran %d times, want 1", len(creates))
	}
	if got, ok := argAfter(creates[0], "--body-file"); !ok || got != body {
		t.Errorf("gh pr create --body-file = %q (present %v), want %q", got, ok, body)
	}

	var refused bytes.Buffer
	rc = runShipCmd([]string{"--class", "manual", "--pr-body-file", body, "--project-root", root, commitSubject}, nil, new(bytes.Buffer), &refused)
	if rc != 10 {
		t.Errorf("--pr-body-file without --pr: rc = %d, want 10; stderr = %q", rc, refused.String())
	}
}
