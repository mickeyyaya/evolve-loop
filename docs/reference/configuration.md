# Configuration

The configuration of the loop is in `.evolve/policy.json`. Its keys are optional, and compiled defaults apply when a key is absent. For the keys, see [runtime-reference.md](../operations/runtime-reference.md). This page describes the files that the loop reads and writes in your project.

## state.json

`.evolve/state.json` holds the runtime state of the loop in your project. The loop creates it on the first run. `go/internal/cyclestate` models these keys:

| Key | What it holds |
|---|---|
| `lastCycleNumber` | The highest completed cycle |
| `lastAllocatedCycleNumber` | The highest cycle number that a run took. A run that crashes uses up its number |
| `currentBatch` | The cost of the current batch (`cycleAccruedCostUSD`) and its `goalHash` |
| `failedApproaches` | One record for each cycle that did not PASS |
| `carryoverTodos` | Work that the loop carries from one cycle to the next |
| `triageThroughput` | The coverage floors of recent PASS cycles. It bounds the commitments of triage |
| `setupCompletedAt`, `setupVersion` | The setup marker that `evolve setup complete` writes |
| `stateRevision` | A counter that each locked write increments |
| `lastUpdated`, `version` | The time of the last write and the schema version |

Each write holds a lock on `state.json.lock`. A write keeps the keys that the model does not know, for example `expected_ship_sha`: the pinned SHA of the ship binary, which `evolve reset-sha` pins again.

Older versions documented more keys and files: `research.queries`, `tokenBudget`, `ledgerSummary`, `instinctSummary`, `processRewards`, `notes.md`, the project digest and the directories for instinct promotion. No code reads them now.

### Failed Approaches

When a cycle ends with a verdict that is not PASS, the loop adds a record to `failedApproaches`. For example:

```json
{
  "failedApproaches": [
    {
      "cycle": 42,
      "verdict": "FAIL",
      "classification": "code-audit-fail",
      "recordedAt": "2026-10-07T12:20:05Z",
      "expiresAt": "2026-10-14T12:20:05Z",
      "auditReportPath": ".evolve/runs/cycle-42/audit-report.md",
      "defects": ["D1: the new test does not fail on the old code"],
      "retrospected": true,
      "summary": "the audit found a test that cannot fail"
    }
  ]
}
```

The failure adapter (`go/internal/failureadapter`) reads these records and returns a deterministic decision: PROCEED, RETRY or BLOCK. `evolve failures list` shows the records. `evolve failures prune` removes the expired records and the expired `carryoverTodos`.

## Domain

The Go code does not detect the domain of a project. A project can declare its domain in `.evolve/domain.json`:

```json
{
  "domain": "writing"
}
```

The `domain` key sets the default deliverable kind of a task: `writing` and `research` give `document`, and every other value gives `code`. The loop gives this default to the scout and triage prompts. A task that declares its own kind keeps it.

The other keys of the old format (`evalMode`, `shipMechanism`, `buildIsolation`) are legacy prompt vocabulary, and no Go code reads them. See [domain-adapters.md](../domain-adapters.md).

## Strategy Presets

Strategies steer the cycle's intent without requiring a full goal string:

```
/evo:loop innovate         # feature-first mode
/evo:loop 3 harden         # stability-first for 3 cycles
/evo:loop repair fix auth   # fix-only with directed goal
```

| Strategy | Scout | Builder | Auditor |
|----------|-------|---------|---------|
| `balanced` | Broad discovery | Standard approach | Normal strictness |
| `innovate` | New features, gaps | Additive changes | Relaxed style, strict correctness |
| `harden` | Stability, tests, edge cases | Defensive coding | Strict on all dimensions |
| `repair` | Bugs, broken tests | Fix-only, minimal diff | Strict regressions, relaxed new code |
| `ultrathink` | Complex reasoning, structural refactors | Stepwise confidence estimation | Strict all dimensions |

The strategy is a parameter of the loop (`evolve loop --strategy`). The loop passes it to the agents. The loop also accepts `autoresearch`.

## Goal Modes

### Autonomous (no goal)

```
/evo:loop
/evo:loop 3
```

Scout performs broad discovery and picks highest-impact work.

### Directed (with goal)

```
/evo:loop 1 add dark mode support
/evo:loop add user authentication
```

Scout focuses discovery and task selection on the goal.

## Eval Definitions

The Scout writes the eval definitions in `.evolve/evals/`. You can also write one before a cycle:

````markdown
# Eval: add-auth

## Acceptance Criteria

### AC1: the auth tests pass [code]
```bash
npm test -- --grep "auth"
```

### AC2: the middleware is exported [code]
- `[code]` `grep -r "export.*authMiddleware" src/`
````

`evolve eval quality-check <eval.md>` reads each command in a ```` ```bash ```` fence. It also reads a top-level bullet of the form ``- `[code]` `<cmd>` ``. It does not read a bullet without these backticks. The result is one of these:

- PASS (exit 0): it read commands, and each command can fail.
- WARN (exit 1): it read no command, or a command is weak. An echo-only command and a grep against a literal in the same command are weak.
- HALT (exit 2): a command cannot fail (for example, `true`, a trivial bracket test, or an assertion that a commit is present).

An eval with a `score_cap` and no command gives PASS.

See [eval-grader-best-practices.md](../eval-grader-best-practices.md).

## Lessons

When a cycle fails, the retrospective writes a lesson as a YAML file in `.evolve/instincts/lessons/`. The research lookup (`go/internal/research`) searches these lessons, `knowledge-base/research/` and `docs/research/`. `research.recall_k` in `.evolve/policy.json` sets how many lessons a lookup returns (default 5).

## Model Configuration

The bash-era tier abstraction is removed, and no code reads `.evolve/models.json`. Each profile names a tier on the ladder `fast`, `balanced`, `deep` and `top`. The manifest of each CLI family maps the tier to a model.

Since v22.27.0, the `cli_routing` table in `.evolve/policy.json` decides which CLI runs each phase. `evolve cli-routing` is the only writer of the table. For the tier map, the table and the commands, see [model-routing.md](model-routing.md).
