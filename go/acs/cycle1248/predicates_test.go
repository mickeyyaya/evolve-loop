//go:build acs

package cycle1248

import (
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	probePkg     = "./internal/reachabilityprobe"
	phasecmdPkg  = "./internal/cli/phasecmd"
	resolveFocus = "TestResolvePackage"
)

func goTest(t *testing.T, args ...string) (string, int) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("go toolchain unavailable: %v", err)
	}
	cmd := exec.Command("go", append([]string{"test"}, args...)...)
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if ok := asExitError(err, &exitErr); ok {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("go test %v: %v\n%s", args, err, out)
		}
	}
	return string(out), code
}

func asExitError(err error, target **exec.ExitError) bool {
	if e, ok := err.(*exec.ExitError); ok {
		*target = e
		return true
	}
	return false
}

var (
	resolveOut  string
	resolveCode int
	resolveDone bool
)

func resolveRun(t *testing.T) (string, int) {
	t.Helper()
	if !resolveDone {
		resolveOut, resolveCode = goTest(t, "-count=1", "-run", resolveFocus, "-v", probePkg)
		resolveDone = true
	}
	return resolveOut, resolveCode
}

func passingNames(out string) []string {
	var names []string
	re := regexp.MustCompile(`(?m)^\s*--- PASS: (\S+)`)
	for _, m := range re.FindAllStringSubmatch(out, -1) {
		names = append(names, m[1])
	}
	return names
}

func requireCase(t *testing.T, out string, want *regexp.Regexp, label string) {
	t.Helper()
	for _, name := range passingNames(out) {
		if want.MatchString(name) {
			t.Logf("%s satisfied by passing test %s", label, name)
			return
		}
	}
	t.Errorf("no PASSING test covers %s (want name matching %s)\npassing names: %v\n--- transcript ---\n%s",
		label, want, passingNames(out), out)
}

func TestC1248_001_resolve_package_tests_execute_green(t *testing.T) {
	out, code := resolveRun(t)
	if code != 0 {
		t.Fatalf("go test -run %s ./internal/reachabilityprobe exited %d\n%s", resolveFocus, code, out)
	}
	if strings.Contains(out, "no tests to run") {
		t.Fatalf("no test matches -run %s: resolvePackage's base-name fallback is still uncovered\n%s", resolveFocus, out)
	}
	names := passingNames(out)
	if len(names) == 0 {
		t.Fatalf("expected at least one PASSING %s* test, got none\n%s", resolveFocus, out)
	}
	t.Logf("passing %s* tests/subtests: %v", resolveFocus, names)
}

func TestC1248_002_covers_multi_candidate_selection(t *testing.T) {
	out, code := resolveRun(t)
	if code != 0 {
		t.Fatalf("go test -run %s exited %d\n%s", resolveFocus, code, out)
	}
	requireCase(t, out, regexp.MustCompile(`(?i)ambig|multi|candidate|nearest|prefix`),
		"multi-candidate base-name selection (nearest neighbour wins)")
}

func TestC1248_003_covers_deterministic_tie_break(t *testing.T) {
	out, code := resolveRun(t)
	if code != 0 {
		t.Fatalf("go test -run %s exited %d\n%s", resolveFocus, code, out)
	}
	requireCase(t, out, regexp.MustCompile(`(?i)tie|determin|lexic|stable`),
		"deterministic lexical tie-break between equally-scored candidates")
}

func TestC1248_004_covers_no_candidate_fail_open(t *testing.T) {
	out, code := resolveRun(t)
	if code != 0 {
		t.Fatalf("go test -run %s exited %d\n%s", resolveFocus, code, out)
	}
	requireCase(t, out, regexp.MustCompile(`(?i)nomatch|no_?candidate|unknown|unresolv|fail_?open|absent|none`),
		"negative case: identifier matching no package resolves to nothing (fails open)")
}

func TestC1248_005_reachabilityprobe_package_green(t *testing.T) {
	out, code := goTest(t, "-count=1", probePkg)
	if code != 0 {
		t.Fatalf("go test %s exited %d\n%s", probePkg, code, out)
	}
}

// acs-predicate: config-check — documentation presence is inherently a
func TestC1248_006_gates_table_documents_frozen_pin_gate(t *testing.T) {
	doc := filepath.Join(acsassert.RepoRoot(t), "docs/operations/runtime-reference.md")
	acsassert.FileExists(t, doc)
	for _, anchor := range []string{
		"CheckFrozenPins",
		"reachabilityprobe",
		"evolve phase verify tdd",
	} {
		acsassert.FileContains(t, doc, anchor)
	}
	acsassert.FileMatchesRegex(t, doc, `(?m)^\|.*(CheckFrozenPins|reachabilityprobe).*\|`)
}

func TestC1248_007_documented_gate_is_live(t *testing.T) {
	out, code := goTest(t, "-count=1", "-run", "TestPhaseVerifyTDD_FrozenPin", phasecmdPkg)
	if code != 0 {
		t.Fatalf("go test -run TestPhaseVerifyTDD_FrozenPin %s exited %d\n%s", phasecmdPkg, code, out)
	}
	if strings.Contains(out, "no tests to run") {
		t.Fatalf("the frozen-pin gate guard has vanished; the documented gate is not live\n%s", out)
	}
}
