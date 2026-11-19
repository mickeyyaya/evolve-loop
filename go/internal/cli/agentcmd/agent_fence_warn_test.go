package agentcmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/launchrecord"
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

var warnPatchPathPattern = regexp.MustCompile(`reverted patch: ([^)\s]+)`)

func TestAgentRun_AFenceRevertedPatchSurvivesBesideTheLaunchRecordAndApplies(t *testing.T) {
	root := project(t)
	wt := seatWorktree(t)
	tampered := "package x // seat tampered\n"
	fb := &fakeBridge{artifact: builderReport, onLaunch: seatWrites(t, map[string]string{
		"go/x.go":     "package x // seat edit\n",
		"go/other.go": tampered,
	})}

	code, env, stderr := runJSON(t, depsWith(fb), builderArgs(root, "--worktree", wt, "--writable", "go/x.go"), "build item X")

	if code != exitFenceRevert {
		t.Fatalf("exit %d stderr=%s, want %d", code, stderr, exitFenceRevert)
	}
	match := warnPatchPathPattern.FindStringSubmatch(stderr)
	if match == nil {
		t.Fatalf("stderr WARN line names no reverted patch path:\n%s", stderr)
	}
	wantPath := filepath.Join(root, ".evolve", launchrecord.Subdir, env.RunLabel, fencePatchName)
	if match[1] != wantPath {
		t.Errorf("WARN patch path = %q, want %q", match[1], wantPath)
	}
	raw, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("kept patch is not readable after run: %v", err)
	}
	if !strings.Contains(string(raw), "go/other.go") {
		t.Errorf("kept patch does not reference go/other.go:\n%s", raw)
	}
	diagMatch := false
	for _, d := range env.Diagnostics {
		if strings.Contains(d.Message, wantPath) {
			diagMatch = true
		}
	}
	if !diagMatch {
		t.Errorf("envelope diagnostics do not name %q: %+v", wantPath, env.Diagnostics)
	}
	if out, err := exec.Command("git", "-C", wt, "apply", wantPath).CombinedOutput(); err != nil {
		t.Fatalf("git apply %s: %v: %s", wantPath, err, out)
	}
	if got, _ := readSeatFile(wt, "go/other.go"); got != tampered {
		t.Errorf("git apply did not restore go/other.go: got %q want %q", got, tampered)
	}
}

func TestKeepFencePatch_ABlockedDirectoryWarnsAndPreservesOriginalDiagnostic(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, ".evolve", "ephemeral", "agent-runs", "run-7")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(workspace, fencePatchName)
	if err := os.WriteFile(src, []byte("diff"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", launchrecord.Subdir), []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	origMsg := "worktree fence: read-only phase console-builder wrote 1 path(s) into its worktree; restored to the dispatched tree: go/other.go (reverted patch: " + src + ")"
	out := launchOutcome{
		writer: true,
		resp:   core.PhaseResponse{Diagnostics: []core.Diagnostic{{Severity: "warning", Message: origMsg}}},
	}
	var stderr bytes.Buffer

	got := keepFencePatch(standaloneRun{root: root, label: "run-7", workspace: workspace}, out, &stderr)

	if !strings.Contains(stderr.String(), "run-7") {
		t.Errorf("stderr = %q, want a warning naming run-7", stderr.String())
	}
	if got.fenceReverted() != origMsg {
		t.Errorf("diagnostic = %q, want original %q", got.fenceReverted(), origMsg)
	}
}

func TestKeepFencePatch_AnUnstatableSourceWarnsWithoutChangingDiagnostics(t *testing.T) {
	root := t.TempDir()
	notADir := filepath.Join(root, "file")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer

	got := keepFencePatch(standaloneRun{root: root, label: "run-8", workspace: notADir}, launchOutcome{}, &stderr)

	if !strings.Contains(stderr.String(), "run-8") {
		t.Errorf("stderr = %q, want a warning naming run-8", stderr.String())
	}
	if len(got.resp.Diagnostics) != 0 {
		t.Errorf("diagnostics = %+v, want none", got.resp.Diagnostics)
	}
}
