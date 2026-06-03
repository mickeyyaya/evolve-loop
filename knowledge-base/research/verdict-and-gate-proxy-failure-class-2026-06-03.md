# The Proxy-vs-Ground-Truth Failure Class: One Root Cause Behind Four Loop-Blocking Fixes

**Date:** 2026-06-03
**Scope:** Architectural review of the four cycle-190…207 ship-blocker fixes (`991daa1`, `64b2d95`, `eadb1f3`, `950b387`).
**Verdict:** 1 of 4 is a true root-cause fix; 3 are durable workarounds that will recur. All four are the *same* failure class.
**Severity of the residual:** HIGH — the highest-recurrence workaround (`64b2d95`) is still the live code at v16.2.0, its 3 unpatched sibling classifiers carry identical latent drift, and the recommended drift alarm was never built.

---

## TL;DR

The four blockers look unrelated — a working directory, a markdown heading, a timeout, a stall detector. They are **one failure class**:

> **A gate or observer reads an *implicit proxy* as a stand-in for the ground truth it actually cares about, and the proxy drifts from reality.**

| Blocker | The gate cares about… | …but reads this proxy | Divergence |
|---|---|---|---|
| cwd (`991daa1`) | "where is the code under test" | inherited working directory | main vs worktree |
| classifier (`64b2d95`) | "did the phase succeed" | a prose heading string in an LLM report | template edited under a frozen regex |
| timeout (`eadb1f3`) | "is the suite green" | "finished within 60s" | green but slow |
| observer (`950b387`) | "is the agent alive" | "stdout grew recently" | alive but silent mid-turn |

Every fix moved in the right *direction* — make the implicit input explicit via dependency injection (inject cwd, inject timeout, inject a liveness probe; enumerate the headings). That is the session's own stated decision: *"make environmental inputs explicit (DI)."* **DI removes the implicitness, but it does not make the signal true, and it adds no alarm when the new coupling drifts again.** That is the line between a root-cause fix and a workaround.

---

## Per-fix assessment (evidence-backed)

### 1. cwd — `991daa1` — ✅ TRUE ROOT-CAUSE FIX (localized). Recurrence: LOW.

`acssuite.runBash` now sets `cmd.Dir = opts.Root` so EGPS predicates compile the tree being shipped (the worktree), not the caller's cwd (main).

**Why it's a true fix, not a band-aid over a systemic hole:** a sweep of all 33 non-test `exec.Command*` sites under `go/internal/**` shows the code-tree-touching launchers that matter already set `cmd.Dir` (`ship.go`, `core/orchestrator.go`, `bridge/engine.go`, `core/resume.go`, …). `acssuite.runBash` was a genuine *omission*, now corrected with a sentinel-in-worktree test (`acssuite_cwd_test.go`). This is not one instance of an unaddressed class.

**Residual (latent, defensible):** the fix introduces a **dual root** — cwd = worktree (for `go test`), but `EVOLVE_PROJECT_ROOT` = main (for `.evolve/` runtime data). That split is correct in principle (the code tree and the runtime-data tree are genuinely two different roots) but it is a convention, not a type — the next launcher author can pick the wrong one. Hardening move: name the pair (`ShippedTreeRoot` vs `RuntimeDataRoot`) and add a test asserting code-tree launchers run in the shipped tree.

### 2. classifier drift — `64b2d95` — ❌ WORKAROUND. Recurrence: HIGH.

The build/scout/tdd verdict classifiers grepped for section headings the agent templates no longer emit; valid reports were classified FAIL; build's spurious FAIL tripped the auditor's report-vs-telemetry cross-check → audit FAIL → no ship (cycle-192). The fix adds the current headings to a tolerant allow-list.

**Why it's a workaround:**

- **The class is systemic, not the 3 patched phases.** All **six** phase classifiers derive their verdict by grepping prose out of an LLM-authored report: `build` (`buildReportMarkers`, `build.go:80`), `scout` (`proposedTasksRE`, `scout.go:35`), `tdd` (`containsAny "## Acceptance"…`, `tdd.go:90`), `audit` (`verdictCanonicalRE`, `audit.go:46`), `intent` (`"goal:"`+`"acceptance_checks:"`, `intent.go:82`), `triage` (`topNHeadingRE`, `triage.go:30`). The fix patched 3; `audit`/`intent`/`triage` carry the *identical* latent drift — any edit to their templates re-introduces cycle-192.
- **No drift alarm exists.** No golden/contract test was found at v16.2.0 (swept `go/internal/phases/*/*golden*`, `*contract*`, `go/testdata/golden*` → none). The `64b2d95` commit message *itself* recommends "a golden-fixture contract test as the drift alarm." It was never built.
- **The heading has already drifted twice** (`build.go:74`: `## Files Modified → ## Files Changed → ## Changes`). The tolerant allow-list is an accumulating list of past mistakes; the *third* edit silently re-breaks it.
- **Tolerant matching weakens the check.** Accepting any of three headings makes the classifier less precise about what "complete" means — it trades a false-negative (today) for a future false-positive (a report with a stale heading but missing content now passes).

**The true-fix mechanism already exists in-repo but is unused by these phases:** `phasespec.ClassifyRules{RequireSections, VerdictOnPass}` is documented verbatim as *"the declarative verdict spec — replaces per-phase Go Classify"* (`phasespec.go:37`), and `specrunner.evaluateClassify` already runs it (`specrunner.go:100`). The smart-advisor framework also added `core.VerdictReason{Status, Summary, Taxonomy}` (`verdict.go`) — but note `verdict.go:48`: *"the classifiers already emit the human 'why' as an error-severity Diagnostic"* — it **structures the output of the prose-grep classifiers while still ingesting their prose-grep input.** Origin built a rich verdict vocabulary on top of the exact brittle front door that caused cycle-192. Completing that direction means fixing the *input*.

### 3. predicate timeout — `eadb1f3` — ❌ MITIGATION, not a fix. Recurrence: MEDIUM.

A full-suite predicate (`go test ./...`) exceeds the 60s `DefaultTimeout` on this repo and flakes to a false RED (exit 124), blocking ship (cycle-200). `resolveTimeout` now honors `EVOLVE_ACS_PREDICATE_TIMEOUT_S`.

**Why it's a mitigation:** the env knob treats the *symptom* (predicate too slow). The *cause* is a predicate running `go test ./...` — the whole repo, **O(repo)** — when it only needs the touched package, **O(change)**. Scope discipline was pushed into **goal prose** (the agent-prompt layer: "scope predicates to the touched pkg"), not enforced structurally. A 300s cap on `./...` still degrades as the repo grows; the next large repo or broad predicate re-trips it. The knob is a legitimate safety valve, but it is not the control.

**True fix:** derive predicate test scope from the cycle's changed-package set (already tracked for audit-binding) → `go test ./<touched-pkg>/...`. Timeout env demotes to a backstop.

### 4. observer liveness — `950b387` — ⚠️ SOPHISTICATED WORKAROUND (proxy-on-proxy). Recurrence: LOW–MED.

A tmux-driver agent in a long single "Incubating" turn writes no scrollback and no artifact → false `stall_no_output` (cycle-190). The fix adds an injectable `LivenessProbe`; the tmux implementation hashes `capture-pane` output and treats a changed pane as alive (`tmux_probe.go`).

**Why it's a workaround:** it replaces a bad liveness proxy (stdout-growth) with a *better* one (pane-hash), but neither is ground truth — pane-redraw ≈ alive is coincidental (a spinner/token-counter happens to animate). Two gaps remain:

- **Non-tmux / headless phases get no probe** — `findBridgeSession` returns `""` → the probe returns `false` → the legacy false-stall risk is fully intact there. The moment a phase runs headless and thinks long, cycle-190 returns.
- **Proxy-on-proxy** — an agent stuck in a tight CPU loop that *does* repaint its UI would read "alive"; one alive but with a frozen UI reads "dead."

**True fix:** a real liveness signal — the bridge emits a heartbeat envelope on a wall-clock timer independent of agent output, or the observer reads child-process CPU/activity. Pane-hash demotes to a fallback, and headless phases are covered.

---

## The deeper point (and why it matters for "3 consecutive ships")

The session's actual ship-rate bottleneck was **not** any of these four — it was per-cycle *agent-output variance*: scout selecting an eval slug without materializing the `.evolve/evals/<slug>.md` file, and tdd authoring tautological (grep-only) predicates. Those are **the same failure class one layer up**: a prose/proxy contract between phases instead of a structural one. The ship gate trusts that the scout report's prose *names* an eval, rather than verifying the eval *file exists*; it trusts that a predicate is behavioral, rather than executing it and asserting it fails on the pre-image.

**The pipeline's reliability ceiling is set by how many of its inter-phase contracts are prose/proxy versus structural/verified.** Every blocker in this batch — and the unsolved variance bottleneck — is a withdrawal from that same account.

---

## The true solution — one principle, four moves

**Principle:** *A gate or observer must read a verified signal or ground truth, never an implicit proxy or an LLM-authored prose contract. Where a proxy is unavoidable, pin it with a contract test that fails the moment producer and consumer drift.*

| # | Move | Kills/addresses | Leverage | Mechanism status |
|---|---|---|---|---|
| 1 | **Structured verdict + single source of truth + golden test.** Migrate the 6 hand-rolled classifiers onto `phasespec.ClassifyRules`; generate BOTH the agent template's "Required sections" AND the classifier's `RequireSections` from ONE shared spec; add a golden-fixture test per phase (real recent report → expected verdict). | classifier drift (#2) + its 3 unpatched siblings | **Highest** | `ClassifyRules` + `VerdictReason` exist; single-source-of-truth + golden test do NOT. ADR-0033. |
| 2 | **Scope predicates to changed packages.** Derive EGPS predicate test scope from the cycle's changed-package set. | timeout flakiness (#3) | Medium | changed-file set already tracked for audit-binding |
| 3 | **Real liveness signal.** Bridge heartbeat envelope on a wall-clock timer (or child-process CPU probe); pane-hash → fallback. | observer false-stall (#4), incl. headless | Medium | injectable `LivenessProbe` seam already exists |
| 4 | **Root-selection guard.** Name `ShippedTreeRoot` vs `RuntimeDataRoot`; test that code-tree launchers run in the shipped tree. | cwd split-brain (#1 residual) | Low | most launchers already correct |
| 5 | **Structural inter-phase contracts.** Scout MUST materialize the eval file (gate checks file existence); tdd predicate-quality gate executes the predicate against the pre-image and asserts RED. | the *actual* 3-consecutive bottleneck | High (for ship-rate) | partial (eval quality-check exists; not wired as a hard structural gate) |

Move 1 is the cheapest high-leverage change and the subject of ADR-0033.

---

## Related

- [verdict-classifier-template-drift-2026-06-02.md](./verdict-classifier-template-drift-2026-06-02.md) — the original cycle-192 diagnosis (instance-level).
- [egps-predicate-cwd-discards-passwork-2026-06-02.md](./egps-predicate-cwd-discards-passwork-2026-06-02.md) — the cwd instance.
- [observer-false-stall-tmux-liveness-2026-06-02.md](./observer-false-stall-tmux-liveness-2026-06-02.md) — the observer instance.
- [ai-factory-pipeline-resilience-2026-06-02.md](./ai-factory-pipeline-resilience-2026-06-02.md) — the resilience framing.
- ADR-0033 — the structured-verdict / single-source-of-truth decision (move 1).
