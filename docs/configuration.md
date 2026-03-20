# Configuration

## state.json

The primary configuration file is `.evolve/state.json` in your project directory. It's auto-created on first run.

### Research Cooldown

Web research has a 12-hour cooldown. The Scout reuses cached results if queries haven't expired:

```json
{
  "research": {
    "queries": [
      {
        "query": "react server components patterns",
        "date": "2026-03-13T10:00:00Z",
        "ttlHours": 12
      }
    ]
  }
}
```

### Failed Approaches

When a task fails after 3 Builder attempts, the approach is logged:

```json
{
  "failedApproaches": [
    {
      "feature": "WebSocket sync",
      "approach": "Socket.io with Redis",
      "error": "Connection pooling in serverless",
      "reasoning": "Serverless functions have ~10s timeout, WebSocket requires persistent connections. Redis pub/sub also needs a persistent subscriber.",
      "filesAffected": ["src/sync/ws-handler.ts", "src/api/stream.ts"],
      "cycle": 3,
      "alternative": "Consider SSE or polling"
    }
  ]
}
```

The Scout reads this to avoid repeating failed approaches.

### Token Budgets

Control resource consumption per task and per cycle:

```json
{
  "tokenBudget": {
    "perTask": 80000,
    "perCycle": 200000
  }
}
```

- **perTask** (default 80,000): Soft limit for a single Builder invocation. The Scout uses this to size tasks appropriately.
- **perCycle** (default 200,000): Soft limit across all agents in one cycle. The orchestrator warns if exceeded.

## Domain Detection

The evolve-loop auto-detects the project domain during initialization (SKILL.md step 3). Detection determines which eval grader patterns, build isolation, and ship mechanisms are available.

### Detection Signals

| Domain | Primary Signals | Secondary Signals | Confidence |
|--------|----------------|-------------------|------------|
| **coding** | `package.json`, `go.mod`, `Cargo.toml`, `*.py`, `.git` with source files | Test commands detected, build scripts, CI config | High (default) |
| **writing** | `*.md`/`*.docx`/`*.txt` majority (>60% of files), no build commands | Prose-heavy content, style guides, editorial config | Medium |
| **research** | `*.md` with citation patterns (`[1]`, `et al.`), `references/` dir, bibliography files | Research questions file, data sources, methodology docs | Medium |
| **design** | `*.figma`/`*.sketch`/`*.svg` majority, design token files (`tokens.json`) | Component library, style dictionary, asset manifests | Medium |

Detection runs once per session during initialization. The detected domain is stored as `projectContext.domain` and passed to all agents.

### Manual Override: `.evolve/domain.json`

If auto-detection is wrong or the project spans multiple domains, create `.evolve/domain.json`:

```json
{
  "domain": "writing",
  "evalMode": "rubric",
  "shipMechanism": "file-save",
  "buildIsolation": "file-copy"
}
```

Fields:
- **`domain`**: `coding` | `writing` | `research` | `design` | `mixed`
- **`evalMode`**: `bash` (default for coding) | `rubric` (LLM-graded) | `hybrid` (bash + rubric)
- **`shipMechanism`**: `git` (default) | `file-save` | `export` | `custom`
- **`buildIsolation`**: `worktree` (default) | `file-copy` | `branch` | `none`

When `domain.json` exists, it takes precedence over auto-detection. Fields not specified fall back to the auto-detected defaults.

### Mixed-Domain Projects

For projects that span coding and writing (e.g., a codebase with extensive documentation):
- Set `domain: "mixed"` or let auto-detection resolve to the dominant domain
- Coding tasks use bash eval graders; writing tasks use rubric graders
- The Scout determines task domain from the files it targets, not the project-level domain

See [domain-adapters.md](docs/domain-adapters.md) for the full adapter interface.

## Strategy Presets

Strategies steer the cycle's intent without requiring a full goal string:

```
/evolve-loop innovate         # feature-first mode
/evolve-loop 3 harden         # stability-first for 3 cycles
/evolve-loop repair fix auth   # fix-only with directed goal
```

| Strategy | Scout | Builder | Auditor |
|----------|-------|---------|---------|
| `balanced` | Broad discovery | Standard approach | Normal strictness |
| `innovate` | New features, gaps | Additive changes | Relaxed style, strict correctness |
| `harden` | Stability, tests, edge cases | Defensive coding | Strict on all dimensions |
| `repair` | Bugs, broken tests | Fix-only, minimal diff | Strict regressions, relaxed new code |

The strategy is stored in `state.json` and passed to all agents via the context block.

## Goal Modes

### Autonomous (no goal)

```
/evolve-loop
/evolve-loop 3
```

Scout performs broad discovery and picks highest-impact work.

### Directed (with goal)

```
/evolve-loop 1 add dark mode support
/evolve-loop add user authentication
```

Scout focuses discovery and task selection on the goal.

## Eval Definitions

Eval definitions are created by the Scout and stored in `.evolve/evals/`. You can also pre-create them manually:

```markdown
# Eval: add-auth

## Code Graders
- `npm test -- --grep "auth"`
- `npx tsc --noEmit`

## Regression Evals
- `npm test`

## Acceptance Checks
- `grep -r "export.*authMiddleware" src/`
- `npm run build`

## Thresholds
- All checks: pass@1 = 1.0
```

## Ledger Summary

Aggregated statistics from `ledger.jsonl`, stored in `state.json` so agents never need to read the full ledger file. Computed by the orchestrator in Phase 4:

```json
{
  "ledgerSummary": {
    "totalEntries": 30,
    "cycleRange": [1, 16],
    "scoutRuns": 16,
    "builderRuns": 12,
    "totalTasksShipped": 42,
    "totalTasksFailed": 3,
    "avgTasksPerCycle": 2.6
  }
}
```

Agents read `ledgerSummary` from state.json instead of reading ledger.jsonl directly. Only the orchestrator reads/writes the full ledger.

## Instinct Summary

Compact array of active instincts stored in `state.json`, updated during Phase 5 instinct extraction:

```json
{
  "instinctSummary": [
    {"id": "inst-004", "pattern": "grep-based-evals", "confidence": 0.95, "type": "technique"},
    {"id": "inst-007", "pattern": "inline-s-tasks", "confidence": 0.9, "type": "process", "graduated": true}
  ]
}
```

Scout and Builder read `instinctSummary` from state.json instead of reading all instinct YAML files. Full files are only read during consolidation (every 3 cycles) or when `instinctCount` changes.

## Notes Compression

`notes.md` uses a rolling window to keep file size bounded (~5KB max):

```markdown
# Evolve Loop Cross-Cycle Notes

## Summary (cycles 1 through N-5)
<~500 byte paragraph: total tasks, key milestones, active deferred items>

## Recent Cycles
<full detail for last 5 cycles only>
```

Compression runs every 5 cycles (aligned with meta-cycle). Entries older than 5 cycles are compressed into the Summary section. Full history is always preserved in `history/cycle-N/` archives.

## Project Digest

Generated on cycle 1 (and regenerated every 10 cycles), stored at `.evolve/workspace/project-digest.md` (~2-3KB):

- Project structure tree with file sizes
- Language/framework/conventions
- Recent `git log --oneline -10`

On cycle 2+, Scout reads the digest instead of re-scanning the full codebase. Only files listed in `changedFiles` (from `git diff HEAD~1 --name-only`) are read directly.

## Process Rewards

Per-phase scores tracking pipeline efficiency. Updated by the orchestrator in Phase 4 after each cycle:

```json
{
  "processRewards": {
    "discover": 0.0,
    "build": 0.0,
    "audit": 0.0,
    "ship": 0.0,
    "learn": 0.0
  }
}
```

Each score ranges from 0.0 to 1.0:
- **discover** — task relevance (did selected tasks ship?) + sizing accuracy
- **build** — first-attempt success rate + gene/instinct application rate
- **audit** — false positive rate + eval coverage
- **ship** — clean commit rate (no post-commit fixes needed)
- **learn** — instinct quality (were new instincts confirmed in later cycles?)

Process rewards feed into meta-cycle reviews (every 5 cycles) to identify which phases need improvement.

## Instinct Promotion

After 5+ cycles, instincts with confidence >= 0.8 promote from project-level to global:
- **Project:** `.evolve/instincts/personal/`
- **Global:** `~/.evolve/instincts/personal/`
