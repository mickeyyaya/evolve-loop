# The graduation test-only class
**Period:** 2026-08-02 → 2026-08-03 · **Status:** shipped
**Primary artifacts:** PR #398 (squash `c76e4b3f`) · `go/internal/ciparity/ciparity.go` (`PackageDirHasProductionGoFiles`) · `go/internal/core/phase_bindings_graduation.go` · `go/internal/phases/audit/ciparity.go`

## Problem

The 2026-08-02 batch halted at cycle-1228: fingerprint `build|unknown|a020515c`
recurred three times (cycles 1223/1224/1228), tripping the
identical-fingerprint breaker (P0 auto-filed; PR #398 body).

The mechanism was a collision between two correct behaviors. The tdd phase,
doing RED-first TDD properly, mints a new package containing **only** a
`_test.go` file (`go/internal/testhygienegate/`). The build-entry graduation
guard (`buildGraduationCheck`) flags any package NEW this cycle that is absent
from `go/.apicover-enforce` and **aborts the build phase** — a hard shipping
obligation, deliberately abort-capable (its file header: "graduation is a hard
shipping obligation the builder itself must satisfy"). But a test-only package
has **zero exported production symbols**, so the repo-wide apicover gate this
graduation protects cannot fire on it — CI's own enforce step passes such
packages with "0 exported, 0 covered". The obligation was **vacuous**, the
abort **fatal**, and by the guard's own documented design the prescription
never rides the correction ladder, so nothing in-cycle could cure it. The next
cycle re-minted the same package; the breaker (correctly) halted the batch
(squash commit body).

## Context & evidence

- Root cause was diagnosed **from the preserved worktrees** of the failed
  cycles, not from the fingerprint alone (PR #398 body) — salvage-before-
  requeue paying off.
- The graduation guard itself carries prior history: its header records this
  as the build-entry half of a recurring blind spot ("3rd recurrence: cycles
  575/587/652"; the audit half landed 2026-07-07), and the prescription text
  exists because cycle-1218 proved that merely *naming* the offending package
  cost three build phases and a halt
  (`phase_bindings_graduation.go`, guard header + `graduationPrescription`
  comment).
- The class evidence for "nothing in-cycle could cure it": the abort reason
  reaches the operator log and the resume path, **not** an in-cycle builder
  re-dispatch — the correction ladder does not carry it
  (`graduationPrescription`'s WHO-READS-THIS comment, itself a review
  correction against overclaiming).

## Approaches considered

- **Fix only the build seam.** Rejected in the design itself: the audit side
  (`apicoverNewPackageGraduationDefault`) applies the same obligation to the
  offender list, so skipping test-only packages at build-entry alone "would
  move the same vacuous FAIL one phase later" (PR #398 body;
  `phases/audit/ciparity.go` comment). Two seams, one predicate, or the class
  survives in a new costume.
- **Guess the edge-case directions.** Refused — each direction was pinned with
  a stated reason: a recursive `...` pattern **stays flagged** (it names no
  single directory to inspect, so the predicate reports true rather than
  guessing); an unreadable dir **fails open** (a package that cannot be proven
  obligation-bearing must not fail a phase — repo-wide CI is the backstop,
  matching `packageNewThisCycle`'s git-error direction); and the moment a
  production file lands in the same package, the abort **returns** — pinned by
  the mixed-package test case (PR #398 body; predicate doc comment in
  `ciparity.go`).
- **Skip silently.** Built that way in round 1; **rejected by the adversarial
  review** (F2, HIGH) — a deferred enrollment obligation recorded nowhere is
  how obligations evaporate. Both seams now announce the deferral.
- **The author's own "two-cycle dodge" worry, refuted by the review.** The
  concern that a builder could land the test-only half in cycle A and the
  production half in cycle B unenrolled was **refuted by F6**: the audit seam
  has no new-this-cycle filter, so cycle B *is* caught there and cannot ship.
  The cost is seam quality (the obligation moves back to audit, where FAIL
  feeds the correction ladder and is curable), not a hole (squash commit,
  second message).

## Decision & reasoning

One predicate, both seams: `ciparity.PackageDirHasProductionGoFiles(moduleDir,
pkg)` — true iff the package directory holds at least one non-test `.go` file,
i.e. an exported API surface the repo-wide gate could actually inspect.
Consulted by **both** `core.buildGraduationCheck` (abort seam) and
`apicoverNewPackageGraduationDefault` (audit offender seam), "so a package
cannot pass the build seam and then die at audit for the same vacuous reason"
(predicate doc comment). Single-sourcing the predicate is the load-bearing
decision: the two seams live in different packages and drift is exactly how
this class would recur. The obligation is deferred, not waived — the audit
seam re-flags the package on **any** later change, so enrollment re-raises the
moment production code lands.

## Implementation

Two rounds, both in the squash `c76e4b3f` (merged 2026-08-03):

**Round 1** — predicate + both seams + tests, RED first:
`test-only-package-does-not-abort` reproduced the live abort **verbatim**
(full prescription text) before the fix. Audit fixtures that declared
`files_new` in handoff JSON without materializing the file now write it — the
gate's contract legitimately includes reading the package dir. 7 files
changed.

**Round 2 — the opus adversarial review BLOCKed, and the block was cleared
with proof** (squash commit, second message):

- **F1 (HIGH): the audit seam's skip was mutation-survivable.** Zero tests
  drove the predicate to false at that seam; deleting the skip left the whole
  suite green — the exact seams-silently-disagreeing class the package's own
  `graduation_registration_test` documents. Cleared by
  `TestApicoverNewPkgGraduationDefault_TestOnlyPackageNotFlagged` **with the
  kill proven**: with the skip deleted the test goes RED; restored, 4/4
  suites green under `-race`. The wiring proof was demanded, not volunteered.
- **F2 (HIGH): the suppression was silent at both seams.** Both now announce:
  `[build-floor] graduation deferred: new package X is test-only (no
  production .go surface) — abort suppressed; audit re-flags the package on
  any later change` and `[audit] graduation deferred: … enrollment obligation
  re-raises when production code lands` (visible in
  `phase_bindings_graduation.go` and `phases/audit/ciparity.go` on main).
- Review sharpenings adopted into the record: F6 (above) and F5 — on the
  *live* path (git-derived changed set; handoffs dormant in the last six run
  dirs) suppression on an absent dir is CORRECT and removes a pre-existing
  false FAIL on deleted packages; the residual risk is confined to the
  dormant-but-preferred handoff branch.

Verification: `go test -race -count=1
./internal/{ciparity,core,phases/audit}/ ./cmd/evolve/` — 4/4 PASS, no
regression; `evolve apicover -enforce internal/ciparity` — 7 exported, 7
covered, 0 uncovered (PR body).

## Results (measured)

- The halt class is closed at both seams with the RED→GREEN pair and the
  mutation-kill as the regression evidence; the mixed-package case pins that
  the abort returns when a production file lands.
- The breaker's behavior throughout was **correct** — three identical
  fingerprints on an incurable abort is precisely what it exists to halt; the
  cycle-normalized fingerprint identity (`bc2e3236` lineage) gave the class a
  stable name to fix against.
- No recurrence of `build|unknown|a020515c` observed in the subsequent
  2026-08-03/04 batch record [session-evidence]; the pinned tests, not the
  absence-of-observation, are the durable evidence.
- Honest residuals, queued rather than gated (the reviewer's own ranking,
  recorded in the squash body for the next boundary filing): **F3** — the
  predicate keys on non-test-file *presence*, not exported-symbol presence, so
  a panic-stub or doc.go-only mint still aborts un-curably (the fix
  **narrowed** the halt class, it did not close it); **F4** — the `...` branch
  is dead in production and its unit row overstates coverage; **F8** — the
  offender-positive fixtures lost manifest-vs-disk discriminating power.

## Retrospective — what we learned

- **Vacuous obligation + fatal enforcement + no in-cycle cure = guaranteed
  breaker halt.** Any abort-capable gate needs an answer to "can the flagged
  condition ever be legitimately empty of obligation?" — here, TDD's own
  RED-first discipline manufactured the condition on every bugfix cycle.
- **Fix a two-seam gate at both seams or you've only moved the failure.** The
  single shared predicate is the mechanism that makes the seams *unable* to
  disagree, which is strictly stronger than fixing them identically.
- **Mutation-kill proofs are the review standard for gate changes.** The
  round-1 fix was behaviorally correct and still BLOCKed, because a skip no
  test can see die is a skip the next refactor deletes silently. "Delete the
  fix, watch the suite go RED" is cheap and decisive.
- **Never silently skip** — a suppressed obligation must announce itself at
  the seam that suppressed it, or status accounting loses it (the same
  disease documented in [the batch integrity
  review](2026-08-batch-integrity-review.md)).
- **The review record kept the author's refuted claim.** F6 disproving the
  two-cycle-dodge worry is written into the commit body with the same care as
  the findings — refutations of our own analysis are evidence, not
  embarrassment.

## Links

- PR #398 / squash `c76e4b3f` (both round messages carry the full record)
- ADR-0069 (CI parity — the repo-wide apicover gate this graduation feeds)
- Sibling entries: [Worktree provisioning retry](2026-08-worktree-provisioning-retry.md)
  — the same week's other identical-fingerprint batch halt;
  [Fingerprint identity](2026-07-fingerprint-identity.md) — why the breaker
  could see this class at all; [The batch integrity
  review](2026-08-batch-integrity-review.md) — the status-accounting disease
  the F2 announcements guard against.
