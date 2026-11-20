//go:build acs

package cycle2548

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const researchDoc = "docs/research/ci-wall-variance-2026-11.md"

var (
	goTestStart = regexp.MustCompile(`go test `)
	pkgArgument = regexp.MustCompile(`\$(pkgs|run)\b`)
	parallelism = regexp.MustCompile(`(?:^|\s)-p[ =]([^\s;|>]+)`)
)

func hostShim(t *testing.T, cores int) []string {
	t.Helper()
	dir := t.TempDir()
	script := fmt.Sprintf("#!/bin/sh\necho %d\n", cores)
	for _, name := range []string{"nproc", "sysctl"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	env := []string{}
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "PATH=") || strings.HasPrefix(kv, "GOMAXPROCS=") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "GOMAXPROCS="+strconv.Itoa(cores))
}

func dryRun(t *testing.T, cores int, target string, vars ...string) string {
	t.Helper()
	args := append([]string{"-n", "-C", acsassert.GoModuleDir(t), target}, vars...)
	cmd := exec.Command("make", args...)
	cmd.Env = hostShim(t, cores)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("make %v on a %d-core host: %v\n%s", args, cores, err, out)
	}
	return string(out)
}

func goTestSegments(t *testing.T, recipe string) []string {
	t.Helper()
	var segments []string
	for _, loc := range goTestStart.FindAllStringIndex(recipe, -1) {
		rest := recipe[loc[1]:]
		if end := strings.IndexAny(rest, ";|>"); end >= 0 {
			rest = rest[:end]
		}
		segments = append(segments, rest)
	}
	return segments
}

func parallelismOf(t *testing.T, segment string) string {
	t.Helper()
	m := parallelism.FindStringSubmatch(segment)
	if m == nil {
		return ""
	}
	if arg := pkgArgument.FindStringIndex(segment); arg != nil && strings.Index(segment, m[0]) > arg[0] {
		return ""
	}
	return m[1]
}

func darwinParallelism(t *testing.T, cores int, vars ...string) []string {
	t.Helper()
	segments := goTestSegments(t, dryRun(t, cores, "test-integration-darwin", vars...))
	if len(segments) < 3 {
		t.Fatalf("the darwin recipe on a %d-core host has %d go test invocations, want the split run and both main-run branches (3)", cores, len(segments))
	}
	values := make([]string, 0, len(segments))
	for _, s := range segments {
		values = append(values, parallelismOf(t, s))
	}
	return values
}

func TestC2548_001_TheDarwinSetPassesTheSameParallelismOnA3CoreAnd5CoreHost(t *testing.T) {
	three := darwinParallelism(t, 3)
	five := darwinParallelism(t, 5)
	if strings.Join(three, ",") != strings.Join(five, ",") {
		t.Fatalf("go test -p differs by host class: 3-core %v, 5-core %v", three, five)
	}
	for i, v := range three {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			t.Errorf("go test invocation %d has -p %q, want a positive integer before the package argument", i, v)
		}
		if v != three[0] {
			t.Errorf("go test invocation %d has -p %s, but invocation 0 has -p %s", i, v, three[0])
		}
	}
}

func TestC2548_002_TheCIInvocationOfTheDarwinSetKeepsTheFixedParallelism(t *testing.T) {
	ciVars := []string{"TEST_JSON=test-integration.json", "TEST_RUSAGE=1"}
	three := darwinParallelism(t, 3, ciVars...)
	five := darwinParallelism(t, 5, ciVars...)
	if strings.Join(three, ",") != strings.Join(five, ",") {
		t.Fatalf("with TEST_JSON and TEST_RUSAGE set, go test -p differs by host class: 3-core %v, 5-core %v", three, five)
	}
	for i, v := range three {
		if v == "" {
			t.Errorf("go test invocation %d of the CI recipe has no -p before the package argument", i)
		}
	}
}

func TestC2548_003_TheFullSetTargetKeepsTheHostDefaultParallelism(t *testing.T) {
	for _, cores := range []int{3, 5} {
		for i, s := range goTestSegments(t, dryRun(t, cores, "test-integration")) {
			if v := parallelismOf(t, s); v != "" {
				t.Errorf("test-integration on a %d-core host pins -p %s in go test invocation %d, but only the darwin set is fixed", cores, v, i)
			}
		}
	}
}

func TestC2548_004_TheDarwinRecipeKeepsRaceTimeoutAndIntegrationTier(t *testing.T) {
	recipe := dryRun(t, 5, "test-integration-darwin")
	for _, segment := range goTestSegments(t, recipe) {
		for _, want := range []string{"-race", "-count=1", "-timeout 30m", "-tags integration"} {
			if !strings.Contains(segment, want) {
				t.Errorf("go test invocation lost %q: %s", want, strings.TrimSpace(segment))
			}
		}
	}
}

func section(t *testing.T, doc string, mustHave ...string) bool {
	t.Helper()
	for _, block := range strings.Split(doc, "\n\n") {
		all := true
		for _, needle := range mustHave {
			if !strings.Contains(block, needle) {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

// acs-predicate: config-check
func TestC2548_005_TheResearchDocRecordsTheSpanRatioOrNamesWhyNot(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, researchDoc))
	if err != nil {
		t.Fatal(err)
	}
	value := darwinParallelism(t, 3)[0]
	if value == "" {
		t.Fatal("the darwin recipe has no -p value, so the document has nothing to record")
	}
	if !section(t, string(raw), "-p "+value, "3-core", "5-core", "span", "1.3") {
		t.Errorf("%s has no paragraph that names -p %s, the 3-core and 5-core span, and the 1.3 limit", researchDoc, value)
	}
}

func TestC2548_006_TheResearchDocIsStaAndMarksR3Done(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, researchDoc)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	r3Done := false
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "| R3 |") && strings.Contains(line, "ci-macos-fixed-parallelism") && strings.Contains(line, "done in cycle 2548") {
			r3Done = true
		}
	}
	if !r3Done {
		t.Errorf("the R3 row of %s does not name ci-macos-fixed-parallelism as done in cycle 2548", researchDoc)
	}
	words, err := stelint.LoadStandard(root)
	if err != nil {
		t.Fatal(err)
	}
	findings, err := stelint.LintFile(path, stelint.Options{Words: words})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		if !f.IsSkipNotice() {
			t.Errorf("%s line %d %s: %s", researchDoc, f.Line, f.Rule, f.Excerpt)
		}
	}
}
