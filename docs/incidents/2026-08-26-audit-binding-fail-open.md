# 2026-08-26 — Ship's independent-review binding fails open across cycles (cycle-1571 H3)

## What happened

Cycle 1571 (verification wave 2) FAILed its audit — a legitimate agent-graded
FAIL with a well-formed sentinel. Ship then errored `AUDIT_BINDING_HEAD_MOVED`
with `audited=e8003a44` — **cycle 1570's** HEAD, not 1571's. The recovery path
declined repair and forced a full deep-tier re-audit of a cycle already
adjudicated FAIL: a burned lane slot. The re-audit (dispatched at xhigh)
produced the H3 finding this document records.

## The hole — two halves composing

**Producer half.** `core/phase_bindings.go` recorded the rich auditor ledger
binding (role=auditor, kind=agent_subprocess, artifact SHA, run_id) only on
`PASS|WARN`. A FAIL verdict emitted **no** binding: the FAIL was the very
thing that removed the ship gate's ability to see it.

**Consumer half.** `phases/ship/audit.go` `findLatestAudit`, when
`opts.RunID` was set and no entry matched, fell back to the newest auditor
entry from **any** run ("zero regression for legacy unstamped ledgers"). With
the producer half above, that fallback is reached on *every FAILed cycle*, and
under fleet concurrency the newest entry belongs to a sibling lane.

**Worst case (all preconditions live, not hypothetical).** If the sibling's
entry shares `git_head` with current HEAD — routine when two lanes run inside
one HEAD window — exit-code, artifact-SHA, and verdict checks all evaluate the
*foreign* artifact. A FAILed cycle ships on a sibling's PASS with no error
surfaced. Same failure shape as the 2026-05-29 "ancient bash-era auditor
entry" incident, reachable by a different route.

## Fix (fail closed at both halves)

- **Producer** (`phase_bindings.go`): FAIL now records the same rich binding,
  `exit_code=1` (Unix findings convention — the auditor process completed;
  severity lives in the bound artifact, so ship's `0|1` exit gate passes and
  the verdict parse of THIS run's report returns the honest terminal
  `AUDIT_BINDING_VERDICT_FAIL` instead of a foreign `HEAD_MOVED`). FAIL is
  ledger-bound but **never** projected into the verdict cache (its consumers
  were designed against PASS|WARN content; a FAILed tree re-audits).
- **Consumer** (`audit.go`): a run-scoped lookup miss is now a hard
  `AUDIT_BINDING_NO_AUDITOR` integrity stop naming the refused foreign entry.
  The old fallback pin `TestFindLatestAudit_RunIDNoMatch_FallsBackToLatest`
  was deliberately FLIPPED: its unstamped-ledger premise is dead (every
  current recorder stamps `run_id`; verified against the live ledger).

- **Third reader** (review sweep finding): the composition snapshot's
  `latestAuditEntry` (`cmd/evolve/cmd_composition_wiring.go`) — the RUNG 0
  trivial-rebase carry-forward's ledger reader — had the same unscoped
  "latest auditor entry" shape, previously safe only via
  `findCompositionVerdict`'s LaneAuditRef hash cross-check. Now run-scoped
  with the identical contract (runID set ⇒ exact match or error), because the
  producer fix makes FAIL entries visible to it for the first time.

## Lessons

1. A verdict is only as enforceable as the *binding* that carries it to the
   gate; excluding the failure verdict from the binding path inverts the gate.
2. Cross-run fallbacks in integrity lookups are fail-open by construction;
   "zero regression" compatibility clauses must carry an expiry condition.
3. The escalation's canned `repro_hint` was wrong for the sixth consecutive
   time; the re-audit's artifact-first decomposition found the real hole.

---

## Follow-up (2026-08-27) — three HIGH defects in the fix itself

The fix above shipped with a language review and a simplifier pass but **not**
the adversarial architect review that operating-policy §3.2 requires for
architecture changes. Run after the fact, that review found three HIGH defects.

### H1 — the fix's central premise was false

**Issue.** #503 removed the cross-run fallback on the stated grounds that "every
current recorder stamps `run_id`". Three of the four `agent_subprocess` ledger
writers did not: `subagent/run.go`'s hand-built JSON literal,
`subagent/subagent.go`'s `LedgerEntry`, and `cyclesimulator`'s map. Only
`core/phase_bindings.go` stamped, and only because it appends through the
Orchestrator's `stampingLedger` — which the out-of-process writers cannot reach.

**Gap.** The premise was verified *empirically*, by reading recent ledger rows —
all of which happened to come from the one writer that does stamp — instead of
*structurally*, by enumerating the writers. An `evolve subagent run` auditor
entry (the sanctioned manual re-audit) could therefore never satisfy a
run-scoped lookup: ship hard-stopped `AUDIT_BINDING_NO_AUDITOR` where it had
previously bound and shipped.

**Solution.** `core.RunIDFromWorkspace` resolves the identity from the run
workspace's `run.json` — already on hand at every writer — and all three
unstamped writers now populate it. The durable half is a class guard
(`ledger_runid_writers_guard_test.go`): the writer set is closed, and every
non-centrally-stamped writer must call the resolver, so a fourth writer cannot
be added silently. Read fail-soft, consumer fail-closed: an unresolvable id
omits the key rather than stamping a blank identity.

### H2 — the fix named a hazard and closed half of it

**Issue.** `latestAuditEntry`'s own comment said an unscoped lookup "can be a
sibling lane's — *or a FAILed* — audit". The filter added below it checked
`Kind`, `Role`, `GitHEAD`, `RunID` — no verdict. A FAILed audit from *this* run
still bound, so RUNG 0 carry-forward ran the entire composed-tree gate set and
wrote a `composition-verdict` record certifying the carry-forward of a rejection
into a hash-chained ledger.

**Gap.** `exit_code` is 1 for WARN and FAIL alike, so the entry alone cannot
carry severity; the verdict lives in the bound artifact, and nothing read it.

**Solution.** `requireReusableAudit` reads the bound artifact, parses its verdict
sentinel, and refuses anything outside `verdictcache.Reusable` (PASS or WARN) —
positioned before any git work, so a rejection costs nothing. Deliberately no
fallback to an older PASS: if this run's newest audit says FAIL, carry-forward
declines rather than reaching behind it.

### H3 — the widened seam had no wiring proof

**Issue.** #503 widened `WithCompositionSnapshot` to carry `cs.RunID` and nothing
asserted the value arrives. Every fixture either called `latestAuditEntry`
directly or used a stub that ignored its arguments.

**Gap.** Unit-green, live-dark: replacing `cs.RunID` with `""` at either call
site left the whole suite green while the hardening went permanently dark — the
same shape as the cycle-1064 `Options.ManifestGate` trap the ship package
documents in its own comments, and a §3.3 violation.

**Solution.** A pin that drives the real recovery path, captures what the closure
actually receives, and compares it to the RunID the orchestrator persisted —
with "never called" failing distinctly from "called with the wrong value" so it
cannot pass vacuously. Verified by mutation: neutering either call site reds it.

### H4 — the release gate, blocked by an unrelated lane

**Issue.** The release preflight takes the newest auditor entry with an on-disk
artifact — no run, cycle, or `git_head` filter — and a non-acceptable verdict
aborts `evolve release`. Before #503 a FAILed cycle wrote no auditor entry, so
that branch was unreachable; #503 made FAIL entries exist, so the newest is
routinely a FAILed lane cycle with no bearing on the release commit. Observed
live: the runtime ledger's newest auditor entry was cycle-1574, exit 1, artifact
present, verdict FAIL — `evolve release` was blocked by an unrelated failure.

**Gap.** The step was already inconsistent: a *missing* audit is advisory-skipped
because "CI-green on the release commit is the authoritative gate", while a
*failed* audit of unrelated work hard-blocked.

**Solution.** Scope the veto to what was actually examined — using TWO
discriminators, because head equality alone is not enough.

The first draft of this fix scoped on `git_head` alone, on the assumption that a
lane audit binds a lane worktree head. **That assumption was false**, and the
architect review caught it before merge: `recordAuditBinding` resolves `git_head`
with `rev-parse HEAD` against the PROJECT ROOT, so every concurrent lane records
main's tip. Verified in the runtime ledger — cycles 1572, 1573 and 1574 all carry
`git_head 31ae6518`, each with a distinct `worktree_tree_sha`. Scoping on head
alone would still have vetoed the very release that motivated the fix.

The second discriminator is the one the ledger already carries, and it is a
*writer* marker rather than a delta marker: the orchestrator's binding recorder
runs `git add -A; git write-tree`, which returns a tree even for a clean
worktree, so **every** cycle audit records a `worktree_tree_sha` — while the
manual `evolve subagent run auditor` writer never emits one. Presence therefore
identifies which writer produced the entry, which is exactly the needed cut: a
manual audit of the released commit keeps its veto. (Attach this to any future
change that stamps `worktree_tree_sha` on the manual writer's entry: it would
make release audits indistinguishable from cycle audits and re-open this hole.) So a failing audit is
advisory when it bound a different commit **or** when it audited uncommitted
work; it still blocks when it examined this commit's committed tree. An
unresolvable HEAD keeps the conservative block, so the scoping is never a bypass.
The scoped-out disposition is its own value (`SCOPED_OUT`), not "no audit", so
the operator log names the artifact and both commits instead of claiming an audit
that exists and failed is absent.

### The premise error, repeated

Worth recording plainly: H4's first draft failed the same way H1 did — a premise
asserted from plausibility rather than checked against the data, in the very PR
whose Lesson 4 is about that. What caught it was not more care; it was running
the adversarial architect review that §3.2 requires and #503 skipped, before
merging rather than after. The review also found two CRITICAL Go defects in the
same diff (a `%` in a run id corrupting the whole ledger line via a format-string
splice, and a `run_id` computed then silently dropped by a serializer allowlist),
neither of which any test in the first draft would have caught.

## Lessons (follow-up)

4. A premise stated in a commit message is not a verified premise. "Every writer
   does X" is a structural claim and needs a structural check — ideally the
   check becomes the guard, as it did here.
5. A comment that names a hazard is evidence the author saw it; if the code
   below closes only part of it, that gap is more likely, not less.
6. Widening a seam and proving the widened value arrives are two changes. The
   second is the one that survives a refactor.
