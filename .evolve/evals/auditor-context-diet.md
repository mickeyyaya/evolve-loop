# Eval: Auditor Context Diet — Compressed Handoff Consumption

## Task
Add context-diet guidance to the Auditor persona (`agents/evolve-auditor.md`): instead of consuming full builder/scout reports verbatim, extract only the verdict, diff SHA, ACS summary, and defects. This mirrors P-NEW-8 (AgentDiet for Builder) and P-NEW-9 (Orchestrator 3-bullet summarization), applying the same pattern to the Auditor's input consumption.

## Acceptance Criteria

### AC-1: Auditor persona has handoff-reading protocol section [code]
```bash
grep -c "Protocol\|Handoff.*Reading\|3.bullet\|three.bullet\|extract.*verdict\|verdict.*SHA\|ACS.*summar\|context.diet\|AgentDiet" \
  /Users/danleemh/ai/claude/evolve-loop/agents/evolve-auditor.md
# expect: >= 1
```

### AC-2: Protocol limits builder-report reading to specific fields [code]
```bash
grep -E "verdict|SHA|ACS.*red|defect|thrusts" \
  /Users/danleemh/ai/claude/evolve-loop/agents/evolve-auditor.md | head -5
# expect: at least 1 line showing field-specific extraction guidance
```

### AC-3: Auditor still reads git diff (not just handoff) [code]
```bash
grep -c "git diff\|git.diff" \
  /Users/danleemh/ai/claude/evolve-loop/agents/evolve-auditor.md
# expect: >= 1 (auditor must verify actual code changes, not just reports)
```

### AC-4: Negative — no instruction to skip ACS predicates [code]
```bash
grep -ic "skip.*acs\|skip.*predicate\|no.*acs\|ignore.*acs" \
  /Users/danleemh/ai/claude/evolve-loop/agents/evolve-auditor.md | tr -d ' '
# expect: 0 (ACS predicates are mandatory; diet should not bypass them)
```

### AC-5: Auditor profile references context_digest signal [code]
```bash
grep -E "context_diet\|context_digest\|diet_fields\|read_protocol" \
  /Users/danleemh/ai/claude/evolve-loop/.evolve/profiles/auditor.json | wc -l | tr -d ' '
# expect: >= 0 (optional profile field; if present must match persona guidance)
```

## Grader Notes
AC-1 is the primary indicator. AC-3 is the safety guard: the context diet must NOT cause the auditor to skip reading actual code diffs (security regression risk). AC-4 guards ACS predicate integrity.
