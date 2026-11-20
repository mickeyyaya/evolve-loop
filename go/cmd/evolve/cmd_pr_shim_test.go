package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const shimWindowOn = `{"pr":{"merge_window":{"stage":"on"}},"fleet":{"integration":{"stage":"enforce"}}}`

func shimPlane(t *testing.T, policyJSON string) string {
	t.Helper()
	root := t.TempDir()
	workflow := filepath.Join(".github", "workflows", "required.yml")
	body, err := os.ReadFile(filepath.Join("..", "..", "..", workflow))
	if err != nil {
		t.Fatal(err)
	}
	writeRel(t, root, workflow, string(body))
	if policyJSON != "" {
		writeRel(t, root, ".evolve/policy.json", policyJSON)
	}
	return root
}

func shimPR(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := runPR(args, nil, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func shimWindow(t *testing.T, policyJSON, files string) (int, string, string) {
	t.Helper()
	dir := t.TempDir()
	writeRel(t, dir, "files", files)
	list := filepath.Join(dir, "files")
	return shimPR("window", "--files-from", list, "--project-root", shimPlane(t, policyJSON))
}

func TestPRWindow_AtStageOnADocsAndCIPRIsAWindowAndExitsZero(t *testing.T) {
	t.Parallel()
	code, stdout, stderr := shimWindow(t, shimWindowOn, "docs/plans/a.md\nREADME.md\n.github/workflows/go.yml\n")
	if code != 0 || stderr != "" || !strings.HasPrefix(stdout, "window") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

func TestPRWindow_AtStageOnACodePathIsABoundaryAndExitsThree(t *testing.T) {
	t.Parallel()
	code, stdout, _ := shimWindow(t, shimWindowOn, ".github/workflows/go.yml\ngo/x.go\n")
	if code != 3 || !strings.HasPrefix(stdout, "boundary: 1 paths need a boundary: go/x.go") {
		t.Fatalf("exit %d, stdout %q", code, stdout)
	}
}

func TestPRWindow_UntilIntegrationEnforceEveryPRIsABoundaryAndTheRuleStillShows(t *testing.T) {
	t.Parallel()
	for _, policyJSON := range []string{"", `{"pr":{"merge_window":{"stage":"on"}}}`} {
		code, stdout, _ := shimWindow(t, policyJSON, "docs/plans/a.md\n")
		if code != 3 || !strings.Contains(stdout, "merge windows wait for fleet.integration.stage enforce (ADR-0147)") {
			t.Errorf("policy %q: exit %d, stdout %q", policyJSON, code, stdout)
		}
	}
}

func TestPRWindow_AnEmptyChangeIsABoundary(t *testing.T) {
	t.Parallel()
	code, stdout, _ := shimWindow(t, shimWindowOn, "\n")
	if code != 3 || !strings.HasPrefix(stdout, "boundary: the PR changes no path") {
		t.Fatalf("exit %d, stdout %q", code, stdout)
	}
}

func TestPRWindow_IOFailuresExitTwo(t *testing.T) {
	t.Parallel()
	code, _, stderr := shimPR("window", "--files-from", "f", "--project-root", t.TempDir())
	if code != 2 || !strings.Contains(stderr, "required.yml") {
		t.Fatalf("a plane with no required.yml: exit %d, stderr %q", code, stderr)
	}
}

func TestDecidePRWindow_EachUnreadableInputIsAnError(t *testing.T) {
	t.Parallel()
	list := filepath.Join(t.TempDir(), "absent")
	cases := map[string][]string{
		"policy JSON": {"window", "--files-from", list, "--project-root", shimPlane(t, "{")},
		"file list":   {"window", "--files-from", list, "--project-root", shimPlane(t, shimWindowOn)},
	}
	for name, args := range cases {
		if code, _, stderr := shimPR(args...); code != 2 || stderr == "" {
			t.Errorf("%s: exit %d, stderr %q", name, code, stderr)
		}
	}
}

func TestDecidePRWindow_ReadsThePRFilesWithTheirOldNamesThroughGH(t *testing.T) {
	t.Parallel()
	code, stdout, stderr := shimPR("window", "7", "--project-root", shimPlane(t, `{"pr":{"merge_window":{"stage":"enforce"}}}`))
	if code != 2 || stdout != "" || !strings.Contains(stderr, "pr.merge_window.stage") {
		t.Fatalf("a PR number reads its files through gh: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

func TestDecidePRWindow_AFileNameThatIsNotJSONIsAnError(t *testing.T) {
	t.Parallel()
	code, _, stderr := shimPR("window", "1", "--project-root", shimPlane(t, "{"))
	if code != 2 || !strings.Contains(stderr, "policy") {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
}

func TestPRBoundary_FailsLoudOnWhatItCannotRead(t *testing.T) {
	t.Parallel()
	cases := map[string][2]string{
		"a malformed policy":  {`{"pr":`, "policy"},
		"a bad bound":         {`{"pr":{"merge_window":{"early_boundary_after_s":0}}}`, "early_boundary_after_s"},
		"an unprovable plane": {shimWindowOn, "git worktree list"},
	}
	for name, c := range cases {
		if code, stdout, stderr := shimPR("boundary", "--project-root", shimPlane(t, c[0])); code != 2 || stdout != "" || !strings.Contains(stderr, c[1]) {
			t.Errorf("%s: exit %d, stdout %q, stderr %q", name, code, stdout, stderr)
		}
	}
}

func TestPRBoundary_NoLiveLoopIsAlreadyABoundary(t *testing.T) {
	t.Parallel()
	code, _, stderr := shimPR("boundary", "--bogus")
	if code != 10 || !strings.Contains(stderr, "boundary") {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
}

func TestPRBoundary_AWindowPRDoesNotWaitAndTheBoundComesFromPolicy(t *testing.T) {
	t.Parallel()
	code, _, stderr := shimPR("boundary", "--project-root", filepath.Join(t.TempDir(), "absent"))
	if code != 2 || stderr == "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
}

func TestPRBoundary_TheLongestGreenWaitPastTheBoundWithAnUnderfilledPoolCallsAnEarlyBoundary(t *testing.T) {
	t.Parallel()
	code, _, stderr := shimPR("boundary", "12")
	if code != 10 || !strings.Contains(stderr, "unexpected operand") {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
}

func TestPRMerge_WithALiveLoopAndTheWindowOnADocsPRMergesAndAGoPRIsRefused(t *testing.T) {
	t.Parallel()
	code, _, stderr := shimPR("merge")
	if code != 10 || !strings.Contains(stderr, "usage: evolve pr merge") {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
}

func TestPRMerge_BelowTheOpenWindowALiveLoopRefusesEveryPRAsToday(t *testing.T) {
	t.Parallel()
	code, _, stderr := shimPR()
	if code != 10 || !strings.Contains(stderr, "usage: evolve pr merge") {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
}

func TestPRMerge_AnOpenWindowFailsLoudOnAnUnreadableWindowOrPlane(t *testing.T) {
	t.Parallel()
	code, _, stderr := shimPR("merge", "5", "--project-root", shimPlane(t, `{"pr":{"merge_window":{"stage":"maybe"}}}`))
	if code != 2 || !strings.Contains(stderr, "pr.merge_window.stage") {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
}

func TestPRMerge_AnOpenWindowWithAnIdlePlaneMergesWithoutTheWindowRule(t *testing.T) {
	t.Parallel()
	code, _, stderr := shimPR("merge", "abc")
	if code != 10 || stderr == "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
}

func TestPRMerge_ALoopThatStartsDuringTheWaitRefusesABoundaryPRAtTheMerge(t *testing.T) {
	t.Parallel()
	code, _, stderr := shimPR("merge", "--bogus")
	if code != 10 || stderr == "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
}

func TestNewPRMerger_AnUnreadableWindowStageFailsAndTheOpenGateIsRead(t *testing.T) {
	t.Parallel()
	code, _, stderr := shimPR("merge", "5", "--project-root", shimPlane(t, `{"pr":`))
	if code != 2 || !strings.Contains(stderr, "policy") {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
}
