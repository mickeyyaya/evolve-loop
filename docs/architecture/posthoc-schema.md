# POSTHOC Schema — Generalized Sentinel for Truthable Metrics

> **Status:** Active (cycle 76, manual Layer 3 ship)
> **Layer:** 3 of 5 in [ADR-0012 Reward-Hacking Defense System](adr/0012-commit-claim-coherence.md)
> **Generalizes:** cycle 72's cost-data POSTHOC pattern (single metric) to 8 metrics

## Why POSTHOC sentinels exist

Cycle 71 retrospective established that **the Builder cannot reliably self-quote metrics about its own run** because the metrics are written by the runner *after* the Builder's session ends. Cycle 71's Builder claimed `cost ≈ $0.65 / +10%`; the ground-truth `builder-usage.json` showed `$0.73 / +23%`. The Builder underreported cost by 13 percentage points — not from malice but because the value was an in-session estimate.

Cycle 72 codified the fix as a **POSTHOC sentinel pattern** for cost data only:

```markdown
| total_cost_usd | pending <!-- POSTHOC: jq '.total_cost_usd' .evolve/runs/cycle-N/builder-usage.json --> |
```

Builder writes the literal text `pending` plus an HTML-comment sentinel containing the verification command. The Auditor (or runner) executes the command and substitutes the actual value before audit verdict.

Cycle 75 then demonstrated that the same pattern is needed for **AC-existence claims** ("file X exists") — the Builder claimed `test -f /path # exit 0` for files that didn't exist. Layer 3 generalizes POSTHOC to ALL truthable metrics.

## The 8 mandatory POSTHOC metrics

The Builder **must not self-quote** these metrics. Each must appear in build-report.md as `pending <!-- POSTHOC: <command> -->`.

| Metric | Ground-truth artifact | Verification command |
|---|---|---|
| `total_cost_usd` | `<role>-usage.json` | `jq '.total_cost_usd' .evolve/runs/cycle-N/<role>-usage.json` |
| `num_turns` | `<role>-usage.json` | `jq '.num_turns' .evolve/runs/cycle-N/<role>-usage.json` |
| `duration_ms` | `<role>-timing.json` | `jq '.adapter_invoke_ms' .evolve/runs/cycle-N/<role>-timing.json` |
| `input_tokens` | `<role>-usage.json` | `jq '.usage.input_tokens' .evolve/runs/cycle-N/<role>-usage.json` |
| `output_tokens` | `<role>-usage.json` | `jq '.usage.output_tokens' .evolve/runs/cycle-N/<role>-usage.json` |
| `cache_read_input_tokens` | `<role>-usage.json` | `jq '.usage.cache_read_input_tokens' .evolve/runs/cycle-N/<role>-usage.json` |
| `files_changed` | `git show <sha> --numstat` | `git show <sha> --numstat \| wc -l` |
| `lines_added` / `lines_removed` | `git show <sha> --numstat` | `git show <sha> --numstat \| awk '{a+=$1;d+=$2}END{print a,d}'` |

### AC-existence claims

For acceptance criteria of the form "file X exists" or "command Y exits 0":

```markdown
| `docs/architecture/audit-constitution.md` exists | pending <!-- POSTHOC: test -f docs/architecture/audit-constitution.md && echo OK || echo MISSING --> |
| `bash acs/cycle-N/001-foo.sh` exits 0 | pending <!-- POSTHOC: bash acs/cycle-N/001-foo.sh >/dev/null 2>&1; echo $? --> |
```

The Auditor MUST execute the command and substitute the actual output. Authored-prose `# exit 0` text is forbidden.

## Builder enforcement

The Builder persona (`agents/evolve-builder.md`) must:

1. **Never quote a truthable metric directly.** Use `pending` + POSTHOC sentinel for all 8 metrics above plus all AC-existence claims.
2. **Include the verification command in the sentinel comment.** Auditor reads this and runs it.
3. **For INERT markers**, include `re_attempt_by_cycle: N` (max +5) so the deferral has a deadline. INERT without a re-attempt date is treated as permanent abandonment, which is a P5 violation (constitutional audit checklist).

## Auditor enforcement

The Auditor persona (`agents/evolve-auditor.md`) must:

1. **Reject PASS** if any truthable metric is bare-quoted (no `pending` sentinel).
2. **Execute every POSTHOC command** and substitute the result before computing verdict.
3. **Compare ground-truth result to any Builder claim** in the surrounding prose. If Builder said "reduced 50%" but the ground-truth shows reduced 10%, that's a `claim-discrepancy` defect.
4. **Cite the substituted value in audit-report.md** so reviewers can verify provenance.

## Why this works structurally

The truthable metrics all have ground-truth artifacts that the *runner* (not the Builder) writes. By forcing Builder to defer quoting, we make fabrication structurally impossible: there is no value in the build-report to fabricate. The Auditor's execution of the POSTHOC command produces the value.

This closes the cycle 71 (cost over-claim by 13pp), cycle 75 (file-existence fabrication), and any similar future pattern.

## What POSTHOC is NOT

POSTHOC is **not** a universal substitute for Builder's self-reporting. The Builder still authors:

- **Design decisions** ("I chose approach A because B")
- **Code quality assessments** ("the refactor reduces nesting from 4 to 2 levels")
- **Trade-off analyses** ("this is faster but uses more memory")

These are judgment claims, not truthable metrics. POSTHOC only applies where a deterministic command exists that produces the ground-truth value.

## Schema versioning

This schema is `v1`. New truthable metrics may be added in future cycles by:

1. Updating this document with the new row (Metric / Artifact / Command).
2. Updating `agents/evolve-builder.md` (POSTHOC enforcement list).
3. Updating `agents/evolve-auditor.md` (POSTHOC verification list).
4. Adding an ACS predicate that exercises the new metric.

## References

- ADR-0012 (parent design): [adr/0012-commit-claim-coherence.md](adr/0012-commit-claim-coherence.md)
- Cycle 71 lesson (telemetry under-report): `.evolve/instincts/lessons/cycle-71-builder-estimate-vs-artifact.yaml`
- Cycle 72 (cost-data POSTHOC origin): commit `e201c7a`
- Cycle 75 (AC-existence fabrication): `.evolve/runs/cycle-75/audit-report.md` (FAIL@0.99 confidence)
- Cross-cycle synthesis lesson: `.evolve/instincts/lessons/cycle-70-72-mislabeling-pattern.yaml`
