# Design recovery validation — 2026-09-09

The recovery closes missing implementation contracts and narrows unsupported design claims. The capability index is [current runtime contract](../architecture/current-runtime-contract.md).

## Changes and evidence

| Gap | Result |
| --- | --- |
| Agent-authored predicate artifacts could authorize Ship | Native Audit executes declared predicates and binds complete results, dispatch identity and the executed source tree into the ledger-bound report. Ship rejects altered, incomplete, stale or host-rejected evidence. |
| Isolation configuration was lost or inferred from nesting | Canonical profile denials reach child launch; required confinement fails closed; macOS linked-worktree metadata grants are restricted and verified with native child fixtures. |
| Resume differed from fresh execution | Shared policy and closeout preserve goal, authoritative verdict, original HEAD, checkpoint progress and quota pause semantics. Completed checkpoints cannot replay. |
| Task context and learning were incomplete | Existing task contracts remain authoritative; bounded, quoted goal/task-specific lesson recall reaches fresh and resumed phases. |
| Writer swarm and singleton dashboard overstated capabilities | Unsupported live writer dispatch refuses before side effects; dashboard tracks active fleet lanes and per-run changes without scanning every dispatch log. |
| Design prose described stronger guarantees | Current architecture/concept guides document actual support; five superseded originals are archived with replacement links. |

## Reviews

- Architecture reviewer: APPROVE / MERGE, no remaining findings. Independently checked lifecycle, authority, confinement, performance and capability boundaries, including native macOS tests and the real Audit → core ledger → Ship green/red round trip.
- Go correctness, simplification and Go-test review: three additional resume defects were fixed with RED → GREEN regressions: quota classification escaped serial checkpoint ownership; legacy goal comparison occurred too late; resumed floor verdict reset. Architecture independently checked these changes.
- Independent defensive security review: PASS, no actionable findings in the documented contract. This was static review with existing test logs, not independent exploit execution or a claim of exhaustive security.

## Validation

- Full default Go suite: 208 packages PASS, 5 packages have no tests (`GOMAXPROCS=4 go test -C go -p 4 -count=1 ./...`).
- Full `go vet` and fresh native binary build: PASS.
- Full affected integration suites: core, checkpoint, cyclestate, acssuite, treefence, Audit, Ship, bridge, sandbox and CLI: 10/10 PASS.
- Targeted race checks: core, dashboard and common phase runner PASS.
- Actual native macOS positive/negative confinement fixtures and host Audit/core-ledger/Ship round-trip: PASS without skips.
- Formatting, whitespace and dashboard JavaScript syntax checks: PASS.

The tests preserve negative checks and exercise successful paths. Legacy fixtures that trusted prestaged verdicts were migrated to the stronger host-execution contract. Real subprocess additions are assigned to the integration tier.

## CI follow-up

The first Linux CI race/integration run exposed an existing fleet test scheduling assumption: a completed B lane could backfill C before A's launch callback had reported its start. The test inferred pool selection order from asynchronous callback observation. Synchronize B's completion after observing both initial lanes, then require C to start while A remains blocked; preserve initial-fill, backfill and completion assertions. This is a fixture correction, with no scheduler behavior change. The unchanged fixture reproduced the exact failure under race stress; the correction passed all 40,000 trials and the full fleet race suite. Independent architecture review approved the preserved contract. The first run's durable ACS and configuration validation passed; final platform CI must pass before landing.

The first macOS CI run passed race/integration tests, then exposed three end-to-end routing fixtures that supplied candidate verdict JSON without executable predicate inputs. The shared project fixture now commits a minimal Go module, cycle-specific predicate and durable package before creating worktrees; its FAIL case executes a real failing test. The fake audit emits structured WARN/FAIL context through the canonical verdict renderer, verified by the actual deliverable checker. Native Audit still retires the fake candidate and mints its own evidence; the fixtures generate no host receipt and retain their routing assertions. The corrected full CLI suite (`go test -tags e2e,evolve_test_phases ./cmd/evolve -count=1 -timeout=45m`) passed in 611.870 seconds; live-provider opt-ins were disabled. Full fake-CLI tests, final vet and native build also passed.

## Limits and live verification

Linux mandatory linked-worktree confinement is explicitly unsupported and fails closed; native Linux enforcement was not verified on this macOS host. Live writer swarm, hard model-family separation and retry adjudication remain outside this recovery. Legacy resume diagnostics are recovery hints, not authenticated Ship evidence. HEAD-based throughput remains a heuristic under concurrent lanes; live assessment must inspect actual lane commits.

Two live fleet waves are the next verification step. Their usefulness is not established by unit tests, historical PASS labels or this report; record actual acceptance evidence and landed changes after those waves finish. The pre-existing unfinished cycle 1606 and user inbox edits are preserved separately.
