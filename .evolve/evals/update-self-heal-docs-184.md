# Eval: update-self-heal-docs-184

## Metadata
- slug: update-self-heal-docs-184
- cycle: 184
- task: Extend self-healing-gaps.md "Completed" summary to cycle-183, reference the new self_heal_events signal, and mark ADR-0026 Stage 1 item #4 (spinner disambiguation) shipped in cycle-184 — while preserving all 12 ranked gap-table rows.

> Pins the cycle-184 documentation consolidation. The load-bearing regression
> guard is AC-3 (12 gap rows preserved): a doc edit must not silently drop a gap
> row while extending the summary. Source: cycle-184; scout-report.md Task 3.

## Acceptance Criteria

### AC-1: self-healing-gaps.md "Completed" summary extended to cycle-183 [code]
```bash
grep -ci "cycle-183" /Users/danleemh/ai/claude/evolve-loop/docs/architecture/self-healing-gaps.md
```
Expected: at least 1

### AC-2: self-healing-gaps.md references the new self_heal_events signal [code]
```bash
grep -c "self_heal_events" /Users/danleemh/ai/claude/evolve-loop/docs/architecture/self-healing-gaps.md
```
Expected: at least 1

### AC-3: all 12 ranked gap-table rows are preserved [code]
A regression guard: the edit must not drop or add a gap row.
```bash
grep -cE '^\| [0-9]+ \|' /Users/danleemh/ai/claude/evolve-loop/docs/architecture/self-healing-gaps.md
```
Expected: exactly 12

### AC-4: ADR-0026 marks Stage 1 item #4 (spinner disambiguation) shipped in cycle-184 [code]
```bash
grep -ci "cycle-184" /Users/danleemh/ai/claude/evolve-loop/docs/architecture/adr/0026-self-healing-review-layer.md
```
Expected: at least 1

## Negative Cases

### NC-1: the gap table must not collapse to a stub (a dropped table would read as "0 rows" or "all rows") [code]
Guards against an edit that accidentally deletes the table body or duplicates
rows. The exact-12 assertion in AC-3 fails on both under- and over-count; this
case documents the intent explicitly.
```bash
test "$(grep -cE '^\| [0-9]+ \|' /Users/danleemh/ai/claude/evolve-loop/docs/architecture/self-healing-gaps.md)" = "12" && echo "OK"
```
Expected: OK
