---
title: self-heal-docs-stale-fixes
cycle: 176
score_cap: 1.0
---

# Eval: self-healing docs stale content fixes

## Context

Two architecture docs have stale content written before cycle-173's changes shipped:
- `artifact-backfill.md` §Observability claims backfill is "invisible in `evolve ledger iter`"
  (stale: cycle-173 added a structured `kind=backfill` ledger entry)
- `self-healing-gaps.md` §Multi-CLI note claims "GAP 1 is the remaining multi-CLI
  resilience gap" (stale: GAP 1 was DONE in cycle-173)

## Acceptance Criteria

### AC-1: artifact-backfill.md references `kind=backfill` ledger entry in Observability section [code]

```bash
# acs-predicate: config-check — doc-content check; waived per acs/AGENTS.md.
REPO_ROOT=$(git rev-parse --show-toplevel)
grep -qi "kind.*backfill\|backfill.*kind\|kind=backfill" \
  "$REPO_ROOT/docs/architecture/artifact-backfill.md" \
  || { echo "RED: artifact-backfill.md does not mention kind=backfill ledger entry"; exit 1; }
echo "GREEN: artifact-backfill.md references kind=backfill"
```

### AC-2: artifact-backfill.md does not claim backfill is invisible in evolve ledger iter [code]

```bash
# acs-predicate: config-check — doc-absence check; waived per acs/AGENTS.md.
REPO_ROOT=$(git rev-parse --show-toplevel)
if grep -q "invisible.*ledger\|Operators can identify backfilled cycles by" \
     "$REPO_ROOT/docs/architecture/artifact-backfill.md"; then
  echo "RED: artifact-backfill.md still contains stale 'invisible in ledger' claim"
  exit 1
fi
echo "GREEN: stale invisible-in-ledger claim removed from artifact-backfill.md"
```

### AC-3: self-healing-gaps.md Multi-CLI note stale sentence is removed [code]

```bash
# acs-predicate: config-check — doc-absence check; waived per acs/AGENTS.md.
REPO_ROOT=$(git rev-parse --show-toplevel)
if grep -q "GAP 1 is the remaining multi-CLI resilience gap" \
     "$REPO_ROOT/docs/architecture/self-healing-gaps.md"; then
  echo "RED: self-healing-gaps.md still contains stale 'GAP 1 remaining' sentence"
  exit 1
fi
echo "GREEN: stale GAP 1 sentence removed from self-healing-gaps.md"
```

### AC-4: self-healing-gaps.md Multi-CLI section still exists (not deleted entirely) [code]

```bash
# acs-predicate: config-check — doc-presence check; waived per acs/AGENTS.md.
REPO_ROOT=$(git rev-parse --show-toplevel)
grep -qi "multi-cli" "$REPO_ROOT/docs/architecture/self-healing-gaps.md" \
  || { echo "RED: Multi-CLI section deleted entirely — context should be preserved"; exit 1; }
echo "GREEN: Multi-CLI section still present in self-healing-gaps.md"
```

## Negative Cases

### NC-1: cli_fallback routing context is preserved after the edit [code]

```bash
# acs-predicate: config-check — doc-presence check; waived per acs/AGENTS.md.
REPO_ROOT=$(git rev-parse --show-toplevel)
grep -q "cli_fallback\|trigger exits\|EVOLVE_.*CLI" \
  "$REPO_ROOT/docs/architecture/self-healing-gaps.md" \
  || { echo "RED: cli_fallback routing context removed — only stale GAP 1 sentence should be removed"; exit 1; }
echo "GREEN: cli_fallback routing context preserved in self-healing-gaps.md"
```
