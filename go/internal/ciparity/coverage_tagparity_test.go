package ciparity

// Cycle-555 RED tests for the SOLE `## top_n` task per triage-report.md:
//
//	coverage-gate-tag-parity — close the residual gap left by the already-landed
//	ciparity.CoverageTestArgs/CoverageTags SSOT (commit 78d73f08): the SSOT tag
//	set is "integration" only, but the repo puts real coverage behind BOTH
//	//go:build integration AND //go:build acs tags. Four internal/** packages
//	(core, acssuite, phases/audit, evalgate) carry in-package `//go:build acs`
//	tests, so a coverage run through the SSOT with the acs tag MISSING under-
//	counts them — the exact defect class the SSOT was created to kill (R1:
//	knowledge-base/research/test-coverage-audit-2026-07.md — 47.0% plain vs
//	90.6% tagged for internal/phases/ship). The Builder makes these GREEN by
//	adding `acs` to CoverageTags (→ "integration acs"); the Builder MUST NOT
//	edit this test file.
//
// These are in-package behavioral tests (they call the SSOT and, for the
// regression, run a real `go test -coverprofile` through it over a hermetic
// tag-gated fixture module). The cycle-555 ACS predicates in
// go/acs/cycle555/predicates_test.go gate on THESE via a `go test` subprocess.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestCoverageTags_IncludesACSTag (AC1): the SSOT tag set names BOTH the
// integration and acs tiers, so a coverage run through it exercises every
// tag-gated test the repo's 6-layer architecture puts behind those tags. RED
// today: CoverageTags == "integration" (no acs).
func TestCoverageTags_IncludesACSTag(t *testing.T) {
	tags := strings.Fields(CoverageTags)
	has := func(want string) bool {
		for _, tg := range tags {
			if tg == want {
				return true
			}
		}
		return false
	}
	// acs is the missing tier (the whole point of this cycle).
	if !has("acs") {
		t.Errorf("CoverageTags = %q; want it to include the \"acs\" build tag so the SSOT measures //go:build acs in-package tests (core/acssuite/phases/audit/evalgate) instead of under-counting them", CoverageTags)
	}
	// Guard against a no-op-that-regresses: keep the integration tier too
	// (dropping it would silently exclude the real-FS/git/tmux tier CI measures).
	if !has("integration") {
		t.Errorf("CoverageTags = %q; want it to STILL include \"integration\" (CI's go.yml tagged step depends on it — adding acs must not drop integration)", CoverageTags)
	}
}

// TestCoverageTestArgs_ThreadsBothTagsAndPreservesPkgOrder (AC2): the SSOT
// command builder threads the full tag set into a single `-tags` value and
// preserves the scoped package list verbatim & in order as trailing args (so
// `go test` measures exactly the scoped set, not the whole module). RED today:
// the -tags value is "integration" only.
func TestCoverageTestArgs_ThreadsBothTagsAndPreservesPkgOrder(t *testing.T) {
	pkgs := []string{"./internal/core", "./internal/evalgate", "./internal/acssuite"}
	args := CoverageTestArgs("cov.out", pkgs)

	// Locate the single -tags value.
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

	// The scoped package list must survive verbatim & in order as the trailing args.
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

// TestCoverageTestArgs_TagGatedFixtureMeasuresTaggedCoverage (AC3, the load-
// bearing regression, non-gameable): stand up a hermetic fixture module with a
// function covered ONLY by an `//go:build acs` test, run a real coverage
// profile through the SSOT (ciparity.CoverageTestArgs), and assert the acs-
// gated function IS measured as covered. This proves the acs tag actually flows
// through the SSOT to `go test` — the defining behavior of the fix.
//
// RED today: CoverageTags lacks acs → the acs-only test is excluded → the
// acs-gated function reports 0.0% → this test fails. A Builder cannot satisfy
// it by touching a magic string; only really threading the acs tag through the
// SSOT makes the fixture's acs-gated code count.
func TestCoverageTestArgs_TagGatedFixtureMeasuresTaggedCoverage(t *testing.T) {
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
	// PlainCovered is exercised by an untagged test → the package ALWAYS has a
	// test file, so `go test -coverprofile` always writes a profile (avoids the
	// "[no test files]" no-profile edge). ACSGated is exercised ONLY by the
	// //go:build acs test → its coverage is the acs-tag discriminator.
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
	// The system under test: build the go-test args through the SSOT.
	args := CoverageTestArgs(profile, []string{"."})

	runGo := func(a ...string) (string, error) {
		cmd := exec.Command("go", a...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOFLAGS=", "GOPROXY=off")
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
	// Sanity: the untagged control function must always be covered — proves the
	// harness itself works and the 0% above is specifically the missing acs tag.
	if p := funcCoveragePct(t, funcOut, "PlainCovered"); p <= 0 {
		t.Fatalf("harness sanity failed: PlainCovered coverage = %.1f%% (expected > 0 from the untagged test); the fixture run is broken, not the SSOT:\n%s", p, funcOut)
	}
}

// funcCoveragePct extracts the coverage percentage for a named function from
// `go tool cover -func` output (lines look like
// "tagfixture/fixture.go:5:\tACSGated\t100.0%"). Returns -1 if the function is
// not listed at all.
func funcCoveragePct(t *testing.T, funcOut, fn string) float64 {
	t.Helper()
	for _, line := range strings.Split(funcOut, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		// second-to-last field is the func name; last is the percentage.
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
