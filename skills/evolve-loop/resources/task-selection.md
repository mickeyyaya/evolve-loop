# Task Selection: Bandit Mechanism & Adaptive Strictness

## Multi-Armed Bandit

Each task type (`feature`, `stability`, `security`, `techdebt`, `performance`) has an arm in `state.json.taskArms`:
- `pulls` (times selected), `totalReward` (shipped successes), `avgReward`
- **Boost:** `avgReward >= 0.8` AND `pulls >= 3` → +1 priority
- **Exploration:** Arms with <3 pulls always eligible
- After SHIP: success → reward +1; failure → pulls +1 only

Strategy interaction: `innovate` forces feature arm, `repair` forces stability, `harden` prioritizes stability + security.

## Novelty Bonus

Files not touched in 3+ cycles get +1 priority via `state.json.fileExplorationMap`. Novelty stacks with bandit boost.

## Auditor Adaptive Strictness

`auditorProfile.<type>.consecutiveClean` tracks clean audits:
- `consecutiveClean >= 5` → reduced checklist (Security + Eval Gate only)
- WARN/FAIL resets counter to 0
- `harden`/`repair` or agent file changes → always full checklist
- Cross-session decay: halve all counters on new invocation
