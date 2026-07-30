# Change record — 2026-07-29 → 07-30

Every change from this window: **what was broken, why we changed it, and what the
change demonstrably did in reality.** Written because a commit message says what
a diff does and a release note says what shipped — neither says whether the fix
actually worked, which is the only question worth asking later.

Ordered by evidence strength: things proven in production first.

---

## 1. Verification single-flight — `internal/verifylock` (v22.10.0)

**The issue.** Fleet lanes are separate OS processes. Each shells full package
suites during EGPS and the build handoff floor. Run concurrently on one host they
oversubscribe it, and long suites go red **while being green in the lane's own
preserved worktree**. Batch-16 died of this: `TouchedPackagesStayGreen` red on
cycles 1166/1167/1169, identical fingerprints, breaker halt.

**Why this fix.** The reds were not defects and not flaky *tests* — they were a
host-capacity artifact. Serialising the expensive verification is the only fix
that removes the cause instead of tolerating it. LLM phases stay parallel; only
verification is single-flight, hub-resident (git common dir) so every worktree of
the repo shares one lock.

**What it did in reality.** Fired in production the same day:
`[verify] single-flight: waiting 5s for the host verification lock (another lane
is verifying) — /Users/…/.git/evolve-verify.lock`. The hub path in that line is
the proof it is the shared lock, not the per-worktree fallback (which WARNs
loudly by design). No contention-class false red has been attributed to
EGPS-vs-EGPS since.

**Known residual.** Builders' own mid-build `go test` self-checks are NOT under
the lock, so whole-package predicates remain load-sensitive. That is why
`acs-metapredicate-suite-scope` (0.95) and `suite-family-flake-deflake` (0.96)
are still queued: the lock bounds the damage, predicate granularity ends it.

---

## 2. FAIL-side attempt accounting → `internal/cycleoutcome` (v22.11.0/.1)

**The issue.** Tasks ground forever. `workspace-hygiene` burned 12 attempts,
`quarantine-dead` 7 — with `failure_count: 0` on disk the whole time. The
ADR-0072 retry ceiling therefore never fired: it was structurally unreachable.

**Why this fix.** The release-path accounting only ran when a lane *claimed* into
`processing/`, which wave lanes never do. Worse, the console's first fix (#373)
wired the chain into `cmd_loop.go`'s **sequential** body — and production runs
wave mode, where lanes are child processes returning a bare exit code. So the fix
was unreachable exactly where the grinding happened.

**What it did in reality.** Two independent proofs:
- The loop *itself* built the better version (ADR-0079 `cycleoutcome.ApplyFailure`,
  a strict superset) in cycle-1180. The merge kept the seam and deleted the
  console helper — one closeout, two entry points.
- Live line from batch-21: `[inbox-mover] release-cycle: 1 file(s) released from
  cycle-1215`. A failed cycle released its claim instead of stranding it.

**What it did NOT fix, discovered by hand.** Two claims stranded *before* the
seam existed had been invisible to triage for weeks:
`contract-block-cli-escalation` since batch-19 — **which is why that class
recurred twice more while its own fix sat locked away** — and
`ship-stage-explicit-paths` for three weeks, already delivered by another path.
Nothing retroactively swept old strandings. `processing/` is now empty.

---

## 3. Red-predicate evidence capture (PR #374)

**The issue.** Three EGPS false reds (cycles 1173/1175/1178) were
**unconfirmable after the fact**: the 600-byte evidence excerpt elides the
*middle* of the inner go-test stream, and a red that stayed red on the bounded
retry looked identical to a red that was never retried.

**Why this fix.** Diagnosis was failing for lack of evidence, not lack of
analysis. Persist each red's complete stream beside the verdict, and record what
the retry established.

**What it did in reality.** First red of batch-21 was diagnosed from **one file
read**:
`acs-red-evidence/buildselfcheck__TestChangedPackagesToolchainGreen.txt` carried
the attribution header, `--- FIRST RUN ---`, `--- RETRY RUN (still red) ---`, and
`retry_outcome: red-on-retry`. Its fifth line named the cause. That would
previously have been an hour of workspace archaeology.

**Design decision worth keeping.** `flaky` still means *passed-on-retry only* —
`acs/cycle468` predicates pin that wire contract. The retry outcome went into a
NEW `retry_outcome` field. Never redefine a pinned field; add one.

---

## 4. Abort-cause distinguishers (PR #376)

**The issue.** Batch-19 halted at cycle-1208 on the diagnosability breaker: three
build aborts (1197/1199/1207) produced **no machine-readable reason**, collapsed
to one `build|unknown|7c02ce1f4f95` fingerprint. The abort's cause was in
`RunCycle`'s return error and simply never persisted.

**Why this fix.** Thread the named return into the deferred epilogue and append
the cause. Truncation keeps the **tail**, because error chains grow prefix-first
(`phase build: attempt 3: bridge: …: <root cause>`) — a head-kept cut collapses
two different roots under one long shared prefix.

**What it did in reality.** Batch-21's halt became **the first in this project's
history readable straight from the digest**:
`cycle aborted in phase build (abnormal-exit epilogue) cause=…absent from
go/.apicover-enforce — graduate each …`. That one line produced the
`acs-apicover-enrollment-in-builder-brief` item (0.94) and PR #383.

**Deliberate, pinned decision.** Teardown-shaped causes are marked `teardown=`
and *do* accumulate in the identical-fingerprint population — one infra
condition mowing down N lanes is the recurring-infra shape ADR-0072 wants
stopped. Reviewed, argued, and pinned so it is not later "fixed" as a false trip.

---

## 5. Write-in-flight grace window (PR #378)

**The issue.** The contract gate rejected a **provably valid** deliverable three
times in cycle-1198 — only `[bad_verdict]`, sections passing. I ran the
production parser against the live file: it parsed clean. The gate had verified
different bytes than the final file.

**Why this fix.** A verifier can observe ENOENT or a zero-length file for a
deliverable that *is* being written, and one unretried read cannot distinguish
that from "never written". Absence/emptiness is now provisional for 500ms:
read-first (the common case pays one `os.ReadFile` and zero sleeps), retry only
absence/blank, surface non-absence IO immediately as infra. It delays a verdict;
it never changes one.

**Honest limits, stated in the doc it ships with.** It does NOT close the
observed cycle-1198 signature — partial-but-non-blank content, sections present
and the trailing sentinel not yet appended. That needs cross-poll artifact-ready
stability in the bridge, queued as `artifact-ready-crosspoll-debounce` (0.92)
**with the reason it was rolled back**: it changes the tick contract for every
artifact-completion fixture and cannot complete inside tests with a 2-second
artifact timeout.

---

## 6. FAIL dossiers carry their failure identity (PR #379)

**The issue.** A FAIL dossier said "see audit-report.md". The knowledge base
recorded *that* a cycle failed, never *why* — so convergence briefs and
cross-batch forensics required archaeology in gitignored runtime.

**Why this fix.** The identity already exists on disk at dossier time. Project
it: fingerprint, pre-class, up to five reason heads. FAIL-gated so PASS/WARN
dossiers keep their exact byte shape.

**Review findings that mattered more than the feature.** Reasons are built from
test-output excerpts, so a newline-bearing reason rendered a **fake `## `
heading** into the committed markdown; whitespace now collapses before the byte
bound. And the digest's own `cycle` must match, or a stale digest from a failed
write commits a **false** forensic identity — worse than an absent one.

---

## 7. Deep-tier artifact budgets + self-describing timeouts

**The issue.** Six phases died `missing_artifact` in one day across four phase
types — audit ×2, retro, adversarial-review, tdd ×2 — **one on a provably quiet
host**, which is what ruled out contention as the sole cause. Arithmetic: base
wait 300s + 6 extends ≈ 650s, and only `retro` was exempted at 900s. A deep-tier
agent doing genuine analysis on this repo needs more than eleven minutes.

**Why this fix.** Raise only the phases with demonstrated deaths, to 1200s.
Every other phase stays on the 300s builtin — the compiled default's own comment
says the narrow exemption exists so global hang detection is not broadly
weakened, and that intent is preserved and pinned.

**Second defect found en route.** The exit-81 cause came from a helper that
returns the last non-empty line when it sees no `[bridge]` prefix — but tmux
drivers prefix `[<cli>-tmux]`, so **a dead phase's recorded error message was a
filename**. Proven by disabling the fix and watching the cause become an
unrelated sandbox warning.

**A claim of mine that was FALSE.** My filed item asserted
`runtime-reference.md` publishes the wrong config key, copied from a `state.json`
string I never verified. Refuted by reflecting the struct tag against every doc
mention. The stale default cell and misleading wording *were* real and are fixed,
plus a reflection-derived drift pin that went RED the moment the defaults
changed. **Lesson: do not file unverified claims.**

---

## 8. Contract-gate CLI escalation

**The issue, twice confirmed.** A phase whose deliverable fails its contract is
re-dispatched to the **same** CLI forever — `cli_fallback` fires only on infra
exit codes. Batch-18: agy failed the adversarial-review contract 7/7 across
corrections, and the circuit demoted the gate enforce→advisory in *both* lanes.
Batch-21 cycle-1215: the same class on **triage**, corrections exhausted, cycle
lost on the top-priority lane.

**Why this shape.** Escalate only the re-dispatch that already failed, never the
phase's primary — the phase ships fine on its primary for the 99% PASS path.
Format adherence turned out to be a **model property past some point, not a
prompt property**: corrections re-issued verbatim to agy converged 0 times in
seven; claude complied.

**Review found 5 real defects, all fixed.** Two were severe: escalation was a
no-op for *minted* phases including adversarial-review — the original evidence —
because the agent-name map covers only the ten spine phases; and
`ChainReviewers` dropped `Demoted` when a later gate rejected, which would have
left the live triage path permanently demoted with zero evidence.

**Bonus finding.** `dispositionrouter.StageIntent` had **no production caller**
before this change. Tests only.

## 8b. The escalation crossed transport — and my first diagnosis was wrong

**Delivery note.** The first PR carrying sections 7 and 8 was cut from a base
that predated the persona and e2e work, and it went red on both platforms. I
diagnosed **both** failures as stale-base artifacts. Half of that was right and
half was a claim I had not earned, so it is worth recording exactly which half.

**The ubuntu half was stale base.** `internal/prompts`'
`TestPersonaStopCriterionDedupe_CombinedLineCountReduced` (the persona line
budget is a hard ceiling, and two independent branches each spent part of it)
and `internal/core`'s `TestGuardRecoversCatalogWritesSourcePhaseLeak` fail on
the *merge* of that stale base against current `main` and pass on a clean
rebuild of the same two commits. That diagnosis held.

**The macOS half was a real defect in section 8's own feature**, and my local
run said "green" because **this machine has tmux and the CI runner does not**.
`TestE2ECLIFallbackChain/trigger_81_falls_back_to_codex` failed with
`build correction 2 dispatch failed: launch exit=10: [codex-tmux] new-session:
exec: "tmux": executable file not found`.

Read the two lines together and the defect is unmissable:

```
[runner] phase=scout  dispatch chain: claude-p@sonnet=81 -> codex@sonnet=0   # fallback → driver "codex"
[engine] ... driver "codex-tmux" ... (agent build)                            # escalation → driver "codex-tmux"
```

**The same string, `codex`, resolved two different ways inside one phase's own
chain.** `llmroute.ApplySoftOverlay` normalized the overlay CLI through
`defaultDriverForFamily`, which maps a bare family to its *default* driver —
correct for a family name, wrong for a name the chain already held as a
concrete, headless driver. So a contract block on a headless phase silently
re-dispatched it onto tmux: a hard `exit=10` cycle failure where tmux is absent,
and where tmux is present, a silent move to a different transport with different
cost, cadence, and quota behaviour that nothing reports.

**The fix** is a two-step resolution with the order stated: if the plan's chain
already contains `ov.CLI`, promote **that exact entry** — the chain was resolved
for this phase and its entries are drivers the phase can actually run; only a
CLI the chain does not name falls through to family normalization. Both existing
overlay tests stay green unchanged (their chains are all-tmux, so the family rule
still fires), and the new pin fails on the pre-fix code.

**The lesson is the same one section 7 already records, one section later.** A
green local run is evidence about the local host, not about CI. When a CI-only
failure is attributed to something else, the attribution needs a reproduction
under CI's constraints — here, re-running the suite with `tmux` removed from
`PATH`, which takes one command and settles it.

---

## 9. House rules reach the agents (PR #383, merged)

**The issue.** Three separate queue items, one root cause: mandatory house rules
lived in operator lore, so every cycle rediscovered them by failing.
- batch-21 **halted** because three lanes' builds aborted on "new internal
  package absent from `go/.apicover-enforce`";
- console hit the identical class twice the same day on PR #372;
- two console implementer subagents both satisfied their stated contract while
  leaving integration unwired — a struct field with no consumer, and a lint with
  no caller at all.

**Why this fix.** The agents cannot follow a rule they never receive. Personas
now carry the two-edit apicover graduation and the caller-proof requirement; the
build floor's abort reason emits the exact two edits instead of naming the
offense; and the machine contract is restated at the **generation point**,
because Claude follows user-turn instructions better than system-message ones —
evidenced in our own logs by correction prompts (turn tail) getting compliance
that the persona body did not.

**Review caught the change's own worst bug.** The new tail block showed the
auditor a bare **PASS** exemplar — but audit's FAIL/WARN verdict must carry a
structured failure block, and the tail is the recency-dominant copy. The version
the auditor would actually follow omitted the requirement, on the one phase whose
verdict gates ship.

**Line budget was earned, not raised.** Combined scout+builder+auditor is 749
against a hard `<751` ceiling, bought by tightening four prose blocks.

---

## 10. Runtime/console plane separation — ADR-0080 (v22.9.0–v22.10.0)

**The issue.** Batch-15 lost four lanes (1149–1152) to operator activity in a
shared checkout: console edits tripped the tree-diff guard, and a `git rm` of
runtime-minted stubs removed them from disk and killed dispatch.

**Why this fix.** Make the class structural rather than behavioural. The loop
owns a dedicated linked worktree; the primary checkout is the operator console;
cross-plane artifacts live in the git common dir; integration happens only via
origin.

**What it did in reality.** Every loop boot since prints
`[loop] plane: linked worktree (runtime plane) branch=main`, and no lane has been
lost to console activity since the cutover — including during a day of heavy
console work alongside running batches.

---

## Cross-cutting: what actually caused failures this window

| Cause | Count | Note |
|---|---|---|
| Harness defects (I/O, wiring, config, diagnosability) | ~6 | Matches the literature's ~65% harness-not-model finding |
| Environment/contention flakes | ~20% of cycles | Being closed by the 0.95/0.96 pair |
| Honest task defects that converged | ~55% | 2–4 generations to ship; the machinery working |
| **A leaked test harness** | 14 cores × ~19h | Two batches degraded; see below |
| False-PASS (shipped broken work) | **0** | Soundness never compromised |

**The leaked harness deserves its own line.** A deflake reproducer spawned `yes`
load generators and never reaped them. Eight burned 8 cores for nine hours; my
first sweep killed only the PIDs it had listed, and **six more survived another
ten hours**, degrading all of batch-20 and batch-21's first three. They are what
starved the deep-tier phases in cycles 1216/1217/1218. Constraint now recorded on
the item: load generation must be context-bound and distinctively named so a
sweep can target it by name.

---

## 11. `TestE2ECLIFallbackChain` — the red that blocked the release (FIXED)

**The issue.** All four subtests (80/81/124/127) failed on `origin/main`. It runs
only in CI's macOS e2e tier, so it went unnoticed. It is load-bearing: the only
automated proof that the CLI fallback chain — what the whole
any-CLI-any-phase invariant rests on — works end to end.

**Why it took real digging.** The surface lied twice. One run showed
`roles=[orchestrator intent]`; another showed **no ledger at all** and six lines
of output. Neither is a fallback bug. Three hypotheses were eliminated with
evidence before the cause appeared: the trigger set is correct
(`resolveTriggers` → `{80,81,85,124,127}`), the fixture's bare `claude-p`/`codex`
driver names are valid (both manifests exist), and the fixture's stale
`model_tier_default:"sonnet"` is irrelevant (tested, reverted rather than left
as unverified noise).

The answer came from running the child process under a diagnostic harness that
**streamed its output to a file** instead of letting the test discard it on
timeout. Two compounding causes:

**Cause 1 — the fixture inherited a live model-catalog refresh.** The compiled
default is `AutoRefresh=true` ("the cycle-start live refresh is on"). Production
disables it in `.evolve/policy.json`; the fixture seeded **no policy at all**, so
every `evolve cycle run` probed **every CLI family** before its first phase. From
the child log: `[recipe] launching: agy`, two fake-CLI boots, then `ollama`, each
burning a `/model` picker wait that can only time out against a fake REPL. That
startup cost alone exceeded the budget, so the cycle was killed before reaching
ship — failing on all four codes for a reason unrelated to fallback.

**Cause 2 — the spine outgrew the budget.** With startup fixed, the chain works,
but reaching ship costs ~10 minutes: `intent → scout → triage → plan-review →
tdd → build-planner → build → builder → tester → audit → auditor → ship`, each
phase paying a primary failure **plus** a fallback. Half those phases did not
exist when the 120s budget was written. Four in parallel never had a chance.

**The fix, by test layer.** Per-code trigger semantics moved to
`llmroute.TestDispatch_FallsBackOnTriggerExit`, now **table-driven over
`defaultFallbackOnExit` itself** — a newly added trigger code is covered
automatically, in microseconds. The e2e keeps the one thing only an e2e can
prove: a real cycle, every phase falling back, still reaches ship. The workflow's
e2e step gained `-timeout 30m` because Go's 10-minute default was itself killing
the package mid-cycle.

**And the rot-proofing, which matters more than the fix.** The harness used to
run the child for a fixed wall-clock budget and then check how far it got — so it
passed only because the cycle *happened* to reach ship before an arbitrary kill.
That is exactly how it rotted the first time. It now **polls the ledger for the
role it is asserting and stops the moment it appears**: a fast host finishes
sooner, a slow host still passes, and the result depends on the invariant rather
than the machine. The generous ceiling remains as an upper bound, not as the
thing being measured.

**Lesson worth keeping.** A fixture that seeds *nothing* silently inherits every
future default. This one predated the cycle-start catalog refresh, production got
the off-switch, and the test never did — so a subsystem it does not test grew
into its critical path. Fixtures should pin the defaults they depend on.

---

## Was still red, now fixed — release gate

`TestE2ECLIFallbackChain` fails all four infra-exit triggers (80/81/124/127) on
`origin/main`. Reproduced on a clean detached checkout, so it is **not** caused by
any change here. It runs only in CI's macOS e2e tier, which is why it went
unnoticed.

It is load-bearing: it is the only automated proof that the CLI fallback chain —
the mechanism the whole any-CLI-any-phase invariant rests on — works end to end.
**While it is red, every fallback claim in this repo is unverified.** Filed as
`e2e-cli-fallback-chain-red-on-main` (0.94) with three hypotheses ruled out
(trigger set correct; driver names valid; the fixture's stale
`model_tier_default:"sonnet"` is not the cause) and the remaining leads ordered.

**UPDATE (same day): fixed — see §11 above.** Root cause was a fixture that
inherited the live model-catalog refresh, compounded by a spine that had roughly
doubled since the 120s budget was written. The release gate is cleared once the
fix lands; no release was cut while the invariant was unproven, which was the
right call at the time.

**Merged as `2bdccc86` (PR #386).** `main` went green on both the `go` and `CI`
workflows at that SHA, and macOS e2e stayed green across every PR that followed
(#387, #388, #389, #390). This is what made v22.12.0 cuttable; §12–§17 below are
the work that landed on top of it.

---

## 12. `IsShippingVerdict` earns its export (PR #387, `0ad3bf48`)

**The issue.** CI `apicover -enforce` went RED on `internal/core`: 282 of 283
exported symbols covered, with `IsShippingVerdict` "uncovered — no test names
it". The symbol had been exported in that same branch *specifically* so the
throughput hook and the loop's non-progress breaker would share one definition
of "this cycle landed work".

**Why the obvious fix was the wrong one.** A test that simply calls the function
satisfies the gate and proves nothing. The export exists to prevent a fork
between two consumers, and nothing asserted the consumers agreed.

**What shipped instead.** Three tests. The vocabulary walk covers all six
declared outcome labels plus `SKIPPED`, `""`, `"pass"` and `"SHIPPED"` —
asserting the *allowlist* shape, because `SKIPPED_UNKNOWN` once fell through a
denylist-shaped breaker and was counted as progress. The coupling test asserts
`shippedOutcome(v, HEAD-moved) == IsShippingVerdict(v)` for every label, so the
in-package consumer cannot drift. And `cmd/evolve` gets the caller proof for the
cross-package consumer with an anti-tautology half: the two shipping labels must
never advance the non-progress streak.

**In reality.** `internal/core` reads 283/283, and a future outcome label cannot
reach the throughput window without also reaching the breaker.

---

## 13. Four queued items, two of which did not reproduce (PR #388, `b21b7010`)

Recorded honestly rather than padded — a fix report that claims to have fixed
what was filed, when it fixed something else, is how a queue accumulates
phantom-closed items.

**`spine-failopen-telemetry` (0.85) — the per-cycle half already existed.**
Cycle-1166 landed the dossier field. What was missing:
`dossier.RollupSpineFailOpens` had **no production caller**. The batch-level
count — the actual ask, "76 fail-opens in one batch is an epidemic without a
dashboard" — was dead code. It is now wired into `loopResult.emit`.

The first implementation would have reported zero for exactly the batch shape
the item was filed about: it sourced only `lr.Cycles`, which fleet wave and pool
lanes never append to, so at the standing width of 3 the block would have been
permanently absent. The fix folds **committed dossiers** for cycles at or after
`batchFirstCycle`, unioned with in-memory results, dossier winning per cycle. A
window of 0 means *unknown* and reads no corpus at all, so an all-time total can
never be misreported as one batch.

**`guards-role-hermetic-home` (0.85) — the filed escape does not exist.**
`role.go`'s `&& !IsProtectedSurface(path)` already closes the claimed C1 hole.
What *was* real is worse in a quieter way: `isAlwaysSafe` read `$HOME` raw, and
the test's fallback meant every `$HOME/.claude` assertion silently exercised
`/tmp` — the test proved nothing about the guard it named. Home is now injected
at the composition root, `isAlwaysSafe` is pure, and an empty home disables the
rule rather than guarding `filepath.Join("", ".claude")`.

**`triage-cards-carry-files` (0.89) — plus a gate-integrity find.** The consumer
side was already correct since #366; the de-facto writer `ProjectDecisionJSON`
emitted only `{id, action}`. Cards now declare `files=` and carry them into the
companion instead of having them inferred from prose. The unfiled defect found
while doing it: floor scans ran on the **raw item text**, so a footprint that
merely mentioned `floors.go` flipped a non-floor card into a floor-bearing one —
one phantom committed floor, which clamps triage capacity. All floor scans now
route through `floorItem`.

**`flaky-predicate-authoring-lint` (0.93) — unlandable because it had no
caller.** It is now `evalgate` Gate D, firing at the *end* of the tdd phase,
which is the first moment `go/acs/cycleN/predicates_test.go` exists. Advisory by
construction: `block=false` is a constant, and a monotonic `maxEvalLevel` join
replaced an `&& overall == LevelPass` conjunct, so the gate can never *lower* a
HALT. Corpus false-positive work is measured, not asserted: 341 → 179 findings
via argv-position resolution and `-run` suppression. `internal/gopkgpattern` was
extracted as a shared leaf (it had been duplicated in `acssuite`) and graduated
in `.apicover-enforce`.

**Review findings applied.** The gate interface now documents that **`block` is
the violation signal and a non-empty `reason` is not** — without that, a future
"simplification" keying on `reason != ""` would fail every tdd phase. A dead
`failedCycleCommittedIDs` forwarder that a stale implementer base had
re-introduced was removed: the very no-caller class this change fixes elsewhere.
The `-run` suppression's promotion preconditions are recorded at the code site,
because two of them are still open (the value is never inspected, and
runtime-built patterns are invisible) and both must close before enforce.

---

## 14. Read the verified bytes, not the file again (PR #389, `2d6b297a`)

**The issue.** `phases/runner` verified a deliverable and then re-read the same
path twice more to classify it. Between verify and classify the file can change,
so the gate's verdict and the content the pipeline acted on were not provably
the same bytes.

**Why this fix.** `deliverable.Result` now carries `Content` (`json:"-"`,
populated once after the verified read, building on the landed grace window
rather than duplicating it), and both re-reads are gone.

**What the item did not know, and what nearly broke.** The two reads were not
always of the same path. `intent` in DELTA mode dispatches `intent-delta.md`
while the intent phasecontract names `intent.md`, so `Verify` judges a different
file than the phase was asked to write. A naive "just classify the verified
bytes" FAILed every intent-delta cycle. `classifiedArtifact` keys on
`res.ArtifactPath == artifactPath`: fast path uses the verified bytes, otherwise
it falls back to the pre-existing read of the dispatched artifact. **That skew
is itself a defect** — the contract gate verifies a file the phase was never
asked to write — and it is queued rather than silently absorbed.

---

## 15. Cancellation that does not resurrect a false-FAIL (PR #389, `2d6b297a`)

**The issue.** `verifyReconcileDeliverable` slept unconditionally: 16 sleeps and
16 probes on an already-dead context.

**Why the straightforward fix was blocked, and rightly.** Honoring cancellation
at the *teardown* call site regresses the cycles-824/825 false-FAIL class —
`bridge/driver_tmux_repl.go` launders a ctx-cancel into `ExitArtifactTimeout`,
and its own comment names the settle ladder as "the only thing standing between
that mislabel and a false FAIL". The adversarial reviewer caught this and the
BLOCK was accepted.

**What shipped.** The teardown site passes `context.WithoutCancel(ctx)`; the
ctx-aware bail is scoped to the clean-exit path only, where the agent exited 0
and nothing more is coming. Post-cancel cost is bounded at one 200ms interval
with no further probe. Pinned by a test that fails without `WithoutCancel`.

---

## 16. A dossier that contradicted its own artifacts (PR #389, `2d6b297a`)

**The issue.** `knowledge-base/cycles/cycle-1193.json` recorded
`skipped_phases:[{phase:retro,reason:FAIL}]` while
`.evolve/runs/cycle-1193/retrospective-report.md` sat on disk. The retro had
run. The durable record said it was skipped — and the durable record is what
cross-cycle consumers read to learn which judgment phases executed.

**Why it happened.** The cycle-802 floor guard declines a post-verdict non-floor
phase's verdict rather than letting it clobber a floor-derived `FinalVerdict`.
That declining was recorded by writing into `SkippedPhases`, conflating "did not
run" with "ran, verdict not adopted".

**The fix.** A new `VerdictNotAdopted{Phase, Verdict}` and
`CycleResult.VerdictsNotAdopted`; `recordFinalVerdict` appends there;
`SkippedPhase`'s doc narrows to real skips. One record, one projection —
`phases_run_verdict_not_adopted` in the dossier, `omitempty`, so a cycle that
declined nothing keeps its exact byte shape.

**Honest residual.** Roughly 90 historical dossiers still carry the mislabel and
the fix is forward-only with no version discriminator, so the disposition
assembler still reads a partly-poisoned corpus. Stated rather than quietly
carried.

---

## 17. The drift test ADR-0055 promised and did not have

**The issue.** `schemas/cycle-dossier.schema.json` declares
`additionalProperties: false` and was missing **four** top-level keys and
**seven** `PhaseRecord` keys. Any external tool that took the committed schema
at its word would reject every real dossier. ADR-0055 states the Go struct is
the SSOT and that "the `TestSchema_NoDrift` drift test guards this in CI" — that
test did not exist, which is precisely why eleven fields rotted unnoticed.

**Why a fourth key was not the fix.** Adding the new key and leaving the other
ten would have been the fifth instance of the same class.

**What shipped.** The schema was repaired (four top-level properties, five new
definitions, seven `PhaseRecord` fields), and
`go/internal/dossier/schema_drift_test.go` now implements the promised check —
**bidirectionally**, via a registry mapping every schema object to its Go type.
One-way in either direction is insufficient: "every Go field appears in the
schema" lets a removed field linger forever, and a schema-side-only check lets a
new Go field rot exactly as these did. A definition present in the schema but
absent from the registry also fails, so a new nested type cannot go unchecked.

**Proof it works.** Run against the pre-repair schema, it reports exactly the
drift — 5 missing definitions, `PhaseRecord` missing 7 fields, root missing 4 —
and it is green after. An anti-no-op twin proves the comparison would catch a new
field rather than passing because both sides read from the same place, and that a
`json:"-"` field is correctly excluded from the wire surface.

