//go:build acs

// Package cycle334 materializes the cycle-334 acceptance criteria for the one
// behavior-preserving, build-ready task committed to triage `## top_n`:
//
//	hoist-quotareset-hint-re — replace the per-call
//	    `re := regexp.MustCompile("(?i)(\d{1,2}):(\d{2})(am|pm)")` inside
//	    parseHint (go/internal/quotareset/quotareset.go) with a package-level
//	    static var (var hintTimeRE = regexp.MustCompile(...)). Behaviour MUST be
//	    byte-identical; only the compilation timing (per-call → package init)
//	    changes.
//
// Floor binding (R9.3 / cycle-280 lesson). Triage `## top_n` for cycle 334 holds
// TWO entries:
//   - hoist-quotareset-hint-re  (scout, behaviour-preserving, goal-aligned) — gated here.
//   - graduated-enforcement     (operator-HIGH inbox feature)              — NOT gated.
//
// `graduated-enforcement` gets ZERO predicates on purpose, and the reason is a
// disposition decision recorded in test-report.md, not an omission: it is a
// behaviour-CHANGING feature (audit FAIL → correction-retry; tree-diff abort →
// quarantine+warn; SELF_SHA block → warn+repin) dropped into a behaviour-
// PRESERVING refactor cycle whose phase plan runs behavior-baseline /
// behavior-compare — phases that BLOCK exactly the behaviour drift the feature
// introduces. It also arrived with no scout build plan or target files. It is
// dispositioned manual+checklist (carry to a dedicated feature cycle), so
// binding a predicate to it would gate work this cycle cannot ship. The triage-
// DEFERRED `dry-truncate-inline-middle` likewise gets no predicate (deferred-
// floor starvation, cycle-280).
//
// Predicate design (cycle-85 lesson — every gate EXERCISES the system under
// test, no load-bearing source grep):
//
//   - C334_001 is BEHAVIORAL + structural (the "Mixed" category): it drives the
//     real exported entry point quotareset.Compute through the hint-file source
//     path (the ONLY caller of the unexported parseHint), pinning every
//     observable consequence of the regexp match — positive parse, rollover-to-
//     tomorrow, and two negative/rejection cases (malformed text and out-of-
//     range minutes) that fall through to the default source. That behavioural
//     half passes TODAY (the refactor preserves behaviour); the auxiliary
//     structural half (parseHint no longer compiles a regexp in its body) is RED
//     today and is what fails until Builder hoists the pattern. So the predicate
//     is RED now for the RIGHT reason yet locks behaviour so a no-op rename or a
//     behaviour-breaking edit cannot pass.
//
//   - C334_002 is the structural goal-completion gate (waived config/structure
//     check, see the inline waiver): a package-level `var hintTimeRE =
//     regexp.MustCompile(...)` exists AND parseHint's body compiles zero
//     regexps. RED today (the compile lives inline in parseHint; no package
//     var). This proves the pattern MOVED to package init rather than being
//     deleted.
//
// AC map (1:1 with scout-report "Acceptance Criteria Summary" + the eval
// .evolve/evals/hoist-quotareset-hint-re.md AC1..AC5):
//
//	AC1 package-level hintTimeRE var declared                 → C334_002
//	AC2 zero regexp.MustCompile inside parseHint body         → C334_002 (+ auxiliary in C334_001)
//	AC3 all quotareset behaviour preserved / tests pass       → C334_001 (drives Compute end-to-end)
//	AC4 old inline `re := ...MustCompile` removed             → C334_002 (no compile in body)
//	AC5 package still builds                                  → implied (this _test.go compiles against the pkg; C334_001 runs it)
package cycle334

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/quotareset"
	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// the single production file the committed top_n task targets.
const quotaresetSrcRel = "go/internal/quotareset/quotareset.go"

// refNow mirrors the package test's fixed clock so the parsed-time assertions
// are deterministic and independent of wall-clock.
var refNow = time.Date(2026, 5, 23, 14, 0, 0, 0, time.UTC)

// writeHint materializes a quota-reset-hint.txt in a fresh temp workspace and
// returns the workspace dir Compute reads from.
func writeHint(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "quota-reset-hint.txt"), []byte(body), 0o644); err != nil {
		t.Fatalf("write hint fixture: %v", err)
	}
	return dir
}

// assertNoCompileInParseHint is the auxiliary structural half of the Mixed
// predicate: parseHint must no longer compile a regexp in its own body (the
// per-call hot-path compile the task removes).
func assertNoCompileInParseHint(t *testing.T) {
	t.Helper()
	src := filepath.Join(acsassert.RepoRoot(t), quotaresetSrcRel)
	n, err := acsassert.CountInGoFunc(src, "parseHint", "regexp.MustCompile")
	if err != nil {
		t.Fatalf("RED: cannot scan parseHint body in %s: %v", quotaresetSrcRel, err)
	}
	if n != 0 {
		t.Errorf("RED: parseHint body still compiles %d regexp(s) — hoist the pattern to a package-level var (want 0)", n)
	}
}

// ---------------------------------------------------------------------------
// C334_001 — parseHint behaviour preserved, exercised via the real Compute API.
//
// Compute("<workspace>", ...) reads quota-reset-hint.txt and feeds it to the
// unexported parseHint (the regexp owner). We pin every observable consequence
// of the match so a no-op or a behaviour-breaking edit is caught:
//   - "resets 8:30pm" @ 14:00          ⇒ Source=parsed, WakeAt 20:30 today
//   - "5:20am"        @ 14:00          ⇒ Source=parsed, WakeAt rolls to tomorrow
//   - "garbage no time"                ⇒ Source=default (no match — negative)
//   - "99:99am"                        ⇒ Source=default (minutes out of range — negative)
//
// The two negatives are the anti-no-op axis: a stub that always "matched" or
// always rejected would fail one of the four. Auxiliary structural half asserts
// the compile left parseHint's body (RED until Builder hoists).
// ---------------------------------------------------------------------------
func TestC334_001_ParseHintBehaviourPreservedViaCompute(t *testing.T) {
	fixedNow := quotareset.Options{Now: func() time.Time { return refNow }}
	withHours := func(h float64) quotareset.Options {
		return quotareset.Options{Now: func() time.Time { return refNow }, HoursFn: func() float64 { return h }}
	}

	// positive: future-today parse.
	if r, err := quotareset.Compute(writeHint(t, "resets 8:30pm"), fixedNow); err != nil {
		t.Fatalf("Compute(8:30pm): %v", err)
	} else if r.Source != "parsed" || r.WakeAt.Hour() != 20 || r.WakeAt.Minute() != 30 {
		t.Errorf("8:30pm: got Source=%q %02d:%02d, want parsed 20:30", r.Source, r.WakeAt.Hour(), r.WakeAt.Minute())
	}

	// positive: rolls to tomorrow when the time already passed today.
	if r, err := quotareset.Compute(writeHint(t, "5:20am"), fixedNow); err != nil {
		t.Fatalf("Compute(5:20am): %v", err)
	} else if r.Source != "parsed" {
		t.Errorf("5:20am: Source=%q want parsed", r.Source)
	} else if r.WakeAt.Day() != refNow.AddDate(0, 0, 1).Day() {
		t.Errorf("5:20am: WakeAt day=%d want %d (tomorrow)", r.WakeAt.Day(), refNow.AddDate(0, 0, 1).Day())
	}

	// negative: no time token at all ⇒ regexp finds nothing ⇒ default source.
	if r, err := quotareset.Compute(writeHint(t, "garbage no time"), withHours(5.0)); err != nil {
		t.Fatalf("Compute(garbage): %v", err)
	} else if r.Source != "default" {
		t.Errorf("garbage: Source=%q want default (no regexp match)", r.Source)
	}

	// negative: matches the shape but minutes are out of range ⇒ rejected ⇒ default.
	if r, err := quotareset.Compute(writeHint(t, "99:99am"), withHours(5.0)); err != nil {
		t.Fatalf("Compute(99:99am): %v", err)
	} else if r.Source != "default" {
		t.Errorf("99:99am: Source=%q want default (minutes out of range)", r.Source)
	}

	assertNoCompileInParseHint(t)
}

// ---------------------------------------------------------------------------
// C334_002 — structural goal-completion gate.
//
// acs-predicate: config-check — this gate inherently asserts a SOURCE-STRUCTURE
// outcome of a code/token-reduction goal: the per-call compile was hoisted to a
// package-level static var rather than deleted. The behavioural exercise of the
// hoisted regexp lives in C334_001; this gate pins (1) a package-level
// `var hintTimeRE = regexp.MustCompile(...)` now exists and (2) parseHint's body
// compiles zero regexps. Waiver per the Predicate Quality table (Waived grep).
// ---------------------------------------------------------------------------
func TestC334_002_QuotaresetHintRegexpHoisted(t *testing.T) {
	src := filepath.Join(acsassert.RepoRoot(t), quotaresetSrcRel)

	// (1) a package-level hintTimeRE var assigned from regexp.MustCompile.
	if !acsassert.FileMatchesRegex(t, src, `var\s+hintTimeRE\s*=\s*regexp\.MustCompile`) {
		t.Errorf("RED: no package-level `var hintTimeRE = regexp.MustCompile(...)` in %s — the pattern was not hoisted", quotaresetSrcRel)
	}

	// (2) parseHint's body no longer compiles any regexp.
	n, err := acsassert.CountInGoFunc(src, "parseHint", "regexp.MustCompile")
	if err != nil {
		t.Fatalf("RED: cannot scan parseHint body: %v", err)
	}
	if n != 0 {
		t.Errorf("RED: parseHint body still compiles %d regexp(s) — want 0 (use the package-level hintTimeRE)", n)
	}
}
