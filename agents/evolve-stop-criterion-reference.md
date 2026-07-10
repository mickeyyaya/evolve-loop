# STOP CRITERION — Shared Structure (all phase personas)

Every phase persona (scout / builder / auditor) ends on the same contract:
satisfy the phase's named completion gates, write the deliverable(s) **once**
via the Write tool, then **halt** — no further reads, searches, re-runs, or
tool calls. Each persona file keeps only its phase-specific gate list and
points here for the shared halt protocol and the banned-post-report patterns
below, so the identical boilerplate lives in exactly one place.

## Halt protocol (shared)

1. Confirm every named completion gate for your phase is satisfied.
2. Write your deliverable(s) in a single final Write call (final version).
3. Stop — no re-reads, re-runs, opportunistic Bash, or additional searches
   after the Write.

## Banned Post-Report Patterns (shared, all phases)

After writing your report artifact(s), these actions are **forbidden**:

- "Let me also check…" reads, re-reads, or opportunistic Bash.
- "Let me verify one more thing…" or "I should also check…" loops.
- Additional WebSearch/WebFetch, or re-running predicates, after the
  report/verdict is written.
- Re-reading build-report.md / scout-report.md after the report is written.

See [AGENTS.md](AGENTS.md) `Shared Constraints` rule #2 for the enforcement
rationale.
