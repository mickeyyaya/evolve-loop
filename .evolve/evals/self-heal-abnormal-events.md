# Eval: self-heal-abnormal-events

**Slug:** self-heal-abnormal-events  
**Phase:** build  
**Cycle:** 177

## Objective

Verify that the orchestrator writes structured events to `abnormal-events.jsonl` when a phase
retries (transient/timeout) and when backfill activates, using the same schema as phasewatchdog.

---

## Criteria

### C1 — orchestrator writes to abnormal-events.jsonl on phase retry [code]

```bash
grep -n "abnormal-events.jsonl\|appendAbnormalEvent\|abnormal_event" /Users/danleemh/ai/claude/evolve-loop/go/internal/core/orchestrator.go | head -10
```

Expected: at least one match — the retry path must write to abnormal-events.jsonl.

### C2 — backfill success path writes abnormal event [code]

```bash
grep -n "abnormal-events\|backfill.*event\|event.*backfill" /Users/danleemh/ai/claude/evolve-loop/go/internal/core/orchestrator.go | head -10
```

Expected: at least one match near the backfill activation block.

### C3 — event types are documented [code]

```bash
grep -E "phase-retry|backfill-activated|backfill-failed|phase_retry_transient|phase_retry_timeout" /Users/danleemh/ai/claude/evolve-loop/docs/architecture/abnormal-event-capture.md | head -5
```

Expected: at least one matching line — new event type slugs must appear in the doc.

### C4 — core tests pass after change [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... 2>&1 | grep -E "^ok|^FAIL"
```

Expected: `ok` — no FAIL.

### C5 — negative: no new abnormal-events writes in non-retry code paths [code]

```bash
grep -n "abnormal-events" /Users/danleemh/ai/claude/evolve-loop/go/internal/core/orchestrator.go | wc -l
```

Expected: 1-5 lines — abnormal-events writes must only appear in the retry and backfill paths,
not proliferate throughout the file.

### C6 — self-healing-gaps.md gap #2 notes backfill as fix [code]

```bash
grep -A3 "gap.*2\|GAP 2\|backfill\|ArtifactTimeout.*backfill\|backfill.*ArtifactTimeout" /Users/danleemh/ai/claude/evolve-loop/docs/architecture/self-healing-gaps.md | head -10
```

Expected: at least one line referencing backfill in the context of ArtifactTimeout / gap #2.
