# Pipeline Regression-Test Architecture (RT)

> Status: DESIGN (2026-07-02) — campaign queued as inbox `regression-test-architecture`.
> Origin: operator ultrathink post-mortem of 15 regressions (session 2026-06-30 → 07-02); user directive: "build robust and strong tests for all the regressions that happened in the pipeline … design robust and general tests to cover broader cases … structure the test plan to scale, adding tests with no code changes."

## 1. Why this exists

One session surfaced 15 regressions. Fixing each instance is table stakes; this document files every regression into a **root-cause class** and pairs each class with a **general test design** so the *class* can never silently recur. The unifying lesson: the pipeline tested the *work* hard but barely tested its own *referees* — most burned cycles were gate-infrastructure failures (timeout, flake, counter semantics), not work failures.

## 2. Regression catalog → taxonomy

| # | Regression | Class |
|---|---|---|
| R1 | cycle-413: scoped per-cycle checks green, main CI red (`apicover -enforce`) | **A. Scoped-vs-global gap** |
| R2 | PR #297: `internal/setup` test pinned agy manifest literals, broke on manifest change | **B. Cross-package literal duplication** |
| R3 | agy quota wall classified `LivenessConverging` → livelock (ADR-0070) | **C. Signal-state misclassification** |
| R4 | Exhaustion check on 300s checkpoint not 2s poll; hidden by no-op `Sleep` fake | **D. Timing semantics invisible to tests** |
| R5 | `fakeTmux.paneSeq` advances per capture; real tmux idempotent → 3 test breaks | **E. Fake-vs-real fidelity divergence** |
| R6 | `exec.CombinedOutput` err==nil on non-zero exit → gate silently no-oped | **F. API-misuse trap** |
| R7 | cycle-444: EGPS 60s timeout ⇒ missing acs-verdict ⇒ FAIL over audit-PASS-0.9 | **G. Gate-infra failure mode indistinguishable** |
| R8 | cycle-447: one flaky test in the EGPS re-run killed a verified-good cycle | **G** |
| R9 | cycles-448/449: triage-cap counted evidence citations as commitments; corrective unactionable; ADR-0046 demotion broken by reset-holes | **H. Gate semantics ≠ directive semantics** |
| R10 | claude picker omits fable; classifier flaps on identical lists; refresh clobbers operator pins; codex silently falls to `detect` | **I. Non-determinism + silent degradation** |
| R11 | agy `model_tier: noop` — model silently dropped for one CLI (fixed #297) | **J. Silent-noop matrix cell** |
| R12 | `model_routing=auto` live but advisor never emits `{cli,tier}` | **K. Shipped-but-dormant capability** |
| R13 | CC 2.1.193 trust dialog; CC 2.1.195 plugin schema; agy 1.0.15 adds `--model` | **L. External interface drift** |

## 3. Class → general test design (patterns chosen for zero-code scaling)

### A/B — SSOT-projection tests + CI-parity twins
- Every scoped gate has a repo-wide twin at ship (ADR-0069, shipped).
- Pinning tests **derive** expectations from the SSOT artifact (load manifest/catalog, assert the projection) — never restate literals. Adding an SSOT entry requires zero test edits; a projection break is a red test naming both sides.

### C/D — conformance suites over interfaces + registry enumeration
- One contract suite per interface (`LivenessProbe`, drivers, probes); implementations enroll by **enumerating the registry**, never a hand-kept list (the pattern of `liveness_conformance_test.go` and `model_tier_parity_test.go`).
- Timing contracts use **iteration-count proxies** (Sleep-count: ≤N polls = fast path, ~150 = checkpoint path — the test shape that caught R4). Never wall-clock.

### E — differential fake-fidelity suite
- Each test double and its real adapter run the **same** behavioral contract suite for every property tests rely on (idempotent `CapturePane`, marker echo, exit-code mapping). A fake that drifts turns one red conformance test, not N mysterious downstream failures.

### F — anti-misuse pins
- A test that demonstrates the trap (e.g. `CombinedOutput` err==nil on exit 1) + a static gate banning the trapped API from gate-critical packages. Each pinned with its incident reference.

### G/H — gate failure-mode matrices + golden-replay harness (the core of RT)
- For EVERY gate (EGPS generator, triage-cap, contract gate, eval gate, commit-gate, CI-parity, ship): a table
  `{artifact missing, malformed, TIMEOUT, flaky-red (passes on retry), partial, compliant-but-counter-divergent} × {expected verdict, expected diagnostic substring}`.
  Non-negotiable properties: **timeout ≠ absence** (distinct diagnostics naming the ceiling + fix knob), **flake ≠ regression** (retry-once semantics), **every reject reason is actionable** (states the counting rule / the fix).
- **Golden-replay harness**: `go/acs/regression/testdata/regressions/<case>/` with `manifest.json` (`{gate, inputs, expected_verdict, expected_diag_substring}`) + real artifacts from failed cycles. The harness `fs.Glob`s the directory — **adding a regression case = dropping a directory, zero Go changes**. Seed cases (artifacts verified on disk 2026-07-02): cycle-449 triage report (must count 3, not 7), cycle-444 timeout scenario (audit-report present, acs-verdict absent), cycle-447 flake scenario (red_count:1, passes on re-run), failedApproaches[448,449] demotion-hole.

### I — determinism + loud-degradation property tests
- Same input ⇒ same output for every classifier/selector (hash-stability; call-counting fakes prove no re-classification on unchanged input).
- Every fallback path asserts its **diagnostic is emitted** — silence is the bug class; tests pin the loudness, not just the fallback.

### J — capability-matrix parity pins
- Enumerate registry × capability; assert every cell is either effective or **explicitly documented** as an exception (the `model_tier_parity_test` pattern generalized: params channels, sandbox knobs, exhausted_regex presence, prompt markers).

### K — feature-liveness golden replays
- For each authority that can be ON-but-dormant: a golden end-to-end replay proving the wire *fires* (advisor response with `{cli,tier}` ⇒ dispatch line carries the clamped overlay). Dormancy becomes a red test, not archaeology.

### L — interface conformance on version drift
- Covered by inbox `latest-model-preference` D9: bridge-driven per-CLI conformance suite triggered by the preflight version-drift inventory.

## 4. Current-state audit (2026-07-02)

| Question | State |
|---|---|
| Tests per module? | 128 packages under apicover; coverage 82.9% (core) → 100% (ciparity, modelquery). `acs/regression/` holds 170 regression predicates + 6 red-team. |
| Public APIs covered? | `apicover -enforce`: every exported symbol must be named in a test AST (gate-enforced). Naming ≠ behavior; behavioral floor is `core` 82.9%. |
| Dependencies broken for layered testing? | Strong seams where recent work landed: `runCmd` var (audit), `PromptDispatcher` (modelquery), `Deps`+`fakeTmux` (bridge), `SetModelCatalogDirFn` (catalog). **Weakest: `core`** — fewest injectable seams; that is why it is the coverage floor. |
| Best practices? | TDD red-first (gated), table-driven idiom, `-race` in CI, golden corpora (routingeval), adversarial phases, mutation gate. Blind spots this session exposed: gate-infra failure modes (G/H), fake fidelity (E). |

## 5. Campaign slices (RT-T1 … RT-T6)

| Slice | Deliverable |
|---|---|
| **T1** | Gate failure-mode matrices + the golden-replay harness (`testdata/regressions/` glob, data-driven; FIRST STEP: copy the verified seed artifacts from `.evolve/runs/cycle-{444.reset-*,447.reset-*,448,449}` into git-tracked testdata). Subsumes the test halves of inbox `egps-timeout-false-fail` + `triagecap-prose-counter-defect` (their production fixes stay in those items). |
| **T2** | Differential fake-fidelity suite (fakeTmux vs tmux contract; sysexec fake vs real exit-code mapping). |
| **T3** | SSOT-projection sweep: find every cross-package literal pin (R2 class) and convert to derive-from-SSOT. |
| **T4** | Determinism + loud-degradation property tests over the selection/classification pipeline. |
| **T5** | `core` seam-injection refactor (constructor DI for orchestrator collaborators) — unblocks the 82.9% floor without behavior change. |
| **T6** | This document maintained as the filing system: every future regression gets a class row + a golden case; the retro phase files one per abnormal cycle. |

## 6. Runbook: adding a regression test (zero code)

1. Grab the failing cycle's artifacts from `.evolve/runs/cycle-N*/`.
2. `mkdir go/acs/regression/testdata/regressions/<short-slug>/`; copy the minimal artifacts.
3. Write `manifest.json`: `{"gate": "...", "inputs": {...}, "expected_verdict": "...", "expected_diag_substring": "..."}`.
4. Run `go test -tags acs ./acs/regression/` — the harness discovers the case by glob. Red proves the fixture bites; the production fix turns it green; it guards forever.
