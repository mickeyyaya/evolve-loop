# Incident 2026-08-10: prompt-compaction lobotomy — the auditor ran on 27% of its persona for the whole zero-ship era

**Severity:** P0 (systemic — degraded every dispatched audit; primary driver of the 1390–1429 FAIL economics)
**Duration:** unknown onset (every section appended below the marker joined the stripped tail at its own birth) → detected 2026-08-10 by the three-perspective batch investigation
**Fixed by:** the `fix/auditor-persona-strip` landing (marker relocated + keep-guard); companion curation for builder/scout/tdd/triage tracked as a follow-up
**Related:** [2026-08-09-zero-ship-batch.md](2026-08-09-zero-ship-batch.md), [2026-08-10-continuation-absorbing-fail.md](2026-08-10-continuation-absorbing-fail.md), ADR-0084

## What happened

`CompactPrompts` (compiled default **ON**, `internal/policy/policy.go` — "strips
~23 KB/cycle of reference tails") truncates every dispatched persona at its
`## Reference Index` heading (`prompts.StripOnDemandSections`, applied at
`internal/phases/runner/runner.go` dispatch and `internal/phases/retro/retro.go`).
In `agents/evolve-auditor.md` that heading sat at **line 75 of 272**: months of
persona evolution appended operational sections at end-of-file — the natural
place to append — and every one of them silently fell below the strip line.

Every dispatched audit prompt was therefore missing, among others:

- the **Verdict Rules** (PASS/WARN/FAIL semantics, per-criterion evidence duty)
- the **STOP CRITERION**, completion gates, and the output-path requirement
- the **MANDATORY continuation-disposition contract** (`defect-dispositions.json`
  duty + its literal example — the #1 cycle killer: 15/30 FAILs in 1390–1429,
  0/11 continuation passes)
- POSTHOC verification, the constitutional audit checklist, WARN prescriptions

Verified live: `audit-prompt.txt` for cycles 1400/1418/1423/1428/1429 contains
**zero** occurrences of "disposition". Cycle-1400's own retro `disposition.json`
diagnosed this as P0; it stayed unfixed for 29 more cycles — the artifact meant
to close the learning loop was itself starved by the sibling defect
(retro completion cutoff, fixed by #432).

Fleet-wide scan at detection: auditor kept **27%** of its persona after
stripping; builder 72% (losing its STOP CRITERION, completion gates, pre-handoff
regression slice, git-tracking attestation, POSTHOC), scout 77% (STOP CRITERION
+ its six gates), tdd-engineer 75% (predicate-quality REQUIRED reading +
assertion templates), triage 75% (inbox ingestion, idempotency skip-list).

## Root cause

The compaction mechanism assumed an invariant — *everything below
`## Reference Index` is on-demand reference* — and **only the destructive half
of that invariant was ever tested**:

- `compaction_coverage_test.go` asserted stripping strictly SHORTENS every persona;
- `realdoc_strip_test.go` set per-persona minimum-savings floors, the auditor's
  at 4096 bytes with the comment "~70 % tail" — fossilizing the lobotomy as a
  test-enforced requirement;
- nothing anywhere asserted that stripping KEEPS the load-bearing directives.

So the guardrails actively defended the defect: any fix that restored the
auditor's operational tail would red the savings floor.

## Fix (this landing)

1. `agents/evolve-auditor.md`: the `## Reference Index` section moved to
   end-of-file; all operational content now sits above the marker. Net line
   count respects the shared scout/builder/auditor line budget (<751).
2. `internal/prompts/realdoc_strip_test.go`: auditor floor recalibrated
   4096 → 256 with the incident cited; what may be stripped is now governed by
   the keep-guard, not a savings quota.
3. **New keep-guard** `internal/phasecoherence/persona_strip_operational_test.go`
   (ships in the repo-contract pack):
   - pins the exact incident anchors surviving compaction of the REAL auditor
     persona (disposition contract + literal example, verdict rules, stop
     criterion, `acs-verdict.json`, POSTHOC, constitutional checklist);
   - for every git-tracked persona (ADR-0084 I1: `repostate.TrackedFiles`),
     asserts no line carrying an operational sentinel
     (`MANDATORY|STOP CRITERION|Completion Gates|force-FAIL|REQUIRED|POSTHOC|Verdict Rules|Constitutional audit`)
     is stripped — with a self-pruning exception list (builder/scout/tdd-engineer;
     an excepted persona that becomes clean fails the test until delisted).
     Triage's tail is operational too (inbox ingestion) but carries no sentinel
     keyword — its curation is a follow-up the sentinel guard cannot cover.

The guard proved itself during authoring: it red'd the fix's own first draft
for placing the word "MANDATORY" inside the stripped tail.

## Token-economics note

Restoring the auditor tail re-adds ~12 KB (~3K tokens) per audit dispatch.
Measured batch economics: ~183K output tokens/cycle average, ~2M+ tokens per
wasted cycle, 15/30 cycles killed by a duty the auditor was never shown.
The compaction "saving" was the single most expensive optimization in the
system's history.

## Follow-ups

- Curate builder/scout/tdd-engineer/triage tails (respecting their savings
  floors or recalibrating them with justification), then shrink the guard's
  exception list to empty.
- The savings floors in `realdoc_strip_test.go` and `compact_marker_gate_test.go`
  remain byte-count incentives; consider replacing them with an explicit
  reference-section allowlist so "compact more" can never again compete with
  "keep the contract".

## Lessons

1. **Every destructive optimization needs a keep-test, not just a
   remove-test.** A mechanism verified only by "it removed bytes" will
   eventually remove the wrong bytes, and its tests will then defend the damage.
2. **Append-at-end + truncate-at-marker is a time bomb** — file growth
   conventions and strip conventions must be reconciled by a structural guard,
   not by author discipline.
3. **Grade the dispatched prompt, not the source file.** Forensics that read
   `agents/*.md` concluded the mandate existed; only reading `audit-prompt.txt`
   (what the model actually received) exposed the truncation. Same class as
   ADR-0084 I2's "single-source the literal example with its reader": the
   reader here is the dispatch pipeline itself.
