# internal/goalhash

## Purpose

`internal/goalhash` computes a stable hash of the goal string of `/evo:loop`. It is a byte-exact port of the bash pipeline in `legacy/scripts/lifecycle/intent-batch-resolve.sh`.

## Design

The bash pipeline that the port reproduces:

```
normalize_goal() {
    local raw="$1"
    printf '%s\n' "$raw" | tr '[:upper:]' '[:lower:]' \
        | tr -s '[:space:]' ' ' \
        | sed 's/^ //; s/ $//'
}
sha256_of() {
    printf '%s' "$text" | sha256sum | awk '{print $1}'
}
GOAL_HASH=$(sha256_of "$(normalize_goal "$GOAL_TEXT")")
```

- **`Normalize`** lowercases the goal, squeezes each run of Unicode whitespace to one ASCII space, and strips leading and trailing whitespace.
- **The trailing newline.** The bash `printf '%s\n'` appends a newline. `tr -s` folds it into a trailing space, and `sed 's/ $//'` removes it. The Go code gets the same final string without the intermediate newline.
- **`Compute`** is the 64-character lowercase hex SHA-256 of `Normalize(goal)`. It matches `sha256_of "$(normalize_goal "$GOAL_TEXT")"`.
- **`Short`** is the first 8 characters of `Compute`. It is for log display and batch-id derivation.

## Invariants

- **Equivalence with bash is load-bearing.** The `goalHash` in `state.json:currentBatch.goalHash` is the identity of a goal across sessions. Drift between the two languages makes same-goal cycles look like different batches. That breaks incremental-intent delta detection (`EVOLVE_INTENT_DELTA`) and resume continuity.
- **Callers that need the canonical hash use `Compute`.** `state.json` stores the full 64-character value. `Short` is never stored as an identity.
