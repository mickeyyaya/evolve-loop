# Eval: TACO Mid-Trajectory Compaction for Builder + TDD Personas

## Task
Add TACO-style observational context compression guidance to `agents/evolve-builder.md` and `agents/evolve-tdd-engineer.md`. The protocol instructs the model to emit a CHECKPOINT summary every 15 turns and stop re-referencing old tool results, reducing tmux-llm context window growth.

## Acceptance Criteria

### AC-1: Builder persona has mid-trajectory compaction section [code]
```bash
grep -c "CHECKPOINT\|trajectory compac\|mid-trajectory\|tool.result.*summar\|TACO\|context.*compress" \
  /Users/danleemh/ai/claude/evolve-loop/agents/evolve-builder.md
# expect: >= 1
```

### AC-2: TDD-engineer persona has compaction guidance [code]
```bash
grep -c "CHECKPOINT\|trajectory compac\|mid-trajectory\|tool.result.*summar\|context.*compress" \
  /Users/danleemh/ai/claude/evolve-loop/agents/evolve-tdd-engineer.md
# expect: >= 1
```

### AC-3: Compaction triggers at a numeric turn boundary [code]
```bash
grep -E "turn [0-9]+|every [0-9]+ turn|[0-9]+-turn" \
  /Users/danleemh/ai/claude/evolve-loop/agents/evolve-builder.md | head -5
# expect: at least one line matching a numeric turn trigger
```

### AC-4: Builder profile has context_compact_trigger_turns field [code]
```bash
grep "context_compact_trigger_turns" \
  /Users/danleemh/ai/claude/evolve-loop/.evolve/profiles/builder.json
# expect: field present (value >= 10)
```

### AC-5: Negative — no conflicting "compact" profile fields that were previously removed [code]
```bash
grep -E "context_compact_expired|context_compact_threshold" \
  /Users/danleemh/ai/claude/evolve-loop/.evolve/profiles/builder.json | wc -l | tr -d ' '
# expect: 0 (P-NEW-21 removed these dead fields)
```

## Grader Notes
AC-3 is the key behavior test: the compaction must specify a turn number (e.g., "every 15 turns"), not just vague guidance. AC-5 guards against reintroducing the dead profile fields removed in P-NEW-21.
