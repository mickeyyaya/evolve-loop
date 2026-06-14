//go:build acs

// Package cycle335 materializes the cycle-335 acceptance criteria for the two
// behavior-preserving DRY tasks committed to triage `## top_n`:
//
//	dry-truncate-inline-middle — extract the byte-identical unexported
//	    truncateInline/truncateMiddle helpers from
//	    go/internal/logfilter/streamjson.go and
//	    go/internal/phasestream/classify.go into a new go/internal/textutil
//	    package (exported TruncateInline/TruncateMiddle), and rewrite both
//	    callers to delegate. Bodies are byte-identical; only the home of the
//	    code changes.
//
//	dry-issemver-4pkg — extract the byte-identical `var semverRE =
//	    regexp.MustCompile("^[0-9]+\.[0-9]+\.[0-9]+$")` + IsSemver wrapper from
//	    four packages (changeloggen, versionbump, marketplacepoll,
//	    releasepipeline) into a new go/internal/semvercheck leaf package. The
//	    four packages delegate; changeloggen keeps its thin EXPORTED IsSemver
//	    wrapper (external callers cmd_changelog.go + releasepipeline/bridges.go).
//
// Floor binding (R9.3 / cycle-280 lesson). Triage `## top_n` for cycle 335 holds
// exactly these two entries; both are gated here. The nine carryoverTodos are
// triage-DEFERRED failure records and get ZERO predicates (deferred-floor
// starvation, cycle-280). No coverage floor is committed, so no floor predicate.
//
// Predicate design (cycle-85 lesson — every gate EXERCISES the system under
// test; no load-bearing source grep):
//
//   - C335_001 / C335_003 are BEHAVIORAL: they import the NEW packages and call
//     the real functions, pinning every observable consequence (positive =
//     no-truncation / valid-semver, negative = truncation marker / rejected
//     semver). Because go/internal/textutil and go/internal/semvercheck do not
//     exist yet, THIS TEST PACKAGE FAILS TO COMPILE today — the correct RED for
//     a new-package task (the import cannot resolve until Builder creates the
//     package). Once GREEN, a no-op or behaviour-breaking edit cannot pass: the
//     exact elision strings and the semver accept/reject set are asserted.
//     C335_003 additionally drives the SURVIVING public wrapper
//     changeloggen.IsSemver to prove the delegation preserves behaviour.
//
//   - C335_002 / C335_004 are the structural dedup-completion gates (waived
//     config/structure checks — see inline waivers): the local copies were
//     REMOVED from the caller packages (not merely shadowed) and the new SSOT
//     file exists on disk. The behavioural weight is carried by C335_001 /
//     C335_003; these gates prove the duplication is actually gone.
//
// AC map (1:1 with scout-report "Acceptance Criteria Summary"):
//
//	A1 textutil.go exists with TruncateInline+TruncateMiddle    → C335_001 (calls them) + C335_002 (file on disk)
//	A2 logfilter & phasestream carry zero local truncate defs   → C335_002
//	A3 all three packages' behaviour preserved / tests pass     → C335_001 (drives textutil end-to-end)
//	B1 semvercheck.go exists with IsSemver                       → C335_003 (calls it) + C335_004 (file on disk)
//	B2 zero `^var semverRE` in the four target packages          → C335_004
//	B3 semver behaviour preserved incl. negatives (v1.2.3/1.2/…) → C335_003 (semvercheck + changeloggen delegation)
package cycle335

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/changeloggen"
	"github.com/mickeyyaya/evolve-loop/go/internal/semvercheck"
	"github.com/mickeyyaya/evolve-loop/go/internal/textutil"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// production files the two committed top_n tasks target.
const (
	logfilterSrcRel       = "go/internal/logfilter/streamjson.go"
	phasestreamSrcRel     = "go/internal/phasestream/classify.go"
	textutilSrcRel        = "go/internal/textutil/textutil.go"
	semvercheckSrcRel     = "go/internal/semvercheck/semvercheck.go"
	changeloggenSrcRel    = "go/internal/changeloggen/changeloggen.go"
	versionbumpSrcRel     = "go/internal/versionbump/versionbump.go"
	marketplacepollSrcRel = "go/internal/marketplacepoll/marketplacepoll.go"
	releasepipelineSrcRel = "go/internal/releasepipeline/releasepipeline.go"
)

// ---------------------------------------------------------------------------
// C335_001 — textutil truncation behaviour, exercised via the real exported API.
//
// The extracted helpers MUST be byte-for-byte behaviour-identical to the
// unexported originals. We pin both observable branches of each function so a
// no-op (return s) or a marker change cannot pass:
//   - TruncateInline short input            ⇒ returned unchanged (positive)
//   - TruncateInline long input             ⇒ "<head>… (N bytes elided)" (negative/truncated)
//   - TruncateMiddle short input            ⇒ returned unchanged (positive)
//   - TruncateMiddle long input             ⇒ "<head>… (N bytes elided) …<tail>" (negative/truncated)
//
// RED today: go/internal/textutil does not exist, so the import above does not
// resolve and the whole cycle335 package fails to compile (the correct RED for a
// new-package task).
// ---------------------------------------------------------------------------
func TestC335_001_TextutilTruncateBehaviour(t *testing.T) {
	// TruncateInline: no truncation when len(s) <= n.
	if got := textutil.TruncateInline("hello", 100); got != "hello" {
		t.Errorf("TruncateInline short: got %q, want %q (must return input unchanged)", got, "hello")
	}

	// TruncateInline: truncation appends the exact elision marker (10 bytes, n=4 → 6 elided).
	if got, want := textutil.TruncateInline("0123456789", 4), "0123… (6 bytes elided)"; got != want {
		t.Errorf("TruncateInline long: got %q, want %q", got, want)
	}

	// TruncateMiddle: no truncation when len(s) <= head+tail+32.
	if got := textutil.TruncateMiddle("short", 5, 5); got != "short" {
		t.Errorf("TruncateMiddle short: got %q, want %q (must return input unchanged)", got, "short")
	}

	// TruncateMiddle: keeps head+tail with the middle elision marker.
	// s = "HEAD!" + 40×"x" + "!TAIL" (len 50); head=5 tail=5 ⇒ 40 elided.
	s := "HEAD!" + strings.Repeat("x", 40) + "!TAIL"
	if got, want := textutil.TruncateMiddle(s, 5, 5), "HEAD!… (40 bytes elided) …!TAIL"; got != want {
		t.Errorf("TruncateMiddle long: got %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// C335_002 — truncate dedup-completion gate.
//
// acs-predicate: config-check — this gate asserts a SOURCE-STRUCTURE outcome of
// the code/token-reduction goal: the local truncateInline/truncateMiddle
// definitions were REMOVED from both caller packages (the duplication is gone),
// and the new textutil SSOT file exists on disk. The behavioural exercise of the
// extracted functions lives in C335_001. Waiver per the Predicate Quality table
// (Waived grep / config-check).
// ---------------------------------------------------------------------------
func TestC335_002_TruncateLocalDefsRemoved(t *testing.T) {
	root := acsassert.RepoRoot(t)

	// the new SSOT file must exist on disk.
	if !acsassert.FileExists(t, filepath.Join(root, textutilSrcRel)) {
		t.Errorf("RED: %s missing — the extracted truncation helpers have no home", textutilSrcRel)
	}

	// both callers must carry ZERO local `func truncateInline`/`func truncateMiddle`
	// definitions (call sites like `textutil.TruncateInline(...)` do NOT match
	// "func truncate" and are unaffected).
	for _, rel := range []string{logfilterSrcRel, phasestreamSrcRel} {
		src := filepath.Join(root, rel)
		if n := acsassert.CountOccurrencesAny(src, "func truncateInline", "func truncateMiddle"); n != 0 {
			t.Errorf("RED: %s still defines %d local truncate helper(s) — delegate to textutil instead (want 0)", rel, n)
		}
	}
}

// ---------------------------------------------------------------------------
// C335_003 — semver validation behaviour, exercised via the real exported API.
//
// semvercheck.IsSemver is the new SSOT; changeloggen.IsSemver is the SURVIVING
// public wrapper that now delegates to it. We drive BOTH so the gate proves the
// SSOT works AND the delegation preserves behaviour. Accept exactly the
// MAJOR.MINOR.PATCH integer triple; reject everything else:
//   - "1.2.3", "0.0.0", "10.20.30" ⇒ true  (positive)
//   - "v1.2.3", "1.2", "1.2.3.4", "", "abc", "1.2.3-rc1" ⇒ false (negative)
//
// The negatives are the anti-no-op axis: a stub returning constant true (or a
// pattern accepting a "v" prefix / 4 segments / pre-release) fails one of them.
// RED today: go/internal/semvercheck does not exist, so the import does not
// resolve and the package fails to compile (correct new-package RED).
// ---------------------------------------------------------------------------
func TestC335_003_SemverIsSemverBehaviour(t *testing.T) {
	valid := []string{"1.2.3", "0.0.0", "10.20.30"}
	invalid := []string{"v1.2.3", "1.2", "1.2.3.4", "", "abc", "1.2.3-rc1"}

	// new SSOT: semvercheck.IsSemver.
	for _, s := range valid {
		if !semvercheck.IsSemver(s) {
			t.Errorf("semvercheck.IsSemver(%q) = false, want true", s)
		}
	}
	for _, s := range invalid {
		if semvercheck.IsSemver(s) {
			t.Errorf("semvercheck.IsSemver(%q) = true, want false (must reject non-MAJOR.MINOR.PATCH)", s)
		}
	}

	// surviving public wrapper: changeloggen.IsSemver must delegate identically.
	for _, s := range valid {
		if !changeloggen.IsSemver(s) {
			t.Errorf("changeloggen.IsSemver(%q) = false, want true (delegation regressed)", s)
		}
	}
	for _, s := range invalid {
		if changeloggen.IsSemver(s) {
			t.Errorf("changeloggen.IsSemver(%q) = true, want false (delegation regressed)", s)
		}
	}
}

// ---------------------------------------------------------------------------
// C335_004 — semver dedup-completion gate.
//
// acs-predicate: config-check — asserts the SOURCE-STRUCTURE outcome: the local
// `var semverRE = regexp.MustCompile(...)` declaration was REMOVED from all four
// caller packages (zero `^var semverRE`, matching scout's verifiableBy grep) and
// the new semvercheck SSOT file exists on disk. The behavioural exercise lives
// in C335_003. Waiver per the Predicate Quality table (Waived grep / config-check).
// ---------------------------------------------------------------------------
func TestC335_004_SemverRELocalDefsRemoved(t *testing.T) {
	root := acsassert.RepoRoot(t)

	// the new SSOT file must exist on disk.
	if !acsassert.FileExists(t, filepath.Join(root, semvercheckSrcRel)) {
		t.Errorf("RED: %s missing — the extracted semverRE/IsSemver have no home", semvercheckSrcRel)
	}

	// none of the four caller packages may keep a local `var semverRE` decl.
	for _, rel := range []string{
		changeloggenSrcRel, versionbumpSrcRel, marketplacepollSrcRel, releasepipelineSrcRel,
	} {
		src := filepath.Join(root, rel)
		if n := acsassert.CountOccurrencesAny(src, "var semverRE"); n != 0 {
			t.Errorf("RED: %s still declares `var semverRE` (%d) — delegate to semvercheck instead (want 0)", rel, n)
		}
	}
}
