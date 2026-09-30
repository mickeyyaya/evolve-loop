# Comment history: `acs/cycle335`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle335/predicates_test.go:3` — above `package cycle335`

```text
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
```

### `go/acs/cycle335/predicates_test.go:147` — above `func TestC335_003_SemverIsSemverBehaviour(t *testing.T) {`

```text
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
```
