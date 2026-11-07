//go:build acs

package cycle2215

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	marginHeading = "## Margin against the go test timeout"
	evolvePkg     = "github.com/mickeyyaya/evolve-loop/go/cmd/slowpkg"
	otherPkg      = "github.com/mickeyyaya/evolve-loop/go/internal/other"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func buildTestlatency(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "testlatency")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-o", bin, "./cmd/testlatency")
	build.Dir = goDir(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build testlatency: %v\n%s", err, out)
	}
	return bin
}

func runTestlatency(t *testing.T, bin, stdin string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = goDir(t)
	cmd.Stdin = strings.NewReader(stdin)
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	code = 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("run testlatency: %v", err)
		}
		code = ee.ExitCode()
	}
	return so.String(), se.String(), code
}

func streamWithWall(evolveWall string) string {
	return `{"Action":"run","Package":"` + evolvePkg + `","Test":"TestSlow"}
{"Action":"pass","Package":"` + evolvePkg + `","Test":"TestSlow","Elapsed":5}
{"Action":"pass","Package":"` + evolvePkg + `","Elapsed":` + evolveWall + `}
{"Action":"run","Package":"` + otherPkg + `","Test":"TestFast"}
{"Action":"pass","Package":"` + otherPkg + `","Test":"TestFast","Elapsed":1}
{"Action":"pass","Package":"` + otherPkg + `","Elapsed":2}
`
}

func marginSection(t *testing.T, report string) string {
	t.Helper()
	start := strings.Index(report, marginHeading)
	if start < 0 {
		t.Fatalf("report has no %q section:\n%s", marginHeading, report)
	}
	rest := report[start+len(marginHeading):]
	if end := strings.Index(rest, "\n## "); end >= 0 {
		rest = rest[:end]
	}
	return rest
}

func rowFor(t *testing.T, section, pkg string) string {
	t.Helper()
	for _, line := range strings.Split(section, "\n") {
		if strings.Contains(line, pkg) {
			return line
		}
	}
	t.Fatalf("margin section has no row for %s:\n%s", pkg, section)
	return ""
}

func TestC2215_001_SlowPackageMarginIsReportedAndFlaggedLow(t *testing.T) {
	bin := buildTestlatency(t)
	stdout, stderr, code := runTestlatency(t, bin, streamWithWall("1500"), "-timeout-margin", "30m")
	if code != 0 {
		t.Fatalf("a low margin is a report, not a failure: exit %d, stderr %q", code, stderr)
	}
	row := rowFor(t, marginSection(t, stdout), "cmd/slowpkg")
	for _, want := range []string{"1500.00", "300.00", "16.7", "LOW"} {
		if !strings.Contains(row, want) {
			t.Errorf("cmd/slowpkg row %q lacks %q (wall 1500s of a 1800s timeout leaves 300s, 16.7%%, under the 20%% floor)", row, want)
		}
	}
}

func TestC2215_002_HealthyMarginIsReportedWithoutLowFlag(t *testing.T) {
	bin := buildTestlatency(t)
	stdout, _, code := runTestlatency(t, bin, streamWithWall("600"), "-timeout-margin", "30m")
	if code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	row := rowFor(t, marginSection(t, stdout), "cmd/slowpkg")
	for _, want := range []string{"600.00", "1200.00", "66.7"} {
		if !strings.Contains(row, want) {
			t.Errorf("cmd/slowpkg row %q lacks %q", row, want)
		}
	}
	if strings.Contains(row, "LOW") {
		t.Errorf("a 66.7%% margin is flagged LOW: %q", row)
	}
}

func TestC2215_003_MarginFloorIsExactlyTwentyPercent(t *testing.T) {
	bin := buildTestlatency(t)
	atFloor, _, _ := runTestlatency(t, bin, streamWithWall("1440"), "-timeout-margin", "30m")
	if row := rowFor(t, marginSection(t, atFloor), "cmd/slowpkg"); strings.Contains(row, "LOW") {
		t.Errorf("a margin of exactly 20%% is flagged LOW: %q", row)
	}
	belowFloor, _, _ := runTestlatency(t, bin, streamWithWall("1441"), "-timeout-margin", "30m")
	if row := rowFor(t, marginSection(t, belowFloor), "cmd/slowpkg"); !strings.Contains(row, "LOW") {
		t.Errorf("a margin just under 20%% is not flagged LOW: %q", row)
	}
}

func TestC2215_004_PackageWithoutTerminalSummaryIsNamedInMarginSection(t *testing.T) {
	bin := buildTestlatency(t)
	killedByTimeout := `{"Action":"run","Package":"` + evolvePkg + `","Test":"TestHang"}
{"Action":"pass","Package":"` + otherPkg + `","Elapsed":2}
`
	stdout, _, code := runTestlatency(t, bin, killedByTimeout, "-timeout-margin", "30m")
	if code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	row := rowFor(t, marginSection(t, stdout), "cmd/slowpkg")
	if !strings.Contains(row, "no terminal summary") {
		t.Errorf("the package that never finished has no explanation in its margin row: %q", row)
	}
}

func TestC2215_005_EmptyStreamSaysSoInsteadOfInventingARow(t *testing.T) {
	bin := buildTestlatency(t)
	stdout, _, code := runTestlatency(t, bin, "", "-timeout-margin", "30m")
	if code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	section := marginSection(t, stdout)
	if strings.Contains(section, "cmd/slowpkg") || strings.Contains(section, "LOW") {
		t.Errorf("an empty stream produced a margin row:\n%s", section)
	}
	if !strings.Contains(section, "_(none)_") {
		t.Errorf("an empty stream must show the _(none)_ placeholder readers scan for:\n%s", section)
	}
}

func TestC2215_006_NoFlagLeavesTheReportUnchangedAndBadValueExitsTwo(t *testing.T) {
	bin := buildTestlatency(t)
	plain, _, code := runTestlatency(t, bin, streamWithWall("1500"))
	if code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	if _, stderr, okCode := runTestlatency(t, bin, streamWithWall("1500"), "-timeout-margin", "30m"); okCode != 0 {
		t.Fatalf("-timeout-margin 30m exit %d, want 0 (a bad-value exit 2 means nothing while the flag is unknown): %q", okCode, stderr)
	}
	if strings.Contains(plain, marginHeading) {
		t.Errorf("the margin section appears without -timeout-margin:\n%s", plain)
	}
	for _, bad := range []string{"soon", "0s", "-5m"} {
		_, stderr, badCode := runTestlatency(t, bin, streamWithWall("1500"), "-timeout-margin", bad)
		if badCode != 2 {
			t.Errorf("-timeout-margin %q exit %d, want 2 (stderr %q)", bad, badCode, stderr)
		}
	}
}

func TestC2215_007_MakefilePrintsTheDarwinTimeoutItAppliesToGoTest(t *testing.T) {
	out, stderr, code, err := acsassert.SubprocessOutput("make", "-s", "-C", goDir(t), "print-test-timeout-darwin")
	if err != nil || code != 0 {
		t.Fatalf("make print-test-timeout-darwin exit %d: %v\n%s", code, err, stderr)
	}
	value := strings.TrimSpace(out)
	d, parseErr := time.ParseDuration(value)
	if parseErr != nil || d <= 0 {
		t.Fatalf("print-test-timeout-darwin printed %q, want one positive duration", out)
	}
	dry, _, dryCode, _ := acsassert.SubprocessOutput("make", "-n", "-C", goDir(t), "test-integration-darwin")
	if dryCode != 0 || !strings.Contains(dry, "-timeout "+value) {
		t.Errorf("test-integration-darwin does not run go test with -timeout %s (the printed number is not the applied one):\n%s", value, dry)
	}
}

var literalDuration = regexp.MustCompile(`-timeout-margin[ =]+['"]?[0-9]+[smh]`)

func TestC2215_008_DarwinJobPassesTheMakefileTimeoutToTheMarginReport(t *testing.T) {
	workflow := filepath.Join(acsassert.RepoRoot(t), ".github", "workflows", "go.yml")
	body := readFile(t, workflow)
	job := darwinJob(t, body)
	var step string
	for _, line := range strings.Split(job, "\n") {
		if strings.Contains(line, "cmd/testlatency") && strings.Contains(line, "-timeout-margin") {
			step = line
		}
	}
	if step == "" {
		t.Fatalf("no testlatency invocation with -timeout-margin in the build-test-darwin job")
	}
	if !strings.Contains(step, "print-test-timeout-darwin") {
		t.Errorf("the margin step does not read the timeout from the Makefile: %q", step)
	}
	if literalDuration.MatchString(step) {
		t.Errorf("the margin step hardcodes a duration, a second home for the number: %q", step)
	}
	if !strings.Contains(step, "GITHUB_STEP_SUMMARY") {
		t.Errorf("the margin is not written to the job summary: %q", step)
	}
}

func TestC2215_009_PackageNoteDocumentsTheMarginFlag(t *testing.T) {
	note := filepath.Join(acsassert.RepoRoot(t), "docs", "architecture", "packages", "cmd-testlatency.md")
	if !acsassert.FileContains(t, note, "-timeout-margin") {
		t.Errorf("%s does not document -timeout-margin", note)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	out, _, code, err := acsassert.SubprocessOutput("cat", path)
	if err != nil || code != 0 {
		t.Fatalf("read %s: %v", path, err)
	}
	return out
}

func darwinJob(t *testing.T, workflow string) string {
	t.Helper()
	const header = "  build-test-darwin:"
	start := strings.Index(workflow, header)
	if start < 0 {
		t.Fatalf("go.yml has no %s job", strings.TrimSpace(header))
	}
	rest := workflow[start+len(header):]
	next := regexp.MustCompile(`\n  [a-z0-9-]+:\n`).FindStringIndex(rest)
	if next != nil {
		rest = rest[:next[0]]
	}
	return rest
}
