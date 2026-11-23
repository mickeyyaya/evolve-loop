//go:build acs

package cycle2623

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitstarts"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	headCoreGitStarts    = 866
	minimumCutPercent    = 10
	coreRecordedRelPath  = "go/internal/core/testdata/gitstarts.json"
	coreGuardTestName    = "TestCoreTestGitStartsStayInsideTheRecordedCount"
	packageNoteRelPath   = "docs/architecture/packages/internal-gitstarts.md"
	coreNoteRelPath      = "docs/architecture/packages/internal-core.md"
	enforceListRelPath   = "go/.apicover-enforce"
	namedTestRelPath     = "go/internal/gitstarts/apicover_named_test.go"
	enforcedPackageEntry = "./internal/gitstarts"
)

func writeTestSource(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := "package sample\n\nfunc sample() {\n" + body + "\n}\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeRecorded(t *testing.T, n int) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "recorded.json")
	if err := os.WriteFile(p, []byte(fmt.Sprintf(`{"recorded": %d}`, n)), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func repeat(line string, n int) string {
	return strings.TrimSuffix(strings.Repeat(line+"\n", n), "\n")
}

func TestC2623_001_CountWeighsEachWayATestStartsGit(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"raw git exec", repeat(`exec.Command("git", "status")`, 3), 3},
		{"git named by a helper argument", repeat(`runIn(t, dir, "git", "add", "-A")`, 2), 2},
		{"method Git on a repo", repeat(`repo.Git("commit", "-m", "x")`, 4), 4},
		{"gittest Fixture", `r := gittest.Fixture(t)`, 5},
		{"gittest Bare", `r := gittest.Bare(t)`, 5},
		{"gittest At", `r := gittest.At(t, dir)`, 5},
		{"gittest Clone", `r := gittest.Clone(t, src)`, 1},
		{"gittest FromTemplate runs no git", `r := gittest.FromTemplate(t, "base", build)`, 0},
		{"gittest BareFromTemplate runs no git", `r := gittest.BareFromTemplate(t)`, 0},
		{"a string that only contains git", `fmt.Println("gitignore", "digit")`, 0},
		{"no git at all", `exec.Command("tar", "-xf", "a.tar")`, 0},
		{"mixed kinds add up", `exec.Command("git", "init")` + "\n" + `r := gittest.Fixture(t)` + "\n" + `r.Git("add", "-A")`, 7},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeTestSource(t, dir, "sample_test.go", tc.body)
			got, err := gitstarts.Count(dir)
			if err != nil {
				t.Fatalf("Count: %v", err)
			}
			if got != tc.want {
				t.Errorf("Count = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestC2623_002_CountSumsEveryTestFileOfTheDirectoryOnly(t *testing.T) {
	dir := t.TempDir()
	writeTestSource(t, dir, "a_test.go", repeat(`exec.Command("git", "status")`, 2))
	writeTestSource(t, dir, "b_test.go", `r := gittest.Fixture(t)`)
	writeTestSource(t, dir, "production.go", repeat(`exec.Command("git", "status")`, 9))
	writeTestSource(t, filepath.Join(dir, "nested"), "c_test.go", repeat(`exec.Command("git", "status")`, 9))
	got, err := gitstarts.Count(dir)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if got != 7 {
		t.Errorf("Count = %d, want 7: two test files of this directory, not the production file or the nested package", got)
	}
	if _, err := gitstarts.Count(filepath.Join(dir, "absent")); err == nil {
		t.Errorf("Count of a directory that does not exist returned no error")
	}
	if err := os.WriteFile(filepath.Join(dir, "broken_test.go"), []byte("package sample\nfunc ("), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := gitstarts.Count(dir); err == nil {
		t.Errorf("Count of a directory with an unparsable test file returned no error")
	}
}

func TestC2623_003_VerifyFailsAboveTheRecordedCountAndOnlyThere(t *testing.T) {
	dir := t.TempDir()
	writeTestSource(t, dir, "sample_test.go", repeat(`exec.Command("git", "status")`, 3))
	for _, tc := range []struct {
		recorded int
		wantFail bool
	}{
		{2, true},
		{0, true},
		{3, false},
		{4, false},
	} {
		err := gitstarts.Verify(dir, writeRecorded(t, tc.recorded))
		if tc.wantFail && err == nil {
			t.Errorf("recorded %d with 3 git starts: Verify returned no error", tc.recorded)
		}
		if !tc.wantFail && err != nil {
			t.Errorf("recorded %d with 3 git starts: Verify failed: %v", tc.recorded, err)
		}
	}
}

func TestC2623_004_VerifyRefusesARecordedCountItCannotTrust(t *testing.T) {
	dir := t.TempDir()
	writeTestSource(t, dir, "sample_test.go", `exec.Command("git", "status")`)
	absent := filepath.Join(t.TempDir(), "absent.json")
	garbled := filepath.Join(t.TempDir(), "garbled.json")
	negative := filepath.Join(t.TempDir(), "negative.json")
	noField := filepath.Join(t.TempDir(), "nofield.json")
	for p, body := range map[string]string{garbled: "not json", negative: `{"recorded": -1}`, noField: `{"total": 9}`} {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for name, p := range map[string]string{"absent": absent, "garbled": garbled, "negative": negative, "missing field": noField} {
		if err := gitstarts.Verify(dir, p); err == nil {
			t.Errorf("%s recorded file: Verify returned no error, so a broken baseline would pass every count", name)
		}
	}
}

func coreTestDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.GoModuleDir(t), "internal", "core")
}

func readRecorded(t *testing.T) int {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(coreRecordedRelPath)))
	if err != nil {
		t.Fatalf("recorded count file: %v", err)
	}
	var doc struct {
		Recorded *int `json:"recorded"`
	}
	if err := json.Unmarshal(body, &doc); err != nil || doc.Recorded == nil {
		t.Fatalf("recorded count file %s is not {\"recorded\": N}: %v", coreRecordedRelPath, err)
	}
	return *doc.Recorded
}

func TestC2623_005_TheCoreGuardRunsFromTheCoreTestBinary(t *testing.T) {
	mod := acsassert.GoModuleDir(t)
	out, errOut, code, _ := acsassert.SubprocessOutput("go", "-C", mod, "test", "-count=1", "-v", "-run", "^"+coreGuardTestName+"$", "./internal/core")
	if code != 0 {
		t.Fatalf("go test -run %s ./internal/core exited %d:\n%s", coreGuardTestName, code, acsassert.OutputTail(out+errOut, 30))
	}
	if !strings.Contains(out, "--- PASS: "+coreGuardTestName) {
		t.Errorf("the core test binary has no passing %s:\n%s", coreGuardTestName, acsassert.OutputTail(out, 20))
	}
}

func TestC2623_006_TheRecordedCountEqualsTheRealCountAndIsCut(t *testing.T) {
	recorded := readRecorded(t)
	real, err := gitstarts.Count(coreTestDir(t))
	if err != nil {
		t.Fatalf("Count of the core tests: %v", err)
	}
	if recorded != real {
		t.Errorf("recorded %d, real %d: the recorded count must equal the real count, with no slack for a later regression to hide in", recorded, real)
	}
	ceiling := headCoreGitStarts * (100 - minimumCutPercent) / 100
	if recorded > ceiling {
		t.Errorf("recorded %d exceeds %d: the core tests must start at least %d%% fewer git processes than the %d of the base", recorded, ceiling, minimumCutPercent, headCoreGitStarts)
	}
	if err := gitstarts.Verify(coreTestDir(t), filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(coreRecordedRelPath))); err != nil {
		t.Errorf("Verify of the real core tests: %v", err)
	}
}

func TestC2623_007_TheNewPackageGraduatesIntoTheApicoverGate(t *testing.T) {
	root := acsassert.RepoRoot(t)
	mod := acsassert.GoModuleDir(t)
	listed := false
	list, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(enforceListRelPath)))
	if err != nil {
		t.Fatalf("enforce list: %v", err)
	}
	for _, line := range strings.Split(string(list), "\n") {
		listed = listed || strings.TrimSpace(line) == enforcedPackageEntry
	}
	if !listed {
		t.Errorf("%s has no line %q", enforceListRelPath, enforcedPackageEntry)
	}
	if !acsassert.FileExists(t, filepath.Join(root, filepath.FromSlash(namedTestRelPath))) {
		t.Fatalf("%s is missing", namedTestRelPath)
	}
	profile := filepath.Join(t.TempDir(), "cover.out")
	out, errOut, code, _ := acsassert.SubprocessOutput("go", "-C", mod, "test", "-count=1", "-coverprofile="+profile, "./internal/gitstarts")
	if code != 0 {
		t.Fatalf("go test ./internal/gitstarts exited %d:\n%s", code, acsassert.OutputTail(out+errOut, 30))
	}
	funcs, errOut, code, _ := acsassert.SubprocessOutput("go", "-C", mod, "tool", "cover", "-func="+profile)
	if code != 0 {
		t.Fatalf("go tool cover -func exited %d:\n%s", code, errOut)
	}
	funcPath := filepath.Join(t.TempDir(), "func.txt")
	if err := os.WriteFile(funcPath, []byte(funcs), 0o644); err != nil {
		t.Fatal(err)
	}
	var report, apicoverErr bytes.Buffer
	if rc := apicover.Main([]string{"-cover", funcPath, "-enforce", filepath.Join(mod, "internal", "gitstarts")}, &report, &apicoverErr); rc != 0 {
		t.Errorf("apicover -enforce on internal/gitstarts exited %d:\n%s%s", rc, report.String(), apicoverErr.String())
	}
}

var steTail = regexp.MustCompile(`ste-lint: (\d+) finding\(s\) in \d+ file\(s\); (\d+) file\(s\) checked`)

func TestC2623_008_TheDocsAreInSimplifiedTechnicalEnglish(t *testing.T) {
	root := acsassert.RepoRoot(t)
	mod := acsassert.GoModuleDir(t)
	for _, rel := range []string{packageNoteRelPath, coreNoteRelPath} {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if !acsassert.FileExists(t, abs) {
			t.Errorf("%s is missing", rel)
			continue
		}
		out, errOut, code, _ := acsassert.SubprocessOutput("go", "-C", mod, "run", "./cmd/evolve", "docs", "ste-lint", "--strict", "--project-root", root, abs)
		m := steTail.FindStringSubmatch(out)
		if m == nil {
			t.Errorf("%s: no ste-lint summary line:\n%s%s", rel, out, acsassert.OutputTail(errOut, 5))
			continue
		}
		findings, _ := strconv.Atoi(m[1])
		checked, _ := strconv.Atoi(m[2])
		if code != 0 || findings != 0 || checked != 1 {
			t.Errorf("%s: ste-lint exit %d, %d finding(s), %d file(s) checked:\n%s", rel, code, findings, checked, out)
		}
	}
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(packageNoteRelPath)))
	if err == nil && !strings.Contains(string(body), "internal/gitstarts") {
		t.Errorf("%s does not name the package it documents", packageNoteRelPath)
	}
}

func TestC2623_009_TheNewFilesAreNotGitIgnored(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, rel := range []string{coreRecordedRelPath, packageNoteRelPath, namedTestRelPath, "go/internal/gitstarts/gitstarts.go"} {
		if !acsassert.FileExists(t, filepath.Join(root, filepath.FromSlash(rel))) {
			t.Errorf("%s is missing", rel)
			continue
		}
		_, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "--quiet", rel)
		if code == 0 {
			t.Errorf("%s is git-ignored: ship would drop it", rel)
		}
	}
}
