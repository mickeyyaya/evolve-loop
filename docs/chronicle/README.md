# Engineering Chronicle

The human-readable record of what was built, why, what else was considered,
what it measurably did, and what it taught us. The repo's commits, ADRs, and
incident docs hold the *what*; this chronicle holds the *reasoning* — the layer
that otherwise lives only in commit bodies, queue items, and session memory,
and evaporates.

**Relationship to other doc surfaces.** ADRs record a single decision's
contract. `docs/operations/` incident docs record a single event forensically.
Operating-policy §3.8 mandates short issue/gap/solution notes on every fix.
A **chronicle entry** is the workstream-level narrative that ties those
together: one entry per campaign/incident-class/strategic move, written for a
human reader who was not there.

## The unified entry template

Every entry uses exactly these sections, in this order:

```markdown
# <Title>
**Period:** <dates> · **Status:** <shipped|in-flight|closed-superseded>
**Primary artifacts:** <PRs / commits / ADRs / docs>

## Problem
What was broken or missing, stated concretely, with the blast radius.

## Context & evidence
How it surfaced; the incidents/cycles/measurements that defined it. Every
claim cites a repo artifact (file, commit, doc, run dir).

## Approaches considered
The real alternatives — including the rejected and the refuted, with why.

## Decision & reasoning
What was chosen and the reasoning chain, named trade-offs included.

## Implementation
What actually shipped, where (files/PRs), and how it was verified (TDD
evidence, mutation proofs, wiring proofs).

## Results (measured)
Before/after numbers or observed behavior deltas. "No data yet" is a valid,
honest entry — say what would measure it.

## Retrospective — what we learned
The transferable lessons: what we'd do differently, what rule/mechanism this
produced, what remains open.

## Links
Cross-references: ADRs, incident docs, docs/research, queue items, sibling
entries.
```

Authoring rules: every factual claim cites its artifact; claims resting only
on operator-session evidence are tagged `[session-evidence]` and kept minimal;
rejected approaches are recorded with the same care as adopted ones — a
refuted fix (e.g. PR #400) is often the most instructive content in an entry.

## Index — 2026-07/08 (the hardening month)

### Harness & verdict integrity
- [False-FAIL storm 862–899 and file-authoritative verdicts](2026-07-false-fail-storm.md)
- [Fingerprint identity: four generations to an honest breaker](2026-07-fingerprint-identity.md)
- [Quota-detection regex drift](2026-07-quota-regex-drift.md)
- [LLM output stability: the contract tail and the capability ceiling](2026-07-llm-output-stability.md)

### Provisioning, fleet & flake war
- [Worktree provisioning contention: the retry, and the refuted alternative](2026-08-worktree-provisioning-retry.md)
- [The graduation test-only class](2026-08-graduation-test-only.md)
- [Channel e2e deflake: sleep-sync and the frozen clock](2026-08-channel-e2e-deflake.md)
- [Retro-fleet stale-worktree: a laundered CRITICAL, recovered](2026-08-retro-fleet-stale-worktree.md)

### Scope disease & config surfaces
- [Scope disease: four costumes of one selection gap](2026-08-scope-disease.md)
- [Regression TIA: real code, dormant switch, restored truth](2026-08-regression-tia.md)
- [Runtime-minted config stubs: phases, profiles, and the release they broke](2026-08-minted-stub-class.md)

### Release engineering
- [Releases v22.11–v22.13.1: assetless failures, demote nets, and the fingerprint flow](2026-08-release-engineering.md)
- [The push-strand class: console merges vs a live batch](2026-08-push-strand.md)

### Process & meta
- [Binary lag: fixes on main while the loop executes the past](2026-08-binary-lag.md)
- [The batch integrity review: gaming lives in status accounting](2026-08-batch-integrity-review.md)
- [The continuation defect ledger: five rounds to an honest mechanism](2026-08-continuation-defect-ledger.md)
- [Contract-block CLI escalation: the fix whose delay was the finding](2026-08-contract-block-escalation.md)
- [Deliverable alignment strategy: the four-layer model](2026-08-deliverable-alignment.md)
