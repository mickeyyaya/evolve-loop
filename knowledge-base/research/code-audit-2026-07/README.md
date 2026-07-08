# Code Audit 2026-07-08 — Fable 5 six-lens deep scan

> Operator-directed (`/goal`: "best utilize fable 5 model to deep scan the code base to find the architecture / design / test / concurrency / etc. code issue and provide solution and inject in the Todo inbox"). Executed on Fable 5's final availability day — the scan agents, verification, and item authoring were all Fable 5, making this the strongest-model audit condition available (cf. `fable-simulation-2026/`: strong-model-authored artifacts +16.2pp).
>
> **Deliverable**: 21 weighted, solution-attached inbox items (`.evolve/inbox/2026-07-08T00-50-00Z-*.json` + `…T02-10-00Z-token-resolver-production-wiring.json`). This report is the durable record of method, evidence, negative results, and the telemetry post-mortem ([token-telemetry-zero-postmortem.md](token-telemetry-zero-postmortem.md)).

## Method

- **Six parallel lens agents** (all Fable 5, read-only, live loop running): concurrency, architecture, silent-failure, test-quality, design/duplication, robustness-at-scale. Each lens got a tailored charter: verified-by-reading requirement (no grep-only findings), per-finding severity/confidence/file:line/failure-scenario/fix-design/pinning-test, max 7-8 findings, known-campaign exclusions (decoupling, flag-reduction, token telemetry, workspace hygiene) and known-filed defects to avoid duplication.
- **Objective analyzers** cross-checking the agents: `go vet ./...`, `staticcheck ./...`, `deadcode ./cmd/evolve` (x/tools), `gremlins unleash` mutation testing.
- **Operator verification pass**: every ≥0.9-weight claim re-verified at source before filing (all confirmed verbatim); findings deduped against 26 pending inbox items and the batch-recent ships.
- **Clustering**: 39 raw findings → 21 cycle-sized items; singles folded into sweeps (minor-hardening ×7, config-authority ×2, growth-caps ×3, test-hygiene ×4).

## Analyzer baseline (objective)

| Tool | Result |
|---|---|
| `go vet ./...` | clean |
| `staticcheck` (production) | 1 finding — SA1012 nil Context, `internal/log/events.go:162` (filed in minor-hardening-sweep) |
| `deadcode ./cmd/evolve` | 186 unreachable funcs; ~168 after excluding test seams — snapshot: [deadcode-2026-07-08.txt](deadcode-2026-07-08.txt) (filed: dead-api-sweep) |
| `gremlins` on `internal/commitgate` | **24 killed / 0 lived / 4 not-covered / 55 timed-out → 100% test efficacy** — the package's own tests are mutation-strong; its real gap is cross-package *wiring* (see gate-wiring-binding-tests) |

The unit layer is demonstrably healthy. The defect mass sits in **cross-package wiring, duplicated single-sources, and silent-failure posture** — classes no per-package gate (apicover, mutation, vet) can see.

## The 21 filed items

| W | Item id | Lens | One-line defect |
|---|---|---|---|
| 0.96 | token-resolver-production-wiring | boundary post-mortem | Telemetry silently OFF: `Deps.TokenResolver` wired nowhere ([post-mortem](token-telemetry-zero-postmortem.md)) |
| 0.95 | ledger-single-writer-consolidation | arch+design (independent×2) | 4 ledger writers, 1 flock, divergent entry_seq semantics on one chain |
| 0.94 | subagent-workspace-absolutize | robustness | LLM-typed relative workspace scatters artifacts into cwd (observed in-repo) |
| 0.93 | statefile-rmw-flock-single-source | design+concurrency | `SealCycle` RMWs state.json with NO flock — fleet lost-update window |
| 0.92 | observer-sink-close-race | concurrency | sink `*os.File` closed while timed-out watcher can still Write |
| 0.91 | selfcheck-breaker-fail-loud | silent-failure | ACS selfcheck write discarded+non-atomic; breaker under-counts on I/O error |
| 0.90 | boot-orphan-sweep-bounded-tombstone | robustness | every boot re-reaps 4,388 historical sessions, serial, no deadline, no tombstone |
| 0.89 | required-roles-ssot | design | requiredRoles ×3 with LIVE drift (cyclehealth counts only `agent_subprocess` kind) |
| 0.88 | inboxmover-promote-mkdir-fail-loud | silent-failure | mkdir failure returned as NoOp success — task stranded in processing/ |
| 0.87 | artifact-name-ssot-retro-backfill | design | artifact-name map ×6; LIVE bug: retro backfill filename mismatch |
| 0.86 | guards-role-hermetic-home | test-quality | C1 security test silently tests wrong path when HOME unset; Alarm never asserted |
| 0.85 | gate-wiring-binding-tests | test-quality | ship attestation reader unbound to writer; qualityGate removable without test failure |
| 0.84 | swarm-session-accounting-fail-loud | silent-failure | Register + MarkReaped errors discarded — reaper's source of truth silently stale |
| 0.83 | engine-launch-deps-call-local | concurrency | `e.deps.OnBoot` mutation; fresh-engine contract enforced by comment only |
| 0.82 | evaluate-batch-retry-parity | design | parallel-evaluate retry loop missing optionalInfraSkip — **blocks the enforce flip** |
| 0.80 | state-growth-caps | robustness | guards.log 23MB/339K lines unrotated; verdict-cache unbounded O(n) RMW; ledger seal built, never invoked |
| 0.78 | paths-subtree-accessors | architecture | `.evolve/runs` etc. hand-joined in ~10 sites; invariants live in comments |
| 0.74 | runlease-phaseobserver-test-hygiene | test+concurrency | package-var seams one t.Parallel from racing; sleep-sync tests; split-brain locking |
| 0.72 | config-single-authority-sweep | architecture | gate defaults authored twice (config literal is dead); agy alias inlined ×3 |
| 0.68 | dead-api-sweep | analyzer | ~168 unreachable funcs incl. whole dormant subsystems, each taxing apicover |
| 0.66 | minor-hardening-sweep | mixed | 7 small verified nits (exit −1 mapping, Gosched pseudo-sync, wire pins, ctx-less network execs, SA1012) |
| 0.65 | llmroute-dispatch-unification | architecture | INVESTIGATE-FIRST: subagent/swarm bespoke CLI resolution vs shared llmroute |

**Status at filing + one batch**: ledger-single-writer-consolidation consumed cycle 616 (PASS); observer-sink-close-race + evaluate-batch work landed cycle 618 (12 files, new `-race` tests); subagent-workspace-absolutize consumed cycle 618 lane (PASS). The scan→loop pipeline closed the top of the queue within hours.

## Negative results (checked, clean — do not re-audit blind)

- **Atomic-write discipline**: uniformly tmp+rename on load-bearing state (state.json repin, checkpoint under flock, cycle-state, leases, swarm manifest). The only violator found is the selfcheck artifact (filed).
- **Timers/tickers**: all 9 sites `defer Stop()`. **RFC3339 parsing**: all cross-process sites handle errors.
- **acsrunner**: drains `go test -json` correctly — no Wait-on-full-pipe deadlock.
- **Inbox item IDs written by Go**: constants, not LLM-controlled (`fleet/starvation.go:152`) — no path-traversal exposure there.
- **Lease heartbeat**: guaranteed `sync.Once` stop. **Per-workspace ndjson appenders**: run-dir-scoped, gc-covered once workspace-hygiene S5 flips mode.
- **guards package**: excellent bidirectional (accept AND reject) coverage triangle on the integrity surface. **phaseintegrity.RepinShipSHA tests**: exemplary (refusal/provenance/operator/relative-path/concurrency). **treestate**: real-git byte-parity test between Go and shell implementations. **inboxmover**: thorough failure-branch coverage (except the mkdir-NoOp defect filed).
- **Accounting packages** (cyclecost/tokenusage/budgethistory): correctly single-sourced — budgethistory imports cyclecost; tokenusage reuses its parser. Only the shared `.evolve/runs` path literal (filed under paths-subtree-accessors).
- **fleet/pool isolation**: correctness rests on the completions channel + disjoint-files invariant, not scheduling order (the Gosched is cosmetic — filed as such, not as a race).

## Verification / regeneration recipes

```bash
cd go
go vet ./...                                   # baseline
~/go/bin/staticcheck ./... | grep -v _test     # 1 known finding until minor-hardening ships
~/go/bin/deadcode ./cmd/evolve                 # regenerate before executing dead-api-sweep
~/go/bin/gremlins unleash ./internal/<pkg>     # mutation-check any gate package; rm -rf $TMPDIR/gremlins-* after
```

Sequencing constraints encoded in the items: inboxmover-promote-mkdir rides with/after `inbox-promotion-requires-landed-ship` (same file); state-growth-caps ledger-seal wiring rides workspace-hygiene S5's batch-end hook; engine-launch-deps-call-local must not disturb the token-telemetry S3 collector seam at `Engine.Launch`; evaluate-batch-retry-parity gates the parallel-evaluate enforce flip.
