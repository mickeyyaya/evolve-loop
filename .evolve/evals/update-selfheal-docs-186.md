# Eval: update-selfheal-docs-186

## Metadata
- slug: update-selfheal-docs-186
- cycle: 186
- task: Update self-healing living docs to reflect cycle-186 work: extend self-healing-gaps.md "Completed" summary to cover cycles 184–186, reference self_heal_events signal and spinner-disambiguation (PaneHasSubstantiveChange), mark ADR-0026 Stage 1 #4 as shipped in cycle-186.

> Pins the cycle-186 documentation consolidation. Cycles 184 and 185 both attempted
> these doc updates but never shipped (cycle-184 FAILed on mixed ACS predicate;
> cycle-185 was reset before scout completed). The load-bearing regression guard
> is AC-3 (12 gap rows preserved): a doc edit must not silently drop a gap row.
> Source: cycle-186; scout-report.md Task 3.

## Acceptance Criteria

### AC-1: self-healing-gaps.md references self_heal_events signal [code]
```bash
grep -c "self_heal_events" /Users/danleemh/ai/claude/evolve-loop/docs/architecture/self-healing-gaps.md
```
Expected: at least 1

### AC-2: self-healing-gaps.md references spinner/PaneHasSubstantiveChange disambiguation [code]
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

### AC-4: ADR-0026 marks Stage 1 item #4 shipped in cycle-186 [code]
```bash
grep -ci "cycle-186" /Users/danleemh/ai/claude/evolve-loop/docs/architecture/adr/0026-self-healing-review-layer.md
```
Expected: at least 1

### AC-5: phase-timing-and-diagnostics.md documents the self_heal_events signal [code]
```bash
grep -c "self_heal_events" /Users/danleemh/ai/claude/evolve-loop/docs/architecture/phase-timing-and-diagnostics.md
```
Expected: at least 1

### AC-6: self-healing-gaps.md references cycle-186 as the latest completed work [code]
```bash
grep -ci "cycle-186" /Users/danleemh/ai/claude/evolve-loop/docs/architecture/self-healing-gaps.md
```
Expected: at least 1

## Negative Cases

### NC-1: the gap table must not collapse (dropped table is caught by AC-3) [code]
Guards against an edit that accidentally deletes or merges gap rows. The exact-12
assertion in AC-3 fails on both under- and over-count.
```bash
test "$(grep -cE '^\| [0-9]+ \|' /Users/danleemh/ai/claude/evolve-loop/docs/architecture/self-healing-gaps.md)" = "12" && echo "OK"
```
Expected: OK
