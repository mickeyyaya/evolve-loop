//go:build acs

// Package cycle1108 materialises the cycle-1108 acceptance criteria for the
// single triage-committed top_n task of this lane:
// `gitstage-quotepath-determinism` (layer 3 of the ship staging onion).
//
// The defect: ship decides "what to stage" from `git status --porcelain` and
// `git check-ignore` output, and neither reader accounts for git's C-quoting.
// porcelainChangedPaths (manifest.go:198) strips wrapping quotes only, so
// `"caf\303\251.txt"` becomes a 15-byte escaped string that exists on no disk
// and matches no manifest entry; dropIgnoredPaths (gitops.go:801) trims
// whitespace only, so a quoted probe line never matches the raw declared path
// it is filtering and an ignored path survives into `git add` — the cycle-1101
// rc=1 ship-killer, narrowed to the non-ASCII/quote-bearing input class.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…1104
// precedent). porcelainChangedPaths, dropIgnoredPaths and stageExplicitPaths
// are all UNEXPORTED in internal/phases/ship, so they cannot be imported from
// here; each predicate instead shells `go test -run` over the RED contract
// tests authored this cycle in go/internal/phases/ship. Every one of those
// CALLS the parser/filter directly or drives the real shipDirect staging path
// and asserts on returned values and on the argv git was actually invoked
// with. None is a source-grep of production code (the cycle-85
// degenerate-predicate ban).
//
// RED now: porcelainChangedPaths does not unescape, the git reads carry no
// `-c core.quotePath=false`, and dropIgnoredPaths does not decode probe output.
package cycle1108

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const shipPkg = "github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"

// runGoTest shells `go test -run '^(<pattern>)$' -count=1 <pkg>` and reports
// whether it exited cleanly plus the combined output. -count=1 defeats the test
// cache so the predicate always exercises current source. A compile failure in
// the target package (a possible RED signal before Builder implements) surfaces
// as a non-zero exit.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	// code < 0 is a genuine launch failure (binary missing / killed by signal),
	// not a test verdict; SubprocessOutput returns non-nil err for ANY non-zero
	// exit, so a plain compile/assertion failure (code 1/2 — the RED signal)
	// must flow through as ok=false, NOT be misread as "failed to launch".
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

// TestC1108_001_PorcelainDecodesQuotedPaths — AC1. Drives porcelainChangedPaths
// over raw `git status --porcelain` output captured from real git: octal-escaped
// non-ASCII, an escaped embedded quote, an escaped backslash, a control escape,
// and BOTH sides of a quoted rename must all round-trip to the literal on-disk
// path. Until they do, the token ship stages is a string no file bears.
func TestC1108_001_PorcelainDecodesQuotedPaths(t *testing.T) {
	ok, out := runGoTest(t, shipPkg, "TestPorcelainChangedPaths_QuotePathUnescapesNonASCII")
	if !ok {
		t.Errorf("porcelain classification still yields git's C-quoted escape text instead of the "+
			"real path — a changed file whose name needs quoting matches no manifest entry and "+
			"stages as a no-op:\n%s", out)
	}
}

// TestC1108_002_AsciiClassificationUnchanged — AC2, NEGATIVE/anti-regression.
// The unquote helper must not touch the common case: plain ASCII entries, the
// quoted-but-unescaped space path git emits regardless of core.quotePath, a
// literal backslash in an UNQUOTED entry, ASCII renames, and short/blank lines
// must decode byte-identically to today. This is the predicate that fails if
// Builder "fixes" quoting by unescaping unconditionally.
func TestC1108_002_AsciiClassificationUnchanged(t *testing.T) {
	ok, out := runGoTest(t, shipPkg, "TestPorcelainChangedPaths_QuotePathAsciiUnchanged")
	if !ok {
		t.Errorf("the unquote change altered ASCII path classification — mangling the overwhelmingly "+
			"common case is a worse regression than the bug it fixes:\n%s", out)
	}
}

// TestC1108_003_GitReadsDisableQuotePath — AC3, argv-level and behavioural: a
// real cycle ship runs through shipDirect and the recorded git argv for BOTH
// classification reads (`status --porcelain`, `check-ignore`) must carry
// `-c core.quotePath=false` before the subcommand — the zero-parsing fix that
// removes escaping for the common non-ASCII case at the source.
func TestC1108_003_GitReadsDisableQuotePath(t *testing.T) {
	ok, out := runGoTest(t, shipPkg, "TestStageExplicitPaths_QuotePathDisabledOnGitReads")
	if !ok {
		t.Errorf("ship still reads `git status --porcelain` / `git check-ignore` with git's default "+
			"core.quotePath=true (or passes the config arg after the subcommand, where git rejects "+
			"it), so every non-ASCII path arrives escaped:\n%s", out)
	}
}

// TestC1108_004_IgnoredProbeMatchesQuotedOutput — AC4, positive half. An ignored
// non-ASCII or quote-bearing declared path must be dropped from the pathspec
// even when `check-ignore` reports it C-quoted; otherwise it reaches `git add`,
// which exits 1 on ANY ignored pathspec and kills the ship (cycle-1101).
func TestC1108_004_IgnoredProbeMatchesQuotedOutput(t *testing.T) {
	ok, out := runGoTest(t, shipPkg, "TestDropIgnoredPaths_QuotePathMatchesQuotedProbeOutput")
	if !ok {
		t.Errorf("the check-ignore filter still keys off git's quoted spelling, so an ignored "+
			"quote-bearing path survives into `git add` and reproduces the cycle-1101 rc=1 "+
			"refusal:\n%s", out)
	}
}

// TestC1108_005_IgnoredProbeNeverOverMatches — AC4, NEGATIVE half. Decoding
// probe output must not make the filter fuzzy: a probe naming a different path,
// a probe naming only the ESCAPED spelling of a path we never declared, and an
// empty probe result must all leave the declared set intact. Silently dropping
// a declared path under-stages the ship into a falsely-clean commit — strictly
// worse than the refusal the filter exists to prevent.
func TestC1108_005_IgnoredProbeNeverOverMatches(t *testing.T) {
	ok, out := runGoTest(t, shipPkg, "TestDropIgnoredPaths_QuotePathKeepsUnignoredPaths")
	if !ok {
		t.Errorf("the check-ignore filter now drops paths git never reported as ignored — an "+
			"under-staged ship commits less than it declares:\n%s", out)
	}
}

// TestC1108_006_ShipStagingContractStillGreen — AC5 anti-regression across the
// whole staging onion: the landed layer-1 (absolute pathspec, `d202aeb6`),
// layer-2 (ignored paths, `e8990e53`), explicit-pathspec (cycle-1067) and
// staged-deletion contracts must all still hold. This is the predicate that
// fails if Builder greens the quoting cases by reworking the shared staging
// path rather than extending it.
func TestC1108_006_ShipStagingContractStillGreen(t *testing.T) {
	if ok, out := runGoTest(t, shipPkg,
		"TestShipDirect_CycleClass_.*|TestShipDirect_ManualClass_EmptyManifestFallsBackToChangedSet|"+
			"TestShipDirect_NoWorkspacePath_StillStagesExplicitly|TestShipDirect_NonReleaseClasses_NeverAddAll|"+
			"TestShipDirect_CheckIgnoreProbeFailure_FailsOpen|TestShipFromWorktree_.*|TestStagePathspec_.*|"+
			"TestIsRepoRelative|TestExtractReportPaths_.*|TestStageExplicitPaths_AlreadyStagedDeletion"); !ok {
		t.Errorf("the ship staging contract (explicit pathspec / repo-relative filter / ignored-path "+
			"drop / staged-deletion handling) regressed:\n%s", out)
	}
}
