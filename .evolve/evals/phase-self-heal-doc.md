# Eval: phase-self-heal-doc

**Slug:** phase-self-heal-doc  
**Phase:** build  
**Cycle:** 177

## Objective

Verify that a new `docs/architecture/phase-self-heal-pipeline.md` document is created and
covers the self-heal stack (retry → backfill → abort), latency sources, and tuning knobs.

---

## Criteria

### C1 — new doc exists [code]

```bash
test -f /Users/danleemh/ai/claude/evolve-loop/docs/architecture/phase-self-heal-pipeline.md && echo "EXISTS" || echo "MISSING"
```

Expected: `EXISTS`

### C2 — doc covers backfill section [code]

```bash
grep -i "backfill" /Users/danleemh/ai/claude/evolve-loop/docs/architecture/phase-self-heal-pipeline.md | head -5
```

Expected: at least one matching line.

### C3 — doc covers retry section [code]

```bash
grep -i "retry\|ErrArtifactTimeout\|transient" /Users/danleemh/ai/claude/evolve-loop/docs/architecture/phase-self-heal-pipeline.md | head -5
```

Expected: at least one matching line.

### C4 — doc covers CLI fallback chain [code]

```bash
grep -i "cli.*fallback\|fallback.*cli\|chain\|candidate" /Users/danleemh/ai/claude/evolve-loop/docs/architecture/phase-self-heal-pipeline.md | head -5
```

Expected: at least one matching line.

### C5 — doc covers tuning [code]

```bash
grep -i "EVOLVE_PHASE_MAX_ATTEMPTS\|EVOLVE_BACKFILL_ENABLED" /Users/danleemh/ai/claude/evolve-loop/docs/architecture/phase-self-heal-pipeline.md | head -5
```

Expected: at least two matching lines (both tuning knobs mentioned).

### C6 — doc has cross-references to related docs [code]

```bash
grep "artifact-backfill\|self-healing-gaps\|phase-timing" /Users/danleemh/ai/claude/evolve-loop/docs/architecture/phase-self-heal-pipeline.md | head -5
```

Expected: at least two matching lines — the doc must cross-reference the ecosystem.

### C7 — doc is non-trivial length [code]

```bash
wc -l /Users/danleemh/ai/claude/evolve-loop/docs/architecture/phase-self-heal-pipeline.md
```

Expected: at least 60 lines.

### C8 — negative: doc does not exist before build (regression guard) [code]

```bash
git -C /Users/danleemh/ai/claude/evolve-loop log --oneline -- docs/architecture/phase-self-heal-pipeline.md | head -3
```

Expected: empty output — this is a new file with no prior commits.
