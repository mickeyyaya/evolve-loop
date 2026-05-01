---
description: Atomic git commit + tag for an audited cycle. Single-writer; cannot fan out.
---

# /ship

Atomically ship the current cycle: verify audit verdict PASS, run release consistency check, atomic git commit + tag, ledger update.

## When to use

- After `/audit` returns Verdict PASS
- The current tree-state SHA matches what the auditor saw (cycle-binding)

## Execution

```bash
bash scripts/ship.sh "<commit message>"
```

Or for a full release with version bump and changelog:

```bash
bash scripts/release-pipeline.sh <X.Y.Z>
```

See `docs/release-protocol.md` for the canonical vocabulary (push / tag / release / propagate / publish / ship).

## Cycle-binding (v8.13.0+)

`scripts/ship.sh` refuses to ship if the current tree-state SHA differs from the SHA in the auditor's ledger entry. Prevents "audit cycle 50, ship cycle 51" exploits.

## Bypass (rare; explicit)

For non-cycle commits, set `EVOLVE_BYPASS_SHIP_VERIFY=1` (NOT `_GATE` — see CLAUDE.md). Direct `git push origin main` is denied by ship-gate.

## See also

- `skills/evolve-ship/SKILL.md`
- `scripts/ship.sh`
- `scripts/release-pipeline.sh`
- `docs/release-protocol.md`
