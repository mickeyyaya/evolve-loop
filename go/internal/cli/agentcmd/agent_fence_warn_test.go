package agentcmd

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

var otherExitCodes = []int{0, exitFail, exitRefused, exitDispatch, exitUsage, exitInput}

func warnLines(stderr string) []string {
	var out []string
	for _, line := range strings.Split(stderr, "\n") {
		if strings.Contains(line, "WARN") {
			out = append(out, line)
		}
	}
	return out
}

func warnLineNaming(stderr string, needles ...string) bool {
	return slices.ContainsFunc(warnLines(stderr), func(line string) bool {
		for _, n := range needles {
			if !strings.Contains(line, n) {
				return false
			}
		}
		return true
	})
}

func runPlain(t *testing.T, d Deps, args []string, stdin string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := New(d)(args, strings.NewReader(stdin), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func strayWrites() map[string]string {
	return map[string]string{"go/x.go": "package x // seat edit\n", "go/stray_test.go": "package x // stray\n"}
}

func TestAgentRun_ASeatThatCreatesAFileOutsideItsGrantPrintsOneWarnLineWithThePathAndTheNextStep(t *testing.T) {
	for _, asJSON := range []bool{true, false} {
		name := "plain report"
		if asJSON {
			name = "json envelope"
		}
		t.Run(name, func(t *testing.T) {
			root := project(t)
			wt := seatWorktree(t)
			fb := &fakeBridge{artifact: builderReport, onLaunch: seatWrites(t, strayWrites())}
			args := []string{"run", "console-builder", "--project-root", root, "--worktree", wt, "--writable", "go/x.go"}
			if asJSON {
				args = append(args, "--json")
			}

			_, _, stderr := runPlain(t, depsWith(fb), args, "build item X")

			if len(fb.requests) != 1 {
				t.Fatalf("launches %d stderr=%s, want 1", len(fb.requests), stderr)
			}
			if !warnLineNaming(stderr, "go/stray_test.go", "--writable") {
				t.Errorf("stderr has no WARN line that names go/stray_test.go and the --writable next step:\n%s", stderr)
			}
		})
	}
}

func TestAgentRun_ASeatThatCreatesAFileOutsideItsGrantExitsWithACodeNoOtherOutcomeUses(t *testing.T) {
	for _, asJSON := range []bool{true, false} {
		name := "plain report"
		if asJSON {
			name = "json envelope"
		}
		t.Run(name, func(t *testing.T) {
			root := project(t)
			wt := seatWorktree(t)
			fb := &fakeBridge{artifact: builderReport, onLaunch: seatWrites(t, strayWrites())}
			args := []string{"run", "console-builder", "--project-root", root, "--worktree", wt, "--writable", "go/x.go"}
			if asJSON {
				args = append(args, "--json")
			}

			code, _, stderr := runPlain(t, depsWith(fb), args, "build item X")

			if slices.Contains(otherExitCodes, code) {
				t.Errorf("exit %d stderr=%s: a fence revert must exit with a code that is none of %v", code, stderr, otherExitCodes)
			}
		})
	}
}

func TestAgentRun_TheFenceWarnLineNamesEveryRevertedPathOfTheRun(t *testing.T) {
	root := project(t)
	wt := seatWorktree(t)
	fb := &fakeBridge{artifact: builderReport, onLaunch: seatWrites(t, map[string]string{
		"go/x.go":          "package x // seat edit\n",
		"go/stray_test.go": "package x // stray\n",
		"go/other.go":      "package x // seat tampered\n",
	})}

	_, _, stderr := runJSON(t, depsWith(fb), builderArgs(root, "--worktree", wt, "--writable", "go/x.go"), "build item X")

	if !warnLineNaming(stderr, "go/stray_test.go", "go/other.go") {
		t.Errorf("no single WARN line names both the removed go/stray_test.go and the restored go/other.go:\n%s", stderr)
	}
}

func TestAgentRun_ASeatThatTampersWithAnExistingFileOutsideItsGrantGetsAWarnLineThatNamesTheRestoredPath(t *testing.T) {
	root := project(t)
	wt := seatWorktree(t)
	fb := &fakeBridge{artifact: builderReport, onLaunch: seatWrites(t, map[string]string{
		"go/x.go":     "package x // seat edit\n",
		"go/other.go": "package x // seat tampered\n",
	})}

	_, _, stderr := runJSON(t, depsWith(fb), builderArgs(root, "--worktree", wt, "--writable", "go/x.go"), "build item X")

	if got, _ := readSeatFile(wt, "go/other.go"); got != seatBase["go/other.go"] {
		t.Errorf("the fence did not restore go/other.go: %q", got)
	}
	if !warnLineNaming(stderr, "go/other.go") {
		t.Errorf("stderr has no WARN line that names the restored go/other.go:\n%s", stderr)
	}
}

func TestAgentRun_ASeatThatStaysInsideItsGrantPrintsNoWarnLineAndExitsZero(t *testing.T) {
	root := project(t)
	wt := seatWorktree(t)
	fb := &fakeBridge{artifact: builderReport, onLaunch: seatWrites(t, map[string]string{
		"go/x.go":       "package x // seat edit\n",
		"go/gen/new.go": "package gen // new\n",
	})}

	code, env, stderr := runJSON(t, depsWith(fb), builderArgs(root, "--worktree", wt, "--writable", "go/x.go", "--writable-dir", "go/gen"), "build item X")

	if code != 0 || env.Verdict != core.VerdictPASS {
		t.Errorf("exit %d verdict %s stderr=%s, want 0 and PASS", code, env.Verdict, stderr)
	}
	if lines := warnLines(stderr); len(lines) != 0 {
		t.Errorf("a seat inside its grant must print no WARN line: %v", lines)
	}
}

func TestAgentRun_TheConsoleRerunsASeatWithTheCorrectedGrantAndTheRunPassesWithoutAWarnLine(t *testing.T) {
	root := project(t)
	wt := seatWorktree(t)
	fb := &fakeBridge{artifact: builderReport, onLaunch: seatWrites(t, strayWrites())}
	deps := depsWith(fb)

	firstCode, _, firstStderr := runJSON(t, deps, builderArgs(root, "--worktree", wt, "--writable", "go/x.go"), "build item X")
	secondCode, env, secondStderr := runJSON(t, deps, builderArgs(root, "--worktree", wt, "--writable", "go/x.go", "--writable", "go/stray_test.go"), "build item X")

	if slices.Contains(otherExitCodes, firstCode) || !warnLineNaming(firstStderr, "go/stray_test.go") {
		t.Fatalf("first run: exit %d stderr=%s, want the fence-revert code and a WARN line that names go/stray_test.go", firstCode, firstStderr)
	}
	if secondCode != 0 || env.Verdict != core.VerdictPASS || len(warnLines(secondStderr)) != 0 {
		t.Errorf("re-run with the corrected grant: exit %d verdict %s stderr=%s, want 0, PASS and no WARN line", secondCode, env.Verdict, secondStderr)
	}
	if got, kept := readSeatFile(wt, "go/stray_test.go"); !kept || got != "package x // stray\n" {
		t.Errorf("the granted file did not survive the re-run: kept=%v content=%q", kept, got)
	}
}

func TestAgentRun_TheUsageDocumentsTheFenceRevertExitCode(t *testing.T) {
	code, stdout, _ := runPlain(t, depsWith(&fakeBridge{}), []string{"help"}, "")

	documented := false
	for _, line := range strings.Split(stdout, "\n") {
		documented = documented || strings.Contains(line, "Exit 0") && strings.Contains(line, "fence")
	}
	if code != 0 || !documented {
		t.Errorf("exit %d, the Exit line of the usage names the fence-revert code: %v", code, documented)
	}
}
