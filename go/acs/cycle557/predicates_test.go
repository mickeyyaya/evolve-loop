//go:build acs

// Package cycle557 materialises the cycle-557 acceptance criteria for this
// fleet lane's SOLE `## top_n` task per triage-report.md:
//
//	fuzz-parser-surfaces (slice 1) — add Go-native fuzz harnesses for
//	clihealth.ParseResetHint and bridge.ClassifyExhausted, seeded from the
//	existing golden fixtures (TestParseResetHintTable,
//	TestClassifyExhausted_RealManifests), asserting never-panic + sane
//	invariants (no negative durations, no far-future resets).
//
// Per the AC-Materialization Contract (R9.3 "predicates bind ONLY to triage-
// committed work"), this package predicates ONLY that item. Slices (2)-(5) of
// the inbox item (panestream pane-rule fuzz, quotastate parse fuzz,
// manifest+inbox loader fuzz, fleet.Partition rapid tests) are explicitly out
// of scope this cycle per triage-report.md's rationale and get NO predicate
// here.
//
// Scope correction (read-first, AGENTS.md rule 8): the inbox item and
// triage-report.md both label FuzzClassifyExhausted a "clihealth" fuzz target
// alongside FuzzParseResetHint. The real exported ClassifyExhausted lives in
// go/internal/bridge (usageclassify.go), not go/internal/clihealth — clihealth
// has no exhausted-classification function at all. Materializing a
// FuzzClassifyExhausted against a nonexistent clihealth API would invent an
// API (banned); the fuzz harness is authored against the real function in its
// actual package instead, which is what the acceptance criterion's plain-
// language function names ("FuzzParseResetHint + FuzzClassifyExhausted")
// substantively require.
//
// Predicate strategy (behavioral-via-subprocess, the cycle-549/553/555
// precedent — never a source grep): each predicate drives `go test -list`
// then `go test -run -fuzz -fuzztime` as subprocesses over the REAL compiled
// fuzz targets the TDD engineer authored this cycle:
//
//	go/internal/clihealth/resetparse_fuzz_test.go   FuzzParseResetHint
//	go/internal/bridge/usageclassify_fuzz_test.go   FuzzClassifyExhausted
//
// Before this cycle, `go test -list '^Fuzz'` on either package printed
// nothing (zero Fuzz funcs existed anywhere in the repo — the inbox's own
// "0 fuzz tests today" claim, independently confirmed by TDD-engineer grep).
// The -list assertion is RED for the right reason until the harness exists;
// this is a test-infrastructure-authoring task, so the harnesses ARE the
// deliverable and are pre-existing GREEN once authored (documented in
// test-report.md) — the underlying ParseResetHint/ClassifyExhausted
// implementations were already correct going in (both -fuzztime=8s local runs
// found zero crashers, "new interesting" corpus growth only). Builder's job
// is to extend fuzz time / wire a CI lane and fix anything a longer run
// surfaces, per the inbox's own "bounded fuzztime in CI" framing.
package cycle557

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// baselineCoverageRe matches `go test -fuzz`'s own progress line, e.g.
// "gathering baseline coverage: 130/130 completed" — the real corpus size the
// fuzzing engine loaded (seeds via f.Add + any cached testdata/fuzz entries),
// reported by the toolchain itself rather than grepped from source.
var baselineCoverageRe = regexp.MustCompile(`gathering baseline coverage: (\d+)/(\d+) completed`)

const (
	clihealthPkg = "github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	bridgePkg    = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

// requireListed fails unless `go test -list <pattern> <pkg>` prints funcName —
// a real compiled-binary reflection over the package's registered tests, not a
// source grep (renaming/removing the func changes this output for real).
func requireListed(t *testing.T, pkg, funcName string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-list", "^"+funcName+"$", pkg)
	if err != nil && code == 0 {
		t.Fatalf("go test -list failed to launch: %v\nstderr:\n%s", err, stderr)
	}
	if !strings.Contains(stdout, funcName) {
		t.Errorf("go test -list '^%s$' %s did not list %s (fuzz harness missing or renamed):\nstdout:\n%s\nstderr:\n%s",
			funcName, pkg, funcName, stdout, stderr)
	}
}

// requireFuzzGreen drives a real bounded fuzz run (not just the seed corpus)
// over the compiled target and requires a clean exit: no crash, no failed
// seed, and at least minSeeds distinct seed subtests actually ran (closes the
// cycle-85 "no tests to run" degenerate trap — a fuzz func with zero seeds and
// a pattern that matches nothing would otherwise pass vacuously).
func requireFuzzGreen(t *testing.T, pkg, funcName string, minSeeds int) {
	t.Helper()
	pattern := "^" + funcName + "$"
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-run", pattern, "-fuzz", pattern, "-fuzztime=3s", "-v", pkg)
	out := stdout + "\n" + stderr

	matches := baselineCoverageRe.FindAllStringSubmatch(out, -1)
	if len(matches) == 0 {
		t.Errorf("no fuzz run detected for %s in %s (missing \"gathering baseline coverage\" line) — harness did not run:\n%s", funcName, pkg, out)
		return
	}
	total, _ := strconv.Atoi(matches[len(matches)-1][2])
	if total < minSeeds {
		t.Errorf("only %d corpus entrie(s) loaded for %s (need >= %d) — seed corpus missing or too small:\n%s",
			total, funcName, minSeeds, out)
		return
	}
	if code != 0 || strings.Contains(out, "--- FAIL") || strings.Contains(out, "FAIL\t") {
		t.Errorf("%s in %s failed a bounded fuzz run (exit=%d):\n%s", funcName, pkg, code, out)
	}
}

// TestC557_001_FuzzParseResetHintRegistered (AC1a): the fuzz harness for
// ParseResetHint is a real, discoverable Go fuzz target in clihealth — not a
// renamed/removed symbol, not a plain unit test masquerading as a fuzz func.
func TestC557_001_FuzzParseResetHintRegistered(t *testing.T) {
	requireListed(t, clihealthPkg, "FuzzParseResetHint")
}

// TestC557_002_FuzzParseResetHintGreenSeededFromGoldens (AC1b): the harness
// runs its golden-seeded corpus plus a bounded live-fuzz pass over the REAL
// ParseResetHint and finds no crasher / invariant violation (negative
// duration-until-reset, or a reset hint that escapes the 24h cap).
func TestC557_002_FuzzParseResetHintGreenSeededFromGoldens(t *testing.T) {
	requireFuzzGreen(t, clihealthPkg, "FuzzParseResetHint", 15)
}

// TestC557_003_FuzzClassifyExhaustedRegistered (AC2a): the fuzz harness for
// ClassifyExhausted is a real, discoverable Go fuzz target in bridge (the
// function's actual home package — see the scope-correction note above).
func TestC557_003_FuzzClassifyExhaustedRegistered(t *testing.T) {
	requireListed(t, bridgePkg, "FuzzClassifyExhausted")
}

// TestC557_004_FuzzClassifyExhaustedGreenSeededFromGoldens (AC2b): the harness
// runs its golden-seeded corpus (real claude/codex/agy /usage pane shapes)
// plus a bounded live-fuzz pass over the REAL ClassifyExhausted and finds no
// panic (adversarial family strings included: path-traversal-shaped, empty,
// NUL/invalid-UTF8 bytes) and no nondeterminism.
func TestC557_004_FuzzClassifyExhaustedGreenSeededFromGoldens(t *testing.T) {
	requireFuzzGreen(t, bridgePkg, "FuzzClassifyExhausted", 8)
}
