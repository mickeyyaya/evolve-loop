# 2026-09-26 — a stale premise reached build: cycle 1691 "fixed" a bug #535 had already closed

**Class:** pipeline. The bugfix path trusted a queued item's premise from the day it was filed. A lane localized, reproduced and "fixed" a defect that had been unreachable for 17 days, and the fix opened a fail-open.
**Surface:** triage's selection judgment (`agents/evolve-triage.md`, `go/internal/phases/triage`), `agents/evolve-fault-localization.md`, `agents/evolve-bug-reproduction.md`.
**Found by:** the audit of cycle 1691 (wave 8, lane `warn-ship-consumption-gap`) and the console's follow-up premise audit of the queue.

## What happened

- **2026-08-16:** `warn-ship-consumption-gap` is filed. A WARN verdict with `red_count:0` ships but never consumes its inbox item.
- **2026-09-09:** #535 (`09d32a19`) makes `acssuite.ReadVerdict` require `verdict=="PASS"` exactly when `red_count==0`. It runs in `verifyAuditBinding` → `verifyPredicateReceipt` before any cycle-class consumption, so the item's triggering state can no longer reach the gate it names.
- **2026-09-26, wave 8:** the item is picked for lane 1691.
  - **Fault-localization** ranked `consume.go`'s verdict-string gate.
  - **Bug-reproduction** ran an existing unit test that seeds `{"verdict":"WARN","red_count":0}` and calls `shipFromWorktree` directly. That bypasses `verifyClass`/`ReadVerdict`, and it inverted the test's exit code into a "reproduction".
  - **The builder** widened the gate to `red_count == 0`. A non-shippable `{verdict:FAIL, red_count:0, ship_eligible:false}` (the shape `acsrunner` writes when predicates never report) now consumed its item on the manual path.
  - **Adversarial review (F1) and the audit (H1, H2)** caught it. H2 named the stale premise. The cycle went to a repair round at the deep tier.

## Root cause

An item's premise is written once, at filing, and nothing re-checks it before a lane spends a cycle on it.

- **Triage** was told to drop stale items but given no evidence or method to judge staleness.
- **Fault-localization and bug-reproduction** took "the exact scenario described in the issue" on trust. Neither asked whether a **production** writer and entry path can still deliver the triggering state to the site, so a fixture that constructs an unreachable state passed as a reproduction.
- **`premise-challenge`** exists, and its goal-is-wrong attack names "the bug is already fixed". But it is inserted only on large cycles, its categories are features, and its verdict is shadow.

**How common it is.** The console premise-audited the next ten dispatchable items, read-only, against `4bd6f2ee`:
- **2 were stale or superseded.** `inbox-console-worklist-view` had landed in `38d3a411`. `routing-advisor-residual` was closed by ADR-0092/0093 and #529.
- **4 were live but mis-scoped:**
  - a declared file that no longer holds the chokepoint;
  - a docs half that already existed;
  - a fourth "duplicate" that is a different concern;
  - a sweep step with zero live targets.

So six of ten would have spent a cycle on a wrong premise or scope.

## Fix (F40)

- **Evidence, deterministic** (`phases/triage/premise_drift.go`). For a fleet lane's scoped items, triage's prompt gains a `premise_drift` section. The item is found wherever triage's claim left it (`inboxmover.Locate`: the root or a `processing/` claim directory), so a re-dispatched triage sees the same evidence. For each item the section gives:
  - its filing date (`inboxbatch.Item.FiledAt`: `created_at`, else the filename stamp);
  - the commits since then that **name its id** (a ship that consumed it lists the item's file, so this finds "already landed" in any package);
  - the commits that touched its **declared paths** (`Item.DeclaredPaths`, F29's one declared-surface token set);
  - the commits, **by subject**, that touched only their packages (never a one-segment parent like `go/`);
  - declared paths **not at HEAD** (`git cat-file`, never the filesystem);
  - paths **declared outside the repository**, reported and never queried.

  The header frames drift as evidence to re-verify, not a verdict. Dates shown are committer dates, the ones `--since` filters on. Every rendered string goes through the one control-character rule (`inboxbatch.StripControl`) and a length cap. The section is bounded (5 items; 5 declared-path commits and 3 others each; one 5 s deadline) and fail-open. A git failure is a visible `drift unavailable` line, never silence. A sequential cycle's prompt is byte-identical.
  - Replayed on the founding case, it lists six commits on the item's own files after filing (#473, #476 "single-source committed-inbox-id resolution", #492, #496, #499 and a cycle ship), plus #535's package activity by subject.
- **Judgment.**
  - The triage persona's **Step 0b** re-verifies a drifted or older item's premise at HEAD **before claiming it**: read the drift, trace production reachability for a bug, confirm a feature is still absent. It drops a premise that no longer holds with `stale: <evidence>` and narrows a partly-true card.
  - Fault-localization traces reachability before ranking and cites an upstream gate that already rejects the state.
  - Bug-reproduction must reproduce through a production writer and entry point, never a fixture that bypasses an upstream reader.
- **A stale drop never retires work on its own** (architecture review C1). `inboxmover.ClosedDroppedIDs`, which lets a PASS landing consume close-class drops, no longer counts `stale`, and it classifies a reason by its **leading tag**: `stale: superseded by #535` is stale; `requires-split (stale)` is a split request. Before this, a lane whose sibling item shipped would have consumed a stale-dropped menu-mate unverified.
  - In a no-work lane the host hands the item to the console (F30). Beside a sibling that ships, it simply stays queued.
  - Pinned at the consumption gate (`TestCommittedInboxIDs_DropReasonGate`) and the reader (`TestClosedDroppedIDs`).
- **Terminal (F30, landed alongside; merge F30 first).** A lane whose triage drops its scoped item with a reason ends as planned no-work (SKIPPED), not FAIL, and the console confirms and retires it.

## Not fixed here

- A prompt-regression replay of cycle 1691's triage input judged stale. It stays open on `stale-premise-reaches-build`.
- A console re-verification anchor (`premise_verified_at`), host verification of a stale drop's cited evidence, and planner-side pre-filters plus a wave-boundary premise audit. Filed as `premise-verification-anchor` (architecture review M3, M5 and follow-ups).
- `.evolve/phases/{bug-reproduction,fault-localization}/agent.md` are drifted, unused copies of the `agents/` personas. Production builds these phases with no inline prompt (`cmd/evolve/cmd_cycle.go`, the user-phase loop), so `agents/*.md` is what runs. The copies should be retired (single source).
- The wave planner counts an item with a pending console-owned dependency as dispatchable, while the launcher's freshness probe refuses it (`artifact-bytes-signal-dual-rendering` → `egps-regression-tia-selection`). Dispatchability should be one belief that includes deps. It costs a WARN line and a refill, not a lane.
