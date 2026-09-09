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

Both platforms subsequently passed all execution suites, including end-to-end tests. Their final API-coverage gate exposed one missing direct consumer test for `core.ResumeBoundaryCheckpointer`. A real-storage integration test now injects checkpoint persistence failure through production resume and checks error propagation, no phase dispatch, the complete saved checkpoint content, preserved identity/worktree, and ordinary FAIL closeout. It permits JSON formatting changes rather than freezing serialization whitespace. No production behavior or coverage gate is changed. The focused boundary/lifecycle/verdict race selection passed; a temporary Go overlay that ignored the callback error made the new test fail. The full enforced API inventory passed locally against CI's execution coverage for unchanged production code and the final test sources (core: 324/324 exports covered).

## First live verification attempt and launch repair

Recovery PR [#535](https://github.com/mickeyyaya/evolve-loop/pull/535) merged as
`09d32a19` after both platform Go jobs and ACS/configuration checks passed.
The runtime was synchronized without changing the pre-existing user inbox edits
or unfinished cycle 1606.

The first requested two-wave dispatch halted in readiness with `cycles: null`;
no wave ran. A Codex PATH hint overrode a successful native sandbox capability
measurement. Applying the real profile then exposed invalid empty-root SBPL,
missing network permission despite `AllowNetwork=true`, and missing interactive
terminal access. The [launch repair](../architecture/measured-sandbox-capability.md)
records these implementation gaps and their correction.

The follow-up retains mandatory confinement, shares the measured wrap decision
between preflight and launch, and confines terminal access to the assigned device
and explicit termios/window commands. Review rejected unrestricted terminal
ioctls: an owned-PTY probe showed input injection was possible under that broader
rule and denied under the filtered one. No real user terminal was used.

Validation before follow-up CI:

- Full default Go suite: 208 packages PASS, 5 without tests; full vet PASS.
- Four affected integration suites PASS (sandbox, preflight, bridge and loop
  preflight), including real Claude boot; all 150 exported APIs covered.
- Native regression fixtures demonstrate valid empty-root policy, allowed versus
  denied local networking, allowed own-terminal operation, denied peer access,
  denied protected-file writes, and denied unlisted terminal ioctls. The generic
  ioctl mutation fails the native fixture.
- Final termios-setting and confinement race selection: all three affected
  packages PASS. The fixture exercises all three termios update timings; the
  previous Node-only policy fails that positive contract.
- Fresh native `doctor boot <driver> --sandbox --json`: Claude, Codex and Agy all
  report `exit_code: 0`, `booted: true`. These are startup checks, not improvement
  waves or useful code changes.

Architecture, Go/simplification/test, and defensive reviewers reviewed the
follow-up. Live two-wave usefulness remains pending until the repaired version
is landed and the native batch finishes.

## Live quota and continuation repair

Launch repair PR [#536](https://github.com/mickeyyaya/evolve-loop/pull/536)
merged as `985cb68e` with Linux, macOS, ACS and configuration CI green.
A second native `loop --cycles 2 --strategy ultrathink` attempt passed blocking
preflight and launched cycles 1607–1609 in its first wave. Scout produced two
concrete plans and identified already-landed work in the third lane. The native
read-only fence also restored a Triage write outside its allowed surface.

All three lanes subsequently reached TDD under Claude's session quota wall,
whose reset was printed as 05:10 Asia/Taipei. The dispatcher missed that wording
and kept waiting. The operator sent SIGINT to the native parent; it returned
130, `stop_reason: signal`, and `cycles: null`. This is an interrupted attempt,
not a completed wave. No lane reached Build, Audit or Ship. Reports and worktrees
remain available; the displayed zero cost is unmeasured telemetry, not evidence
of free execution.

The follow-up addresses two observed causes:

- The shared manifest recognizes the session wall only with line-leading CLI
  chrome and reset wording. Live detection retains persistence and provider
  corroboration; prompt echoes, diff text and healthy quoted content do not
  immediately bench a working provider. Bench reset parsing honors the explicit
  IANA timezone and existing two-minute margin, rejecting invalid zone suffixes.
  This repairs the provider bench deadline; it does not claim to wire the
  separate persisted cycle checkpoint wake-time hint.
- Continuation adoption merges a pinned main commit into the clean snapshot
  before staging unpublished explanation archives. It uses that exact commit
  as the review/archive base, preserving newly landed canonical records. A
  merge, archive or adoption-persistence failure stops further dispatch and
  preserves work; it cannot silently adopt a stale base. A real Git regression
  reproduces the original archive-before-merge failure, while a raced-main
  conflict proves TDD/Build/Audit/Ship are not dispatched after failed adoption.

Validation of this follow-up: full default suite 208 packages PASS (five without
tests), full vet PASS, targeted race checks PASS in core/bridge/clihealth, and
native adoption failure integration/race PASS. The four affected integration
packages passed, with bridge requiring a dedicated test tmux server initialized
with `SHELL=/bin/sh` (34.434 seconds). Two runs on the pre-existing `/bin/zsh`
server failed accelerated boot deadlines before fixture commands executed;
independent reviewer login-shell commands also hung. The dedicated-shell run
keeps real tmux, sandbox and terminal assertions intact. User shell configuration
was not changed; subsequent live verification uses the same explicit shell.

A fresh native two-wave run allocates new cycle IDs. It preserves interrupted
1607–1609 and the earlier unfinished 1606; no manual claim recovery or unscoped
resume is needed. Explicit interrupted-phase resume remains a separate concern:
these phase-complete checkpoints are intentionally excluded from automatic
cross-run discovery.

## TDD eval authoring repair

Quota/continuation PR [#537](https://github.com/mickeyyaya/evolve-loop/pull/537)
merged as `ab3d99dd` with Linux, macOS, ACS and configuration CI green. The next
native attempt allocated cycles 1610–1612. Cycle 1611 demonstrated the repaired
continuation boundary live: it advanced the adopted snapshot to pinned main
`e8796188` before archiving the unpublished ancestor record.

That lane then exposed a separate role-policy contradiction. TDD's persona
requires permanent `.evolve/evals/<task-slug>.md` authoring, but its checked-in
profile prohibited Edit, Write and OS writes to that directory. The live child
received `Operation not permitted`; its report and ACS tests could not establish
the required complete eval inventory. The operator interrupted the batch and
preserved all reports and worktrees. It returned `stop_reason: signal` and
`cycles: null`; this attempt also does not count as a completed wave.

The repair removes those three TDD-specific denials and declares worktree eval
authoring in its write paths. Builder's eval denial, main-repository read-only
boundary and unrelated protected paths remain enforced. No append-only eval
mechanism or broader profile pattern-language implementation is introduced.

A native macOS regression loads the actual checked-in role profiles through the
bridge and launches a harmless provider fixture. Before the fix, TDD fails on
its required eval write while Builder passes its negative probes. After the
fix, both role cases pass: TDD authoring succeeds, eval reading succeeds, and
Builder eval creation/modification plus both roles' protected/main writes fail
without changing fixture contents. This is behavioral sandbox evidence, not a
claim that model output is useful or that the requested two waves are complete.

Validation: full default Go suite 208 packages PASS (five without tests), full
vet PASS, full native bridge integration PASS (73.566 seconds on a dedicated
`SHELL=/bin/sh` tmux server), and the new native role fixture under the race
detector PASS. The architecture reviewer independently reran both native role
cases successfully.

The attempt also recorded routing overhead (one post-Scout router took 948
seconds and ran broad repository checks) and a TDD report treating undefined
API compilation as RED evidence. These are quality/efficiency observations to
assess against completed native Audit and Ship results, not successful changes.

## Limits and live verification

Linux mandatory linked-worktree confinement is explicitly unsupported and fails closed; native Linux enforcement was not verified on this macOS host. Live writer swarm, hard model-family separation and retry adjudication remain outside this recovery. Legacy resume diagnostics are recovery hints, not authenticated Ship evidence. HEAD-based throughput remains a heuristic under concurrent lanes; live assessment must inspect actual lane commits.

Two live fleet waves are the next verification step. Their usefulness is not established by unit tests, historical PASS labels or this report; record actual acceptance evidence and landed changes after those waves finish. The pre-existing unfinished cycle 1606 and user inbox edits are preserved separately.
