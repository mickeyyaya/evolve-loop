//go:build acs

package cycle1787

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const baseCommit = "608217455"

var (
	buildOnce sync.Once
	binDir    string
	binPath   string
	buildOut  string
	buildCode int
)

func TestMain(m *testing.M) {
	code := m.Run()
	if binDir != "" {
		os.RemoveAll(binDir)
	}
	os.Exit(code)
}

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func evolveBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		var err error
		if binDir, err = os.MkdirTemp("", "cycle1787-evolve-"); err != nil {
			buildCode, buildOut = -1, err.Error()
			return
		}
		binPath = filepath.Join(binDir, "evolve")
		out, errOut, code := run(goDir(t), binPath, "go", "build", "-o", binPath, "./cmd/evolve")
		buildCode, buildOut = code, out+errOut
	})
	if buildCode != 0 {
		t.Fatalf("go build ./cmd/evolve exited %d:\n%s", buildCode, buildOut)
	}
	return binPath
}

func run(dir, _ string, name string, args ...string) (string, string, int) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return stdout.String(), stderr.String(), 0
	case errors.As(err, &exitErr):
		return stdout.String(), stderr.String(), exitErr.ExitCode()
	}
	return stdout.String(), stderr.String() + err.Error(), -1
}

func evolve(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	return run(t.TempDir(), "", evolveBinary(t), args...)
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func benchStore(t *testing.T, entries map[string]time.Duration) string {
	t.Helper()
	root := t.TempDir()
	benches := map[string]any{}
	for family, untilFromNow := range entries {
		benches[family] = map[string]any{
			"family":        family,
			"reason":        "rate_limit",
			"benched_at":    time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
			"benched_until": time.Now().Add(untilFromNow).UTC().Format(time.RFC3339),
			"strikes":       1,
		}
	}
	body, err := json.Marshal(map[string]any{"schema_version": 1, "benches": benches})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, ".evolve", "cli-health.json"), string(body))
	return root
}

func storedFamilies(t *testing.T, root string) map[string]bool {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, ".evolve", "cli-health.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Benches map[string]json.RawMessage `json:"benches"`
	}
	if err := json.Unmarshal(body, &f); err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for family := range f.Benches {
		out[family] = true
	}
	return out
}

func TestC1787_001_ClihealthListJSONPrintsOnlyActiveBenches(t *testing.T) {
	root := benchStore(t, map[string]time.Duration{"codex": time.Hour, "agy": -time.Hour})
	out, errOut, code := evolve(t, "clihealth", "list", "--json", "--project-root", root)
	if code != 0 {
		t.Fatalf("clihealth list --json exit=%d stderr=%s", code, errOut)
	}
	var entries []struct {
		Family       string    `json:"family"`
		BenchedUntil time.Time `json:"benched_until"`
	}
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("stdout is not a JSON array of entries: %v\n%s", err, out)
	}
	if len(entries) != 1 || entries[0].Family != "codex" || entries[0].BenchedUntil.IsZero() {
		t.Errorf("want exactly the active codex bench with benched_until, got %+v", entries)
	}
}

func TestC1787_002_ClihealthListHumanNamesFamilyAndEmptyStoreExitsZero(t *testing.T) {
	root := benchStore(t, map[string]time.Duration{"codex": time.Hour})
	out, errOut, code := evolve(t, "clihealth", "list", "--project-root", root)
	if code != 0 || !strings.Contains(out, "codex") || !strings.Contains(out, "rate_limit") {
		t.Errorf("list must name the family and its reason: exit=%d out=%q stderr=%q", code, out, errOut)
	}
	emptyRoot := t.TempDir()
	if out, errOut, code := evolve(t, "clihealth", "list", "--json", "--project-root", emptyRoot); code != 0 || strings.TrimSpace(out) != "[]" {
		t.Errorf("no benches prints [] and exits 0: exit=%d out=%q stderr=%q", code, out, errOut)
	}
}

func TestC1787_003_ClihealthClearRemovesOnlyTheNamedBench(t *testing.T) {
	root := benchStore(t, map[string]time.Duration{"codex": time.Hour, "agy": time.Hour})
	_, errOut, code := evolve(t, "clihealth", "clear", "codex", "--project-root", root)
	if code != 0 {
		t.Fatalf("clear of a benched family exit=%d stderr=%s", code, errOut)
	}
	if got := storedFamilies(t, root); got["codex"] || !got["agy"] {
		t.Errorf("clear codex must leave agy benched, store has %v", got)
	}
}

func TestC1787_004_ClihealthClearUnbenchedFamilyExitsOneAndTouchesNothing(t *testing.T) {
	root := benchStore(t, map[string]time.Duration{"agy": time.Hour})
	_, errOut, code := evolve(t, "clihealth", "clear", "codex", "--project-root", root)
	if code != 1 || !strings.Contains(errOut, "codex") {
		t.Errorf("clear of an unbenched family exits 1 naming it: exit=%d stderr=%q", code, errOut)
	}
	if got := storedFamilies(t, root); !got["agy"] || len(got) != 1 {
		t.Errorf("store must be untouched, has %v", got)
	}
}

func TestC1787_005_ClihealthRejectsMissingFamilyAndUnknownVerb(t *testing.T) {
	root := benchStore(t, map[string]time.Duration{"codex": time.Hour})
	if _, errOut, code := evolve(t, "clihealth", "list", "--project-root", root); code != 0 {
		t.Fatalf("precondition: clihealth list must work: exit=%d stderr=%s", code, errOut)
	}
	for _, args := range [][]string{
		{"clihealth", "clear", "--project-root", root},
		{"clihealth", "frobnicate", "--project-root", root},
		{"clihealth"},
	} {
		if _, _, code := evolve(t, args...); code == 0 {
			t.Errorf("%v must exit non-zero", args)
		}
	}
	if got := storedFamilies(t, root); !got["codex"] {
		t.Errorf("a rejected invocation must not clear anything, store has %v", got)
	}
}

func ratchetFixture(t *testing.T, funcLines int, offenders, baseline string) string {
	t.Helper()
	root := t.TempDir()
	var src strings.Builder
	src.WriteString("package fixture\n\nfunc Big() int {\n\ttotal := 0\n")
	for i := 0; i < funcLines; i++ {
		fmt.Fprintf(&src, "\ttotal += %d\n", i)
	}
	src.WriteString("\treturn total\n}\n")
	writeFile(t, filepath.Join(root, "internal", "fixture", "big.go"), src.String())
	writeFile(t, filepath.Join(root, "internal", "fixture", "big_test.go"), "package fixture\n\nimport \"testing\"\n\nfunc TestBig(t *testing.T) { _ = Big() }\n")
	writeFile(t, filepath.Join(root, "internal", "sizeratchet", "offenders.json"), offenders)
	writeFile(t, filepath.Join(root, "internal", "rawgitratchet", "baseline.json"), baseline)
	return root
}

func TestC1787_006_RatchetCheckCleanFixtureExitsZero(t *testing.T) {
	root := ratchetFixture(t, 5, "{}", "{}")
	out, errOut, code := evolve(t, "ratchet", "check", "--root", root)
	if code != 0 {
		t.Errorf("clean fixture exit=%d out=%s stderr=%s", code, out, errOut)
	}
}

func TestC1787_007_RatchetCheckFunctionOverSizeLimitExitsOneNamingIt(t *testing.T) {
	root := ratchetFixture(t, 80, "{}", "{}")
	_, errOut, code := evolve(t, "ratchet", "check", "--root", root)
	if code != 1 || !strings.Contains(errOut, "fixture.Big") {
		t.Errorf("an unlisted 80-line function exits 1 naming internal/fixture.Big: exit=%d stderr=%s", code, errOut)
	}
}

func TestC1787_008_RatchetCheckFunctionPastItsAllowanceExitsOne(t *testing.T) {
	root := ratchetFixture(t, 80, `{"internal/fixture.Big": 60}`, "{}")
	_, errOut, code := evolve(t, "ratchet", "check", "--root", root)
	if code != 1 || !strings.Contains(errOut, "allowance") {
		t.Errorf("a function past its allowance exits 1: exit=%d stderr=%s", code, errOut)
	}
	within := ratchetFixture(t, 80, `{"internal/fixture.Big": 200}`, "{}")
	if _, errOut, code := evolve(t, "ratchet", "check", "--root", within); code != 0 {
		t.Errorf("a function within its allowance passes: exit=%d stderr=%s", code, errOut)
	}
}

func TestC1787_009_RatchetCheckRawGitInitOutsideBaselineExitsOne(t *testing.T) {
	root := ratchetFixture(t, 5, "{}", "{}")
	writeFile(t, filepath.Join(root, "internal", "fixture", "raw_test.go"),
		"package fixture\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\nfunc TestRaw(t *testing.T) {\n\texec.Command(\"git\", \"init\", t.TempDir()).Run()\n}\n")
	_, errOut, code := evolve(t, "ratchet", "check", "--root", root)
	if code != 1 || !strings.Contains(errOut, "raw_test.go") {
		t.Errorf("a raw git init outside the baseline exits 1 naming the file: exit=%d stderr=%s", code, errOut)
	}
}

func TestC1787_010_RatchetCheckMissingOffenderListFailsLoudly(t *testing.T) {
	root := ratchetFixture(t, 5, "{}", "{}")
	if _, errOut, code := evolve(t, "ratchet", "check", "--root", root); code != 0 {
		t.Fatalf("precondition: the intact fixture must pass: exit=%d stderr=%s", code, errOut)
	}
	if err := os.Remove(filepath.Join(root, "internal", "sizeratchet", "offenders.json")); err != nil {
		t.Fatal(err)
	}
	_, errOut, code := evolve(t, "ratchet", "check", "--root", root)
	if code == 0 || strings.TrimSpace(errOut) == "" {
		t.Errorf("an unreadable offender list must exit non-zero with a message: exit=%d stderr=%q", code, errOut)
	}
}

func TestC1787_011_RatchetCheckRealModuleIsClean(t *testing.T) {
	out, errOut, code := evolve(t, "ratchet", "check", "--root", goDir(t))
	if code != 0 {
		t.Errorf("the module must satisfy both ratchets through the command: exit=%d out=%s stderr=%s", code, out, errOut)
	}
}

func TestC1787_012_LaneLeavesOffenderListsUntouched(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, rel := range []string{"go/internal/sizeratchet/offenders.json", "go/internal/rawgitratchet/baseline.json"} {
		out, errOut, code := run(root, "", "git", "diff", "--name-only", baseCommit, "--", rel)
		if code != 0 {
			t.Fatalf("git diff %s exit=%d %s", rel, code, errOut)
		}
		if strings.TrimSpace(out) != "" {
			t.Errorf("%s was edited; the lane must not touch it", rel)
		}
	}
}

func TestC1787_013_GroupsAreRegisteredInTheDispatcherUsage(t *testing.T) {
	for _, group := range []string{"clihealth", "ratchet"} {
		out, errOut, code := evolve(t, "help")
		if code != 0 || !strings.Contains(out+errOut, group) {
			t.Errorf("evolve help must list %q: exit=%d", group, code)
		}
	}
}

func TestC1787_014_RuntimeReferenceDocumentsBothVerbs(t *testing.T) {
	// acs-predicate: config-check
	doc := filepath.Join(acsassert.RepoRoot(t), "docs", "operations", "runtime-reference.md")
	for _, want := range []string{"clihealth list", "clihealth clear", "ratchet check"} {
		if !acsassert.FileContains(t, doc, want) {
			t.Errorf("runtime-reference.md must document %q", want)
		}
	}
}

func rawGitFixture(t *testing.T) string {
	t.Helper()
	root := ratchetFixture(t, 5, "{}", "{}")
	writeFile(t, filepath.Join(root, "internal", "fixture", "raw_test.go"),
		"package fixture\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\nfunc TestRaw(t *testing.T) {\n\texec.Command(\"git\", \"init\", t.TempDir()).Run()\n}\n")
	return root
}

func TestC1787_015_RatchetCheckSizeSelectorHonorsRootAndNamesTheFunction(t *testing.T) {
	overAllowance := ratchetFixture(t, 80, `{"internal/fixture.Big": 60}`, "{}")
	out, errOut, code := evolve(t, "ratchet", "check", "size", "--root", overAllowance)
	if code != 1 || !strings.Contains(errOut, "fixture.Big") {
		t.Errorf("`ratchet check size --root <fixture over its allowance>` exits 1 naming it: exit=%d out=%q stderr=%q", code, out, errOut)
	}
	if strings.Contains(out, "clean") {
		t.Errorf("a violating fixture must never report clean: out=%q", out)
	}
}

func TestC1787_016_RatchetCheckSelectorScansOnlyTheNamedRatchet(t *testing.T) {
	rawOnly := rawGitFixture(t)
	if _, errOut, code := evolve(t, "ratchet", "check", "rawgit", "--root", rawOnly); code != 1 || !strings.Contains(errOut, "raw_test.go") {
		t.Errorf("`check rawgit` on a raw-git violation exits 1 naming the file: exit=%d stderr=%q", code, errOut)
	}
	if _, errOut, code := evolve(t, "ratchet", "check", "size", "--root", rawOnly); code != 0 {
		t.Errorf("`check size` must not run the raw-git ratchet: exit=%d stderr=%q", code, errOut)
	}
	sizeOnly := ratchetFixture(t, 80, "{}", "{}")
	if _, errOut, code := evolve(t, "ratchet", "check", "rawgit", "--root", sizeOnly); code != 0 {
		t.Errorf("`check rawgit` must not run the size ratchet: exit=%d stderr=%q", code, errOut)
	}
}

func TestC1787_017_RatchetCheckRejectsUnknownSelectorAndTrailingArgsWithExitTwo(t *testing.T) {
	clean := ratchetFixture(t, 5, "{}", "{}")
	if _, errOut, code := evolve(t, "ratchet", "check", "--root", clean); code != 0 {
		t.Fatalf("precondition: the clean fixture passes: exit=%d stderr=%q", code, errOut)
	}
	for _, args := range [][]string{
		{"ratchet", "check", "bogus", "--root", clean},
		{"ratchet", "check", "--root", clean, "junk"},
		{"ratchet", "check", "size", "--root", clean, "junk"},
	} {
		out, errOut, code := evolve(t, args...)
		if code != 2 || strings.TrimSpace(errOut) == "" {
			t.Errorf("%v must exit 2 with a usage message, not scan: exit=%d out=%q stderr=%q", args, code, out, errOut)
		}
	}
}

func gateTestReachesScan(t *testing.T, pkg, gateTest string) {
	t.Helper()
	profile := filepath.Join(t.TempDir(), pkg+".cov")
	out, errOut, code := run(goDir(t), "", "go", "test", "-count=1", "-v", "-run", "^"+gateTest+"$", "-coverprofile="+profile, "./internal/"+pkg)
	if code != 0 || !strings.Contains(out, "--- PASS: "+gateTest) {
		t.Fatalf("the repo-wide gate %s must run and pass: exit=%d\n%s%s", gateTest, code, out, errOut)
	}
	funcs, errOut, code := run(goDir(t), "", "go", "tool", "cover", "-func="+profile)
	if code != 0 {
		t.Fatalf("go tool cover exit=%d: %s", code, errOut)
	}
	for _, line := range strings.Split(funcs, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[1] == "Scan" && strings.Contains(fields[0], "/"+pkg+".go:") {
			if fields[2] == "0.0%" {
				t.Errorf("%s never reaches %s.Scan: the gate composes its own scan instead of sharing the one `evolve ratchet check` runs", gateTest, pkg)
			}
			return
		}
	}
	t.Errorf("no %s.Scan in the coverage of %s:\n%s", pkg, gateTest, funcs)
}

func TestC1787_018_SizeratchetRepoGateRunsThroughScan(t *testing.T) {
	gateTestReachesScan(t, "sizeratchet", "TestRatchet_ModuleFunctionsFitTheirAllowances")
}

func TestC1787_019_RawgitratchetRepoGateRunsThroughScan(t *testing.T) {
	gateTestReachesScan(t, "rawgitratchet", "TestRatchet_NoNewRawGitFixtures")
}

func TestC1787_020_ClihealthClearOfAnExpiredBenchExitsOne(t *testing.T) {
	root := benchStore(t, map[string]time.Duration{"agy": -time.Hour})
	out, errOut, code := evolve(t, "clihealth", "list", "--json", "--project-root", root)
	if code != 0 || strings.TrimSpace(out) != "[]" {
		t.Fatalf("precondition: an expired bench is not listed: exit=%d out=%q stderr=%q", code, out, errOut)
	}
	_, errOut, code = evolve(t, "clihealth", "clear", "agy", "--project-root", root)
	if code != 1 || !strings.Contains(errOut, "agy") {
		t.Errorf("clear of a family list does not show as benched exits 1 naming it: exit=%d stderr=%q", code, errOut)
	}
}

func TestC1787_021_CliHealthHyphenatedSpellingServesListAndClear(t *testing.T) {
	root := benchStore(t, map[string]time.Duration{"codex": time.Hour, "agy": time.Hour})
	out, errOut, code := evolve(t, "cli-health", "list", "--json", "--project-root", root)
	if code != 0 || !strings.Contains(out, `"codex"`) {
		t.Errorf("`evolve cli-health list --json` lists the benched codex: exit=%d out=%q stderr=%q", code, out, errOut)
	}
	if _, errOut, code := evolve(t, "cli-health", "clear", "codex", "--project-root", root); code != 0 {
		t.Errorf("`evolve cli-health clear codex` exit=%d stderr=%q", code, errOut)
	}
	if got := storedFamilies(t, root); got["codex"] || !got["agy"] {
		t.Errorf("`cli-health clear codex` removes only codex, store has %v", got)
	}
}
