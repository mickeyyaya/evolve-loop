//go:build acs

package cycle2064

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const modulePrefix = "github.com/mickeyyaya/evolve-loop/go/"

const warnHeading = "## Packages above their recorded p90"

var testlatencyBinary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "testlatency-bin-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := func() int {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		root, err := acsassert.ModuleDir(wd)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		testlatencyBinary = filepath.Join(dir, "testlatency")
		build := exec.Command("go", "build", "-o", testlatencyBinary, "./cmd/testlatency")
		build.Dir = root
		if out, err := build.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "build testlatency: %v\n%s", err, out)
			return 1
		}
		return m.Run()
	}()
	os.RemoveAll(dir)
	os.Exit(code)
}

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

type run struct {
	stdout string
	stderr string
	code   int
}

func runTestlatency(t *testing.T, stream string, args ...string) run {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(testlatencyBinary, args...)
	cmd.Stdin = strings.NewReader(stream)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("run testlatency: %v", err)
	}
	return run{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

func packageStream(pkg string, wallSeconds float64) string {
	return fmt.Sprintf("{\"Action\":\"run\",\"Package\":%q,\"Test\":\"TestSlow\"}\n"+
		"{\"Action\":\"pass\",\"Package\":%q,\"Test\":\"TestSlow\",\"Elapsed\":%.2f}\n"+
		"{\"Action\":\"pass\",\"Package\":%q,\"Elapsed\":%.2f}\n", pkg, pkg, wallSeconds, pkg, wallSeconds)
}

type recorded struct {
	Class string  `json:"class"`
	P90   float64 `json:"p90"`
}

func writeRatchet(t *testing.T, packages map[string]recorded) string {
	t.Helper()
	body, err := json.MarshalIndent(map[string]any{"packages": packages}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "test-latency.json")
	if err := os.WriteFile(path, append(body, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readRatchet(t *testing.T, path string) map[string]recorded {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Packages map[string]recorded `json:"packages"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("%s is not the ratchet shape {\"packages\":{pkg:{class,p90}}}: %v\n%s", path, err, body)
	}
	return parsed.Packages
}

func warnSection(report string) string {
	start := strings.Index(report, warnHeading)
	if start < 0 {
		return ""
	}
	rest := report[start+len(warnHeading):]
	if end := strings.Index(rest, "\n## "); end >= 0 {
		return rest[:end]
	}
	return rest
}

func TestC2064_001_RatchetFileRecordsClassAndP90OfEveryTestedPackage(t *testing.T) {
	root := goDir(t)
	rel := ".test-latency.json"
	path := filepath.Join(root, rel)
	if !acsassert.FileExists(t, path) {
		t.Fatalf("RED: go/%s is missing on disk", rel)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("RED: go/%s is untracked, so it may be gitignored and dropped at ship", rel)
	}
	packages := readRatchet(t, path)
	if len(packages) == 0 {
		t.Fatalf("go/%s records no package", rel)
	}
	for name, rec := range packages {
		if !slices.Contains([]string{"small", "medium", "large"}, rec.Class) {
			t.Errorf("%s: class %q is not small, medium or large", name, rec.Class)
		}
		if rec.P90 <= 0 {
			t.Errorf("%s: p90 %v must be a positive number of seconds", name, rec.P90)
		}
	}
	list := exec.Command("go", "list", "-f", "{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}", "./...")
	list.Dir = root
	out, err := list.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	for _, importPath := range strings.Fields(string(out)) {
		key := strings.TrimPrefix(importPath, modulePrefix)
		if _, ok := packages[key]; !ok {
			t.Errorf("package %s has tests but no record in go/%s", key, rel)
		}
	}
}

func TestC2064_002_GateFailsOnlyAboveTwiceTheClassCeiling(t *testing.T) {
	cases := []struct {
		class       string
		okWall      float64
		failingWall float64
	}{
		{"small", 110, 130},
		{"medium", 590, 610},
		{"large", 1190, 1250},
	}
	for _, c := range cases {
		t.Run(c.class, func(t *testing.T) {
			pkg := "example.com/m/" + c.class + "pkg"
			ratchet := writeRatchet(t, map[string]recorded{pkg: {Class: c.class, P90: c.failingWall}})
			if r := runTestlatency(t, packageStream(pkg, c.okWall), "-ratchet", ratchet); r.code != 0 {
				t.Errorf("%s package at %.0fs: exit %d, want 0 (within 2 x ceiling)\nstdout:\n%s\nstderr:\n%s", c.class, c.okWall, r.code, r.stdout, r.stderr)
			}
			r := runTestlatency(t, packageStream(pkg, c.failingWall), "-ratchet", ratchet)
			if r.code != 1 {
				t.Errorf("%s package at %.0fs: exit %d, want 1 (above 2 x ceiling)\nstdout:\n%s\nstderr:\n%s", c.class, c.failingWall, r.code, r.stdout, r.stderr)
			}
			if !strings.Contains(r.stdout+r.stderr, pkg) {
				t.Errorf("failure names no package:\n%s%s", r.stdout, r.stderr)
			}
		})
	}
}

func TestC2064_003_GateNeverFailsAFastPackageAmongSlowOnes(t *testing.T) {
	slow, fast := "example.com/m/slowpkg", "example.com/m/fastpkg"
	ratchet := writeRatchet(t, map[string]recorded{
		slow: {Class: "small", P90: 50},
		fast: {Class: "small", P90: 1},
	})
	r := runTestlatency(t, packageStream(slow, 500)+packageStream(fast, 0.5), "-ratchet", ratchet)
	if r.code != 1 {
		t.Fatalf("exit %d, want 1 for the package over its ceiling\nstdout:\n%s", r.code, r.stdout)
	}
	if strings.Contains(r.stderr, fast) {
		t.Errorf("a package within its ceiling is named as a failure:\n%s", r.stderr)
	}
}

func TestC2064_004_TestTimeoutCapsTheCeiling(t *testing.T) {
	cases := []struct {
		class   string
		wall    float64
		timeout string
	}{
		{"small", 100, "90s"},
		{"large", 700, "10m"},
	}
	for _, c := range cases {
		t.Run(c.class, func(t *testing.T) {
			pkg := "example.com/m/" + c.class + "pkg"
			ratchet := writeRatchet(t, map[string]recorded{pkg: {Class: c.class, P90: c.wall}})
			stream := packageStream(pkg, c.wall)
			if r := runTestlatency(t, stream, "-ratchet", ratchet); r.code != 0 {
				t.Errorf("default timeout, %s package at %.0fs: exit %d, want 0", c.class, c.wall, r.code)
			}
			r := runTestlatency(t, stream, "-ratchet", ratchet, "-test-timeout", c.timeout)
			if r.code != 1 {
				t.Errorf("-test-timeout %s, %s package at %.0fs: exit %d, want 1 (min(2 x ceiling, timeout) is the limit)\nstdout:\n%s\nstderr:\n%s", c.timeout, c.class, c.wall, r.code, r.stdout, r.stderr)
			}
		})
	}
}

func TestC2064_005_RatchetThatCannotBeReadOrParsedExitsTwo(t *testing.T) {
	stream := packageStream("example.com/m/p", 1)
	dir := t.TempDir()
	malformed := filepath.Join(dir, "malformed.json")
	unknownClass := filepath.Join(dir, "unknown-class.json")
	if err := os.WriteFile(malformed, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	unknown := writeRatchet(t, map[string]recorded{"example.com/m/p": {Class: "huge", P90: 1}})
	body, err := os.ReadFile(unknown)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unknownClass, body, 0o644); err != nil {
		t.Fatal(err)
	}
	valid := writeRatchet(t, map[string]recorded{"example.com/m/p": {Class: "small", P90: 1}})
	if r := runTestlatency(t, stream, "-ratchet", valid); r.code != 0 {
		t.Errorf("valid ratchet and a fast package: exit %d, want 0\nstderr:\n%s", r.code, r.stderr)
	}
	for name, path := range map[string]string{
		"missing":       filepath.Join(dir, "absent.json"),
		"malformed":     malformed,
		"unknown class": unknownClass,
	} {
		r := runTestlatency(t, stream, "-ratchet", path)
		if r.code != 2 {
			t.Errorf("%s ratchet: exit %d, want 2\nstdout:\n%s\nstderr:\n%s", name, r.code, r.stdout, r.stderr)
		}
		if !strings.Contains(r.stderr, filepath.Base(path)) {
			t.Errorf("%s ratchet: stderr does not name %s, the failure must be loud:\n%s", name, filepath.Base(path), r.stderr)
		}
	}
}

func TestC2064_006_WarnsOnlyAboveP90ByFiftyPercentAndThirtySeconds(t *testing.T) {
	cases := []struct {
		name string
		p90  float64
		wall float64
		warn bool
	}{
		{"doubled and far above", 100, 200, true},
		{"just past both margins", 100, 151, true},
		{"just past the thirty second margin", 40, 71, true},
		{"doubled but only fifteen seconds", 10, 25, false},
		{"forty seconds but only forty percent", 100, 140, false},
		{"under thirty seconds over", 40, 69, false},
		{"faster than recorded", 100, 20, false},
	}
	recordedByPkg := map[string]recorded{}
	var stream string
	for i, c := range cases {
		pkg := fmt.Sprintf("example.com/m/case%d", i)
		recordedByPkg[pkg] = recorded{Class: "large", P90: c.p90}
		stream += packageStream(pkg, c.wall)
	}
	r := runTestlatency(t, stream, "-ratchet", writeRatchet(t, recordedByPkg))
	if r.code != 0 {
		t.Fatalf("exit %d, want 0: a warning is not a failure\nstdout:\n%s\nstderr:\n%s", r.code, r.stdout, r.stderr)
	}
	section := warnSection(r.stdout)
	for i, c := range cases {
		pkg := fmt.Sprintf("example.com/m/case%d", i)
		if listed := strings.Contains(section, pkg+" ") || strings.Contains(section, pkg+"|"); listed != c.warn {
			t.Errorf("%s (p90 %.0f, wall %.0f): listed under %q = %v, want %v\nreport:\n%s", c.name, c.p90, c.wall, warnHeading, listed, c.warn, r.stdout)
		}
	}
}

func TestC2064_007_WarningNamesRecordedP90AndMeasuredWall(t *testing.T) {
	pkg := "example.com/m/regressed"
	ratchet := writeRatchet(t, map[string]recorded{pkg: {Class: "large", P90: 100}})
	r := runTestlatency(t, packageStream(pkg, 200), "-ratchet", ratchet)
	section := warnSection(r.stdout)
	for _, want := range []string{pkg, "100.00", "200.00"} {
		if !strings.Contains(section, want) {
			t.Errorf("warning section lacks %q:\n%s", want, r.stdout)
		}
	}
}

func TestC2064_008_TightenLowersARecordedP90AndKeepsTheClass(t *testing.T) {
	ratchet := writeRatchet(t, map[string]recorded{
		"example.com/m/faster": {Class: "medium", P90: 100},
		"example.com/m/slower": {Class: "small", P90: 30},
		"example.com/m/unrun":  {Class: "large", P90: 400},
		"example.com/m/steady": {Class: "small", P90: 12},
	})
	stream := packageStream("example.com/m/faster", 40) +
		packageStream("example.com/m/slower", 80) +
		packageStream("example.com/m/steady", 12) +
		packageStream("example.com/m/unrecorded", 5)
	r := runTestlatency(t, stream, "tighten", "-ratchet", ratchet)
	if r.code != 0 {
		t.Fatalf("tighten exit %d, want 0\nstdout:\n%s\nstderr:\n%s", r.code, r.stdout, r.stderr)
	}
	got := readRatchet(t, ratchet)
	want := map[string]recorded{
		"example.com/m/faster": {Class: "medium", P90: 40},
		"example.com/m/slower": {Class: "small", P90: 30},
		"example.com/m/unrun":  {Class: "large", P90: 400},
		"example.com/m/steady": {Class: "small", P90: 12},
	}
	if len(got) != len(want) {
		t.Errorf("tighten changed the set of recorded packages: got %v, want %v", got, want)
	}
	for pkg, w := range want {
		if got[pkg] != w {
			t.Errorf("%s: got %+v, want %+v", pkg, got[pkg], w)
		}
	}
}

func TestC2064_009_TightenNeverRaisesARecordedValue(t *testing.T) {
	ratchet := writeRatchet(t, map[string]recorded{
		"example.com/m/a": {Class: "small", P90: 10},
		"example.com/m/b": {Class: "medium", P90: 200},
	})
	before, err := os.ReadFile(ratchet)
	if err != nil {
		t.Fatal(err)
	}
	stream := packageStream("example.com/m/a", 55) + packageStream("example.com/m/b", 200)
	if r := runTestlatency(t, stream, "tighten", "-ratchet", ratchet); r.code != 0 {
		t.Fatalf("tighten exit %d, want 0\nstderr:\n%s", r.code, r.stderr)
	}
	after, err := os.ReadFile(ratchet)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("tighten rewrote a ratchet it could only raise:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestC2064_010_TightenWithoutAReadableRatchetExitsTwo(t *testing.T) {
	stream := packageStream("example.com/m/a", 1)
	missing := filepath.Join(t.TempDir(), "absent.json")
	if r := runTestlatency(t, stream, "tighten", "-ratchet", missing); r.code != 2 {
		t.Errorf("tighten of a missing ratchet: exit %d, want 2\nstderr:\n%s", r.code, r.stderr)
	}
	if r := runTestlatency(t, stream, "tighten"); r.code != 2 {
		t.Errorf("tighten without -ratchet: exit %d, want 2\nstderr:\n%s", r.code, r.stderr)
	}
}

// acs-predicate: config-check
func TestC2064_011_BothCIJobsGateOnTheCommittedRatchet(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), ".github", "workflows", "go.yml"))
	if err != nil {
		t.Fatal(err)
	}
	gating := 0
	for _, line := range strings.Split(string(body), "\n") {
		if !strings.Contains(line, "cmd/testlatency") || !strings.Contains(line, "-ratchet") {
			continue
		}
		gating++
		if !strings.Contains(line, ".test-latency.json") {
			t.Errorf("a ratchet step does not read the committed file: %s", strings.TrimSpace(line))
		}
		if !strings.Contains(line, `"$GITHUB_STEP_SUMMARY"`) {
			t.Errorf("a ratchet step does not write the job summary: %s", strings.TrimSpace(line))
		}
		if strings.Contains(line, "||") {
			t.Errorf("a ratchet step swallows its exit status, so it cannot fail CI: %s", strings.TrimSpace(line))
		}
	}
	if gating < 2 {
		t.Errorf("go.yml has %d ratchet steps, want one in each of build-test and the darwin job", gating)
	}
}

func TestC2064_012_ExistingPackageBudgetFlagsKeepTheirContract(t *testing.T) {
	over := packageStream("example.com/m/slow", 500)
	if r := runTestlatency(t, over, "-package-budget"); r.code != 1 {
		t.Errorf("-package-budget over budget: exit %d, want 1", r.code)
	}
	if r := runTestlatency(t, over, "-budget", "180s"); r.code != 0 {
		t.Errorf("-budget alone is advisory: exit %d, want 0", r.code)
	}
	if r := runTestlatency(t, over); r.code != 0 || strings.Contains(r.stdout, warnHeading) {
		t.Errorf("no -ratchet: exit %d, report must carry no %q section\n%s", r.code, warnHeading, r.stdout)
	}
}

func TestC2064_013_CIParityTestsStayGreen(t *testing.T) {
	cmd := exec.Command("go", "test", "-count=1", "./internal/ciparity")
	cmd.Dir = goDir(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("go test ./internal/ciparity: %v\n%s", err, acsassert.OutputTail(string(out), 40))
	}
}
