# Eval: advisory-routing-regression-pin

**Slug:** advisory-routing-regression-pin
**Phase:** cycle-231
**Task:** Promote the advisory-routing regression predicate (acs/cycle-227/008) to the permanent regression suite, and update stale docs that say EVOLVE_DYNAMIC_ROUTING=off is the default.

---

## AC1 — Regression predicate present in regression-suite [code]

```bash
#!/usr/bin/env bash
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}"
PRED="$WORKTREE/acs/regression-suite/cycle-227/008-evolve-phase-registry-advisory.sh"
if [ -f "$PRED" ]; then
  echo "GREEN: advisory regression predicate present at $PRED"
else
  echo "RED: $PRED does not exist — promotion to regression-suite not done" >&2
  exit 1
fi
```

---

## AC2 — Regression predicate executes green [code]

```bash
#!/usr/bin/env bash
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}"
PRED="$WORKTREE/acs/regression-suite/cycle-227/008-evolve-phase-registry-advisory.sh"
export EVOLVE_WORKTREE_PATH="$WORKTREE"
if bash "$PRED" 2>&1 | grep -q GREEN; then
  echo "GREEN: advisory regression predicate exits 0"
else
  echo "RED: advisory regression predicate failed" >&2
  bash "$PRED" 2>&1 | tail -5 >&2
  exit 1
fi
```

---

## AC3 — Stale "default off" text removed from runtime-reference.md [code]

The runtime-reference table previously documented `EVOLVE_DYNAMIC_ROUTING` default as `off`. After this task the table must say `advisory` (or omit the stale note about "off").

```bash
#!/usr/bin/env bash
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}"
RREF="$WORKTREE/docs/operations/runtime-reference.md"
# The old text: "off (static state machine drives, v13.0.0/PR #4)"
# After fix: must NOT say "| off (static state machine drives, v13.0.0/PR #4) |"
if grep -q 'off (static state machine drives, v13.0.0/PR #4)' "$RREF"; then
  echo "RED: runtime-reference.md still contains stale 'off' default text" >&2
  exit 1
fi
echo "GREEN: stale 'default off' text removed from runtime-reference.md"
```

---

## AC4 — dynamic-phase-routing.md updated to reflect advisory default [code]

```bash
#!/usr/bin/env bash
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}"
DOC="$WORKTREE/docs/architecture/dynamic-phase-routing.md"
# Must no longer say "off by default" as a blanket statement (registry pins advisory)
if grep -q '^> \*\*Status:\*\*.*default-off' "$DOC"; then
  echo "RED: dynamic-phase-routing.md still says default-off in status line" >&2
  exit 1
fi
# Must acknowledge advisory as current default
if grep -qi "advisory.*default\|default.*advisory\|registry.*advisory\|advisory.*registry" "$DOC"; then
  echo "GREEN: dynamic-phase-routing.md acknowledges advisory as default"
else
  echo "RED: dynamic-phase-routing.md does not mention advisory as current default" >&2
  exit 1
fi
```

---

## AC5 — ACS suite green [code]

```bash
#!/usr/bin/env bash
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}"
out=$("$WORKTREE/go/evolve" acs suite --cycle 231 -root "$WORKTREE" 2>&1)
if echo "$out" | grep -qE "green=[0-9]+ red=0"; then
  echo "GREEN: ACS suite passes"
else
  echo "RED: ACS suite has red predicates" >&2
  echo "$out" | tail -5 >&2
  exit 1
fi
```

---

## Gaming-fake check

A trivial fake: sed-replace "off" with "advisory" in docs without running the behavioral test. AC2 catches this — it executes the actual predicate which runs `go test ./internal/config/... -run TestRealRegistry_EvolveAdvisoryPinned`.
