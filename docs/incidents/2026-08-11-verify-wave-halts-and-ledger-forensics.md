# 2026-08-11 — Verify-wave halts (closure-claim false-RED, ACS wrong-root) and the ledger chain-break forensics

**Status:** all three defect classes CLOSED (#448, #449, #450 merged; live-proven).
**Context:** a 6-cycle verification wave (cycles 1430–1435, fleet width 2, codex quota-benched) run after the persona-strip-lobotomy campaign (#434–#447) to validate the rebuilt pipeline. The wave did its job twice over: 3 ships landed cleanly, and each of its two ADR-0072 halts self-diagnosed a real, previously-latent pipeline defect with surgical evidence — the halt/escalation machinery built in #440–#442 is what made both root-causes one-read findable.

## Problem

Three independent defects surfaced in one arc:

1. **Cycle-1431 halt (infra-systemic):** the audit's deterministic closure-citation gate force-FAILed a narrative-PASS audit because unbounded `strings.Contains(line, "closed")` matched the substring inside **"disclosed"** — on a line that literally ended *"still open"*. Prior firings of the same class: cycles 1339, 1371, 1428.
2. **Cycle-1434 halt (infra-systemic):** `evolve acs suite` red'd 3 predicates that the audit phase's own run showed 8/8 green. The CLI derived `EVOLVE_PROJECT_ROOT` via `git --git-common-dir`'s parent — the **owning repo**. Correct when a cycle worktree's owner *is* the plane; wrong since the runtime plane became a **linked worktree** of the console repo (all linked worktrees share one common dir, so the walk skipped the plane and landed on the console checkout, whose `.evolve` has none of the cycle's runs). The wrong-root CLI artifact then **suppressed** the phase's correct-root generation, because the verdict-exists gate honored any pre-staged file.
3. **Ledger chain breaks (~per cycle since mid-July):** attempting the queued console-plane rebaseline exposed both halves of item `ledger-fleet-concurrency-chain`: (a) `inboxmover.writeLedger` raw-`O_APPEND`ed **unchained** NDJSON (no `prev_hash`, no flock, no tip update) into the hash-chained `ledger.jsonl` on essentially every cycle's ship.postship — the per-cycle break generator (~180 dense console breaks; the runtime plane re-broke at line 115374 *during* this very wave); (b) `Rebaseline` chained its seal from `ledger.tip`, but `sealChainsFromPrev` validates the **physical** predecessor — with a foreign line past the tip, the seal bound the wrong line and the command failed on exactly the damage class it was built for (live error: *"seal appended but the chain still does not verify forward"*).

A fourth operational finding, not a defect: cycle-1430's ship stranded locally when origin moved mid-cycle (#446/#447 merged during the wave). The #446 recovery flow got its **first live validation**: one-time operator journal seed for 12 pre-journal loop commits → `evolve ship --push-only` pushed 24 attested commits (12 journaled + 12 structurally-provenanced sync merges).

## Hypothesis → verification

- **1431:** the halt's own evidence chain (`audit-fail-reason.json` FailReasons + verdict-conflict record, both landed in #442) quoted the exact offending line and named `closure_claim.go:46-47`. Reproduced RED at unit level with the live line shape before fixing. Note: the halt's templated `root_cause` field *guessed* "verdict-surface forged a verdict" — wrong; the deterministic gate was the cause. The evidence fields, not the guess, were what mattered.
- **1434:** the retro verified the topology live (`.git` is a `gitdir:` pointer; `git-common-dir` yields the console repo; the cited baseline exists only under the runtime checkout) and named all three code seams. Reproduced RED with a nested-worktree git fixture (console → plane worktree → sibling cycle worktree).
- **Ledger:** forensics on the failed console seal found the physically-last line was a foreign `class:"inbox-lifecycle"` record (no `prev_hash`/`entry_seq`) while the seal chained from the tip's last *chained* line — reproduced RED with a foreign-tail fixture that emitted the live error verbatim.

## Solution

| Defect | Fix | PR |
|---|---|---|
| Closure-claim substring/negation false-RED | Word-bounded matchers (`\bclosed\b`, `\bverified closed\b`) with a **two-rung posture**: the STRONG rung ("verified closed") is never guard-suppressed (the original anti-gaming record holds — an appended "…still open" must not become a one-token bypass); the WEAK rung (bare "closed" + cycle-ref) accepts negation/openness guards, revised on the evidence of four false-RED batch halts vs a one-rung leak the per-id dispositions gate still backstops. Design record updated in `closure_claim.go`. | #448 |
| ACS wrong-root verdicts | Three legs: `suiteProjectRoot` anchors on the **kernel-owned `runs/cycle-N/cycle-state.json`** under the invocation's `--evolve-dir` (file presence, not field values; a bare `runs/cycle-N` dir is agent-mintable by `acs run` and was rejected in review as the anchor); verdicts stamp `suite_root`/`project_root` (absence = unstamped, never mismatch); audit regenerates a pre-staged verdict whose *stamped* root mismatches, preserving the foreign artifact as `acs-verdict.foreign.json`. | #449 |
| Ledger chain breaks + unsealable damage | Lifecycle records routed through `FileLedger.AppendLifecycle` (chained, flocked, atomic tip replace; `core.LedgerEntry` gains additive `task_id`; from/to/reason fold into `message`); `Rebaseline` seals via `appendChainedFromTail` (prev_hash = sha256 of the **physical** last line; shared write-half so the two writers cannot diverge). | #450 |

**Live proof:** console plane rebaselined GREEN — first intact chain since the 2026-07-22 break@78729 (~180 breaks sealed as a preserved-unvalidated prefix per ADR-0048); runtime plane verify GREEN; cycle-1433's honest narrative passed the reworked closure gate while cycle-1432's uncited closure assertion was still (correctly) demanded a citation.

## Lessons (grep-worthy)

- **A linked-worktree plane breaks every `git-common-dir`-based root derivation.** Anything resolving "the main checkout" via git must instead anchor on the state directory the invocation already names (`--evolve-dir` + kernel-owned file). Related queued class: `router-replan-worktree-designation`, `fingerprint-normalizer-path-variance`.
- **A machine-graded artifact must carry provenance for the inputs that shaped it** (here: which roots minted the verdict). ADR-0084 I3 extended: the cycle-1434 misdiagnosis was invisible *from the file*.
- **Every writer of a hash-chained file must go through the chained append path.** A "best-effort telemetry" raw append is not harmless: each line broke the walk AND defeated the repair command. Grep guard: no `O_APPEND` opens of `ledger.jsonl` outside `internal/adapters/ledger`.
- **Repair commands must bind the file as it physically exists,** not as sidecar state remembers it (tip vs physical tail).
- **The halt's templated `root_cause` guess can be wrong while its evidence is exactly right.** Trust the quoted artifacts (FailReasons, verdict-conflict records), not the template's classification prose.
- **`--resume` after a wave-boundary halt correctly refuses** ("no live checkpoint") when both lanes sealed their cycles before the halt — the continuation is a fresh launch, and `--cycles` counts **waves**, not cycles (`cmd_loop.go`).
- **#433 apicover class, 4th occurrence:** every new exported symbol (methods *and* types) needs a NAMED in-package covering test at authoring time — cross-package behavioral coverage does not count (per-package profiles).

## Follow-ups

- Escalation-boundary refile hygiene and the remaining pipeline queue items are tracked in the inbox (see consumed annotations on `pipeline-defect-infra-systemic` ×2 and `ledger-fleet-concurrency-chain` for the full resolution text).
- Halt-message wording: distinguish "resume the checkpoint" from "relaunch fresh" when the halt lands at a wave boundary (minor; queued only if it recurs).
