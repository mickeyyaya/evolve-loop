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

## 7. Deep-tier artifact budgets + self-describing timeouts (PR #384, open)

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

## 8. Contract-gate CLI escalation (PR #384, open)

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

## Still red, still open — do not release past this

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

No release was cut. The preflight would have attested to something known false.
