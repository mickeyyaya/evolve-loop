# Comment history: `acs/cycle1684`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1684/predicates_test.go:3` — above `package cycle1684`

```text
// Package cycle1684 materialises the acceptance criteria of the one
// fleet-scoped task pinned to this lane: `continuation-release-cli-authority-gate`.
//
// WHAT THE CONTRACT IS. The inbox record
// (.evolve/inbox/2026-08-18T17-30-00Z-continuation-release-cli-authority-gate.json)
// states the fix in one sentence:
//
//	"Gate the release subcommand like its siblings: require -operator (or
//	 EVOLVE_OPERATOR_CONFIRM=1), refuse when a live cycle lease exists for the
//	 scope's lane unless -force, and log the release into the ledger
//	 (who/when/why) so lineage erasure is itself evidenced."
//
// Today `evolve continuation release <scope-id>` (runContinuationRelease,
// go/cmd/evolve/cmd_continuation.go:104-144) has NO authority gate at all: its
// FlagSet declares only -project-root, and the function drops straight from
// arg-parsing to an unconditional inboxmover.ReleaseContinuationBinding with a
// hardcoded "operator-release" reason that names no actual caller. Any
// Bash-capable process — an in-cycle agent included — can drop a live scope's
// binding, which silently widens the registry's ORCHESTRATOR-side-only
// authority invariant (ADR-0085/0089, cycle-1285 anti-tamper) and erases the
// lineage the defect-ledger gate depends on.
//
// WHY THESE PREDICATES DRIVE THE BINARY. runContinuationRelease lives in
// package main, so no test can import it; the ONLY way to prove the gate is
// reached from the production entry point (rather than sitting in dead code a
// test calls directly) is to build ./cmd/evolve once in TestMain and drive the
// real CLI. 001-005 do exactly that; 006 additionally calls the authority
// helper in-process, because criterion 5 is specifically that a direct package
// caller cannot BYPASS the gate the CLI goes through.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - NEGATIVE : 001 (ungated refuses, binding intact, nothing recorded) and
//     004 (a LIVE lease refuses even for a fully authorized operator). 004 is
//     the load-bearing discriminator — an implementation that adds only the
//     -operator flag passes 001-003 and fails here.
//   - EDGE/OOD : 003b drives EVOLVE_OPERATOR_CONFIRM=0, which a sloppy
//     os.Getenv(...) != "" gate accepts; 005b drives a lease whose heartbeat
//     has aged past runlease.DefaultTTL, which must NOT block — otherwise every
//     dead cycle's leftover lease bricks its scope forever (over-correction is
//     as much a defect as the gap).
//   - SEMANTIC : 002/005a do not merely assert "a release happened"; they
//     demand the durable record answer WHO (a named authority field), WHEN (a
//     parseable RFC3339 stamp) and WHY (a reason), and that a -force release be
//     distinguishable from an ordinary one. A constant reason string for every
//     path fails 005a.
//
// The record's HOME is deliberately not pinned. The inbox record says "the
// ledger"; the run ledger (.evolve/ledger.jsonl, already reachable from
// inboxmover.Options.Ledger) and the released_continuations[] annotation on the
// scope's inbox item are both durable and both defensible, and fault
// localization named the second while the item text reads as the first. The
// criterion is about the record's CONTENT, so releaseRecords scans both homes
// and a hit in either satisfies it. test-report.md records this reading.
```

### `go/acs/cycle1684/predicates_test.go:339` — above `root := acsassert.RepoRoot(t)`

```text
// Auxiliary wiring evidence: the production CLI must reach the helper
// rather than inlining a second copy of the check (the drift that produced
// audit cycle-1507's H2). Not load-bearing on its own.
```

### `go/acs/cycle1684/predicates_test.go:349` — above `func TestC1684_007_EditedPackagesVetAndGofmtClean(t *testing.T) {`

```text
// TestC1684_007_EditedPackagesVetAndGofmtClean pins criterion 4.
//
// Deliberately NARROWED to the two packages this task edits. The criterion's
// literal text ("go vet ./... and go build ./... clean; full suite green") is a
// repo-wide sweep, which the flaky-predicate-shape rules ban from a cycle
// predicate: a whole-repo go test under fleet load is the false-RED generator
// that failed cycles 1173/1175/1178 on sound work. The repo-wide sweep is the
// build phase's own obligation and CI's `-count=1` job; what a predicate can
// hold honestly is that the packages the diff touches stay vet- and
// gofmt-clean. test-report.md states this narrowing.
```

### `go/acs/cycle1684/predicates_test.go:589` — above `func TestC1684_008_ExplanationLimitationsMatchTheBoundTree(t *testing.T) {`

```text
// TestC1684_008_ExplanationLimitationsMatchTheBoundTree pins the defect audit
// round 2 raised as H1: the cycle's REQUIRED explanation document is
// cryptographically bound (diff_sha256 over every changed path, test files
// included — go/internal/explanationdocs/gitio.go:48-53) to a tree its prose
// misdescribes. Its `## Limitations` says cycle 1515's predicate "asserts that
// an ungated release exits 0" and "is recorded for adjudication rather than
// edited here" — but that predicate was re-authored inside this same diff and
// now drives `-operator`. The durable record tells a future reader this cycle
// shipped leaving the superseded contract unadjudicated, which is backwards.
//
// WHY THIS IS NOT A GREP. The load-bearing half EXECUTES the system: it drives
// the real `evolve` binary with no authority and observes whether the gate this
// cycle ships is actually live, exactly as predicate 001 does. That executed
// observation — not a string in a file — is what establishes which of the two
// prose readings is the true one. The document check is then a CONSISTENCY
// assertion between an executed fact and the durable record of it.
//
// It is also not vacuous in either direction: both branches assert. If the
// tree is gated (the expected state, which 001-007 independently pin) the
// document must record the adjudication and must not claim the deferral; if the
// tree were somehow ungated the document must not claim an adjudication that
// did not happen. A document can only satisfy this predicate by describing the
// tree it is bound to. Adding a magic string cannot pass it — the vocabulary
// required is decided by what the binary does.
```

### `go/acs/cycle1684/predicates_test.go:630` — above `p1515 := filepath.Join(root, "go", "acs", "regression", "cycle1515", "predicates_test.go")`

```text
// (2) Auxiliary corroboration only: cycle 1515's release predicate now
//     reaches that gate through -operator rather than asserting exit 0
//     without it. Read from disk because running a second ACS package
//     inside this one is the nested-suite shape the flaky-predicate rules
//     ban; the EGPS suite already runs cycle1515 for real.
```

### `go/acs/cycle1684/predicates_test.go:664` — above `var deferralMarkers = []string{`

```text
// deferralMarkers assert "this cycle did NOT fix cycle 1515's predicate".
```
