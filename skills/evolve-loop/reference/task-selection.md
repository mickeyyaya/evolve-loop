# Task Selection

> Read this file when the Scout selects tasks or the Auditor determines checklist depth.

## Multi-Armed Bandit

Each task type has an arm in `state.json.taskArms`:

| Field | Description |
|-------|-------------|
| `pulls` | Times selected |
| `totalReward` | Shipped successes |
| `avgReward` | totalReward / pulls |

**Rules:**
- `avgReward >= 0.8` AND `pulls >= 3` → **+1 priority boost**
- Arms with `<3 pulls` → always eligible (exploration floor)
- After SHIP: success → `reward + 1`; failure → `pulls + 1` only

**Strategy interaction:**
- `innovate` → forces feature arm
- `repair` → forces stability arm
- `harden` → prioritizes stability + security

## Novelty Bonus

Files not touched in 3+ cycles get **+1 priority** via `state.json.fileExplorationMap`. Stacks with bandit boost.

## Auditor Adaptive Strictness

`auditorProfile.<type>.consecutiveClean` tracks consecutive clean audits:

| Condition | Behavior |
|-----------|----------|
| `consecutiveClean >= 5` | Reduced checklist (Security + Eval Gate only) |
| Any WARN/FAIL | Reset counter to 0 |
| `harden`/`repair` strategy | Always full checklist |
| Agent/skill file changes | Always full checklist |
| New invocation | Halve all counters (cross-session decay) |
