package ciparity

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gotooltest"
)

func TestCoverageTags_IncludesACSTag(t *testing.T) {
	t.Parallel()
	tags := strings.Fields(CoverageTags)
	has := func(want string) bool {
		for _, tg := range tags {
			if tg == want {
				return true
			}
		}
		return false
	}
	if !has("acs") {
		t.Errorf("CoverageTags = %q; want it to include the \"acs\" build tag so the SSOT measures //go:build acs in-package tests (core/acssuite/phases/audit/evalgate) instead of under-counting them", CoverageTags)
	}
	if !has("integration") {
		t.Errorf("CoverageTags = %q; want it to STILL include \"integration\" (CI's go.yml tagged step depends on it — adding acs must not drop integration)", CoverageTags)
	}
}

func TestCoverageTestArgs_ThreadsBothTagsAndPreservesPkgOrder(t *testing.T) {
	t.Parallel()
	pkgs := []string{"./internal/core", "./internal/evalgate", "./internal/acssuite"}
	args := CoverageTestArgs("cov.out", pkgs)

	tagVal := ""
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-tags" {
			tagVal = args[i+1]
			break
		}
	}
	if tagVal == "" {
		t.Fatalf("CoverageTestArgs did not emit a `-tags <value>` pair: %v", args)
	}
	fields := strings.Fields(tagVal)
	seen := map[string]bool{}
	for _, f := range fields {
		seen[f] = true
	}
	if !seen["integration"] || !seen["acs"] {
		t.Errorf("`-tags` value = %q; want it to carry BOTH \"integration\" and \"acs\" so the scoped run measures both tiers", tagVal)
	}

	if len(args) < len(pkgs) {
		t.Fatalf("args shorter than pkg list: %v", args)
	}
	got := args[len(args)-len(pkgs):]
	for i := range pkgs {
		if got[i] != pkgs[i] {
			t.Errorf("trailing pkg arg[%d] = %q; want %q (scoped package list must be preserved verbatim & in order): full args %v", i, got[i], pkgs[i], args)
		}
	}
}

func TestCoverageTestArgs_TagGatedFixtureMeasuresTaggedCoverage(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("go toolchain not on PATH: %v", err)
	}
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write("go.mod", "module tagfixture\n\ngo 1.23\n")
	write("fixture.go", "package tagfixture\n\n"+
		"func PlainCovered() int { return 1 }\n\n"+
		"func ACSGated() int { return 2 }\n")
	write("plain_test.go", "package tagfixture\n\n"+
		"import \"testing\"\n\n"+
		"func TestPlainCovered(t *testing.T) { _ = PlainCovered() }\n")
	write("acs_gated_test.go", "//go:build acs\n\n"+
		"package tagfixture\n\n"+
		"import \"testing\"\n\n"+
		"func TestACSGated(t *testing.T) { _ = ACSGated() }\n")

	profile := filepath.Join(dir, "cover.out")
	args := CoverageTestArgs(profile, []string{"."})

	runGo := func(a ...string) (string, error) {
		cmd := exec.Command("go", a...)
		cmd.Dir = dir
		cmd.Env = gotooltest.Env(append(os.Environ(), "GOFLAGS=", "GOPROXY=off"))
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	if out, err := runGo(args...); err != nil {
		t.Fatalf("scoped coverage run via CoverageTestArgs failed: %v\nargs: go %s\noutput:\n%s", err, strings.Join(args, " "), out)
	}
	funcOut, err := runGo("tool", "cover", "-func="+profile)
	if err != nil {
		t.Fatalf("go tool cover -func failed: %v\n%s", err, funcOut)
	}

	pct := funcCoveragePct(t, funcOut, "ACSGated")
	if pct <= 0 {
		t.Errorf("ACSGated coverage = %.1f%%; want > 0 — the acs-tagged test did not run through the SSOT, so the acs tier is NOT measured (CoverageTags is missing the acs tag).\ncover -func output:\n%s", pct, funcOut)
	}
	if p := funcCoveragePct(t, funcOut, "PlainCovered"); p <= 0 {
		t.Fatalf("harness sanity failed: PlainCovered coverage = %.1f%% (expected > 0 from the untagged test); the fixture run is broken, not the SSOT:\n%s", p, funcOut)
	}
}

func funcCoveragePct(t *testing.T, funcOut, fn string) float64 {
	t.Helper()
	for _, line := range strings.Split(funcOut, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		name := fields[len(fields)-2]
		if name != fn {
			continue
		}
		pctStr := strings.TrimSuffix(fields[len(fields)-1], "%")
		pct, err := strconv.ParseFloat(pctStr, 64)
		if err != nil {
			t.Fatalf("parse coverage pct %q for %s: %v", pctStr, fn, err)
		}
		return pct
	}
	return -1
}
