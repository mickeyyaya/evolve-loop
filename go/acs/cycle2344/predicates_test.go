//go:build acs

package cycle2344

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	shipPkg      = "./internal/phases/ship"
	packageNote  = "docs/architecture/packages/internal-phases-ship.md"
	integrationT = "go/internal/phases/ship/repocontract_cmd_unix_test.go"
	noteHeading  = "### Process-group cancel test"
)

func goTestInShip(t *testing.T, root string, args ...string) string {
	t.Helper()
	argv := append([]string{"-C", filepath.Join(root, "go"), "test", "-count=1"}, args...)
	argv = append(argv, shipPkg)
	out, stderr, code, err := acsassert.SubprocessOutput("go", argv...)
	combined := out + stderr
	if err != nil || code != 0 {
		t.Fatalf("go test %v exit=%d err=%v\n%s", args, code, err, acsassert.OutputTail(combined, 25))
	}
	return combined
}

func noteSection(t *testing.T, root string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, packageNote))
	if err != nil {
		t.Fatalf("read %s: %v", packageNote, err)
	}
	text := string(raw)
	start := strings.Index(text, noteHeading)
	if start < 0 {
		t.Fatalf("%s has no %q section", packageNote, noteHeading)
	}
	body := text[start+len(noteHeading):]
	if next := regexp.MustCompile(`(?m)^#{1,3} `).FindStringIndex(body); next != nil {
		body = body[:next[0]]
	}
	return body
}

func TestC2344_001_FailureMessageNamesExitStatusSignalAndElapsed(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out := goTestInShip(t, root, "-v", "-run", "^TestDescribeWaitResult_")
	for _, name := range []string{
		"--- PASS: TestDescribeWaitResult_NamesExitStatusSignalAndElapsed",
		"--- PASS: TestDescribeWaitResult_DistinguishesACleanExitFromAKill",
	} {
		if !strings.Contains(out, name) {
			t.Errorf("fake-state test did not run and pass: missing %q\n%s", name, acsassert.OutputTail(out, 25))
		}
	}
	if !acsassert.FileContains(t, filepath.Join(root, integrationT), "describeWaitResult(") {
		t.Errorf("the process-group cancel test never builds its failure message with describeWaitResult")
	}
}

func TestC2344_002_NoteRecordsTheRaceCount200MacOSRun(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	section := noteSection(t, root)
	run := regexp.MustCompile(`(?m)^- Run:.*$`).FindString(section)
	if run == "" {
		t.Fatalf("section %q has no '- Run:' bullet", noteHeading)
	}
	for _, token := range []string{"-race", "-count=200", "macOS", "parallel"} {
		if !strings.Contains(run, token) {
			t.Errorf("Run bullet %q lacks %q", run, token)
		}
	}
	result := regexp.MustCompile(`(?m)^- Result:.*$`).FindString(section)
	if !regexp.MustCompile(`\b\d+/200\b`).MatchString(result) {
		t.Errorf("Result bullet %q does not state the outcome as N/200", result)
	}
}

func TestC2344_003_CancelStillKillsTheWholeProcessGroup(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out := goTestInShip(t, root, "-v", "-tags", "integration", "-run", "^TestCancelKillsTheProcessGroup_")
	for _, name := range []string{
		"--- PASS: TestCancelKillsTheProcessGroup_CancelReapsTheGrandchildToo",
		"--- PASS: TestCancelKillsTheProcessGroup_PutsTheChildInItsOwnGroup",
	} {
		if !strings.Contains(out, name) {
			t.Errorf("missing %q\n%s", name, acsassert.OutputTail(out, 25))
		}
	}
	if !acsassert.FileContains(t, filepath.Join(root, "go/internal/phases/ship/repocontract_cmd_unix.go"), "syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)") {
		t.Errorf("the group kill in cancelKillsTheProcessGroup was changed")
	}
}

func TestC2344_004_NoteRecordsTheCauseOfTheFlake(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	section := noteSection(t, root)
	cause := regexp.MustCompile(`(?m)^- Cause:\s*(.*)$`).FindStringSubmatch(section)
	if cause == nil {
		t.Fatalf("section %q has no '- Cause:' bullet", noteHeading)
	}
	if len(strings.Fields(cause[1])) < 8 {
		t.Errorf("Cause bullet %q is too short to record a cause", cause[1])
	}
	if !strings.Contains(strings.ToLower(section), "process group") {
		t.Errorf("section does not state that cancel kills the whole process group")
	}
}
