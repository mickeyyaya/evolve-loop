# Incident: cycle-158 — stale `<!-- Challenge:` template drift → audit Structure FAIL → work discarded

**Date:** 2026-05-31
**Severity:** HIGH (blocks ship; surfaced by agy/Gemini following the template literally)
**Run:** multi-CLI validation, builder=`agy-tmux`
**Status:** FIXED

---

## Symptom

Cycle 158's audit (after the Option-C fix made the build's work visible) passed **every** check —
`Matches ACs: PASS` (the auditor ran `go test ./internal/core/... ./internal/phases/backfill/...`
and it passed; ACS 001-008 green), Patterns PASS, Complexity PASS, No secrets PASS, No injection
PASS — **except Structure: FAIL**, which failed the whole verdict → `SKIPPED_UNKNOWN` → work discarded.

> Structure FAIL: `build-report.md:2` uses `<!-- Challenge:` instead of the canonical lowercase
> `<!-- challenge-token:` header, so the handoff format no longer matches the canonical contract.

`acs-verdict.json` showed `red_count: 0` (EGPS predicates all green) yet `verdict: FAIL` — the
auditor's Structure check, not EGPS, was the blocker.

## Root cause — conflicting templates across agent files

The build-report header template existed in **two contradictory forms**:

| Form | Where | Status |
|---|---|---|
| `<!-- Challenge: {challengeToken} -->` | `evolve-builder-reference.md:306`, `evolve-tdd-engineer.md:152`, `evolve-scout-reference.md:187` | **stale (capital C, wrong label)** |
| `<!-- challenge-token: … -->` | `agent-templates.md:122` (canonical) + 8 other agent files | correct |

The auditor enforces the canonical `<!-- challenge-token:`. agy/Gemini followed the builder
reference's stale template literally and emitted `<!-- Challenge:`, which the auditor rejected.

### Same multi-CLI pattern as cycles 154 & 156

This is the third instance of the same class: a doc/template inconsistency that was invisible while
Claude ran the phase (Claude apparently emitted the canonical header regardless of the stale
reference) but breaks the moment a different CLI follows the written template literally. See:

- `cycle-154-agy-tmux-m-flag-repl-boot-timeout.md` (agy launch flags)
- `cycle-156-builder-commit-vs-audit-pending-diff.md` (agy commits its work)

## Fix

Changed all **3** stale occurrences to the canonical `<!-- challenge-token: {challengeToken} -->`
(builder-reference, tdd-engineer, scout-reference). `grep -rn "<!-- Challenge:" agents/` now returns
nothing; all agent templates agree with `agent-templates.md:122`.

## Follow-up (not blocking, MEDIUM)

The audit also flagged a MEDIUM: POSTHOC metric resolution failed because `builder-usage.json` and
`builder-timing.json` sidecars were never emitted into the cycle workspace, so the build report's
`pending <!-- POSTHOC: jq … -->` placeholders for `num_turns`/`duration_ms` stayed unverifiable. The
runner references sidecars it doesn't emit for the agy path. Queued — it WARNs, it doesn't block ship.
