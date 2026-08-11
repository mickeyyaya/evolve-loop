# Eval: update-selfheal-docs-185

## Metadata
- slug: update-selfheal-docs-185
- cycle: 185
- task: Update the self-healing living docs to reflect cycle-185 work: extend self-healing-gaps.md "Completed" summary to cycle-184/185, reference the new self_heal_events signal and spinner-disambiguation, mark ADR-0026 Stage 1 #4 as shipped in cycle-185.

> Pins the cycle-185 documentation consolidation. The load-bearing regression
> guard is AC-3 (12 gap rows preserved): a doc edit must not silently drop a gap
> row while extending the summary. Source: cycle-185; scout-report.md Task 3.
> Note: cycle-184's doc task FAILed to ship (audit block on mixed predicate) so
> this update covers both cycle-184 and cycle-185 work.

## Acceptance Criteria

### AC-1: self-healing-gaps.md references self_heal_events signal [code]
```bash
grep -c "self_heal_events" /Users/danleemh/ai/claude/evolve-loop/docs/architecture/self-healing-gaps.md
```
Expected: at least 1

### AC-2: self-healing-gaps.md references spinner/PaneHasSubstantiveChange progress disambiguation [code]
```bash
grep -ciE "PaneHasSubstantiveChange|spinner.disambig|progress.disambig|Stage 1.*4|item.*4.*spinner" /Users/danleemh/ai/claude/evolve-loop/docs/architecture/self-healing-gaps.md
```
Expected: at least 1

### AC-3: all 12 ranked gap-table rows are preserved [code]
A regression guard: the edit must not drop or add a gap row.
```bash
grep -cE '^\| [0-9]+ \|' /Users/danleemh/ai/claude/evolve-loop/docs/architecture/self-healing-gaps.md
```
Expected: exactly 12

### AC-4: ADR-0026 marks Stage 1 item #4 (spinner disambiguation) shipped in cycle-185 [code]
```bash
grep -ci "cycle-185" /Users/danleemh/ai/claude/evolve-loop/docs/architecture/adr/0026-self-healing-review-layer.md
```
Expected: at least 1

### AC-5: self-healing-gaps.md "Completed" section references cycle-184 or cycle-185 as the latest [code]
```bash
grep -ciE "cycle-18[45]" /Users/danleemh/ai/claude/evolve-loop/docs/architecture/self-healing-gaps.md
```
Expected: at least 1

## Negative Cases

### NC-1: the gap table must not collapse (dropped table is caught by AC-3) [code]
Guards against an edit that accidentally deletes the table body. The exact-12
assertion in AC-3 fails on under-count; this negative case documents the intent.
```bash
test "$(grep -cE '^\| [0-9]+ \|' /Users/danleemh/ai/claude/evolve-loop/docs/architecture/self-healing-gaps.md)" = "12" && echo "OK"
```
Expected: OK
