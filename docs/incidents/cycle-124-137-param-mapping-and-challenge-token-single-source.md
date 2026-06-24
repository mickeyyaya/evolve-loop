# Incident & Architecture Report: cycle-124 → cycle-137 — Parameter-Mapping Refactor + Challenge-Token Single-Source

> **Window:** 2026-05-28 → 2026-05-29 · **Commits:** `7e0e7d1` → `73f543a` (7 PRs) · **Status:** PRs 1–7 shipped to `origin/main`, CI green; standing goal (two clean sequential cycles) validated live on cycle-137.
> Reads top-to-bottom as a narrative; jump via the TOC. Companion to [cycle-123 modal-defense](cycle-123-codex-edit-approval-modal-and-empty-fallback-chain.md) and [cycle-109–116 Go meta-loop bring-up](cycle-109-116-go-meta-loop-bringup.md).

## Table of Contents

1. [Executive summary](#1-executive-summary)
2. [Origin: the operator's actual ask](#2-origin-the-operators-actual-ask)
3. [The cascade: how chasing one goal surfaced six gaps](#3-the-cascade-how-chasing-one-goal-surfaced-six-gaps)
4. [PR-by-PR: what happened and how we resolved it](#4-pr-by-pr-what-happened-and-how-we-resolved-it)
5. [The architecture change in depth: challenge-token single-source](#5-the-architecture-change-in-depth-challenge-token-single-source)
6. [Cycle forensics table (126–137)](#6-cycle-forensics-table-126137)
7. [Lessons & follow-ups](#7-lessons--follow-ups)
8. [References](#8-references)

---

## 1. Executive summary

| Field | Value |
|---|---|
| **Trigger** | Operator directive: store each LLM CLI's parameter mapping in external text/JSON, separated from code (clean-code/design-pattern). |
| **Actual ask** | Delivered by **PR 1 + PR 2** — fully complete. |
| **Cascade** | A standing goal ("two clean `/evo:loop` cycles") drove **PRs 3–7**, each fixing a *procedural/architectural* gap that blocked audit PASS — not a code defect in the operator's feature. |
| **Root architectural fault** | The cycle's **challenge token had no single source of truth.** Two code paths minted it independently (orchestrator at cycle start, bridge at every phase launch), so phase reports diverged → every cycle 134–136 failed audit C1. |
| **Resolution** | PR 6 made the orchestrator the **primary writer**; PR 7 made the bridge a **read-first consumer** (mint only as standalone fallback). The on-disk `challenge-token.txt` is now the single source of truth. |
| **Proof** | cycle-137 scout-report line 2 carried the exact orchestrator-seeded token `f73314437229c5fa` — the divergence that killed 3 prior cycles is closed. |

---

## 2. Origin: the operator's actual ask

> "each LLM CLI should have its parameter mapping table stored in text (e.g. json) format, that translates the abstract layer definition into LLM-CLI-specific call. Make sure such mapping exists and is separated from the code to meet clean code and design-pattern guidelines."

**Pre-state (the problem):**
- Model "tiers" were expressed with Anthropic-specific vocabulary (`haiku`/`sonnet`/`opus`) baked into code paths, leaking one vendor's names into a provider-neutral layer.
- Per-agent `--cli` / `--model` overrides were keyed inconsistently between the CLI flag parser and the runner's env-lookup, so an override for `tdd-engineer` could be read under the wrong key and silently ignored.

**Target architecture (what we built):**

| Layer | Declares | Where | Vocabulary |
|---|---|---|---|
| Profile JSON | `model_tier_default` | `.evolve/profiles/<agent>.json` | abstract (`fast`/`balanced`/`deep`) |
| Manifest JSON | `model_tier_map: {fast,balanced,deep}` | `go/internal/bridge/manifests/<cli>.json` | per-CLI native model names |
| Realizer code | generic translation | `go/internal/bridge/realizer.go` | none — pure Strategy over config |

The realizer became a **Strategy pattern driven by declarative config**: it reads the abstract tier from the profile, looks up the per-CLI native model in that CLI's manifest, and emits the concrete call. Adding a new CLI is now a JSON file, not a code change.

---

## 3. The cascade: how chasing one goal surfaced six gaps

The operator's feature was done at PR 2. A **standing-goal hook** ("two complete successful `/evo:loop` cycles, resolve issues from an architecture perspective") then drove repeated cycle attempts. Each attempt ran the full 7-phase pipeline and failed audit — never on a code defect, always on a **procedural artifact** the pipeline itself failed to produce or verify. Each failure exposed the next layer:

```
PR1/PR2 ship ──> run cycle ──> audit FAIL: missing eval file ──────────> PR3
                 run cycle ──> audit FAIL: predicate undercounts ───────> PR3 (indent anchor)
                 run cycle ──> audit FAIL: scout token absent ──────────> PR4
                 run cycle ──> audit FAIL: builder token missing ───────> PR5 (centralize)
                 run cycle ──> audit FAIL: scout minted OWN token ───────> PR6 (orchestrator mints)
                 run cycle ──> audit FAIL: bridge OVERWROTE token ───────> PR7 (read-first)  ← architecture root
                 run cycle ──> token consistent ✅ ──────────────────────> cycle-137
```

The deep lesson: **PRs 3–5 were prompt/doc patches that treated symptoms.** PR 6 + PR 7 were the architecture fix that treated the cause — there was no authoritative owner of the per-cycle token.

---

## 4. PR-by-PR: what happened and how we resolved it

| PR | Commit | Type | Problem | Resolution |
|---|---|---|---|---|
| **1** | `7e0e7d1` | code | Per-agent `--cli`/`--model` overrides keyed inconsistently between flag parser and runner env-lookup → override silently ignored. | Unified the env-key contract (`EVOLVE_<AGENT>_CLI` / `_MODEL`); runner reads the same key the dispatcher writes. 5 RED→GREEN test cases in `runner_perphase_env_test.go`. ADR-0022 addendum documents the agent-keyed contract. |
| **2** | `e4a9c74` | refactor | Vendor vocabulary (`haiku/sonnet/opus`) leaked into provider-neutral layer; manifests lacked per-CLI tier maps. | Abstract `fast/balanced/deep` everywhere; each manifest declares `model_tier_map`; two-layer back-compat (v1 read-shim + realizer fallback ladder). 7 manifests migrated; `manifest_v1_compat_test.go` pins the migration. |
| **3** | `ec3d628` | docs | TDD prompt didn't mandate `.evolve/evals/<slug>.md` for code tasks; ACS pass-count predicate used `^--- PASS:` which misses **indented** Go subtests (undercount → false FAIL). | TDD-engineer prompt Step 6b "Eval File Authoring" (mandatory all task types) + indent-anchor guidance (use `--- PASS:` without `^`). |
| **4** | `09a4e6c` | docs | scout-report.md lacked the challenge-token header → audit C1 FAIL. | scout prompt mandates the `<!-- challenge-token: VALUE -->` header on line 2. |
| **5** | `c4d0f19` | docs | Only scout was fixed in PR 4; builder/other phases still omitted the header. | Centralized the contract in `agent-templates.md` (format precedence + fail-loudly + worked example) so **all** phases inherit it. |
| **6** | `24b4e51` | code | No Go code injected the cycle's token into any phase prompt → scout *invented its own* (cycle-135: `59576594…` instead of `5b96ecb6…`). | Orchestrator mints the token **once per cycle** at start, surfaces it via `Context["challengeToken"]` (scout's `ComposePrompt` reads it) **and** writes `<workspace>/challenge-token.txt`. |
| **7** | `73f543a` | code | Even after PR 6, bridge's `preparePrompt` **re-minted + overwrote** the file on every phase launch → scout had the orchestrator's token, later phases saw a different one in the file. | `preparePrompt` now **reads** the existing file first (reuse if non-empty) and mints **only when absent** (standalone `evolve bridge launch`). One token per cycle. |

Each code PR (1, 2, 6, 7) went through the full gate: `code-simplifier` → language `go-reviewer` → `commit-gate-runner.sh` (lint + targeted `-race` tests) → tree-SHA-bound attestation → `evolve ship --class manual`. Docs PRs (3, 4, 5) used the sanctioned docs-only bypass.

---

## 5. The architecture change in depth: challenge-token single-source

### 5.1 What the challenge token is

A per-cycle 8-byte hex nonce. Every phase agent must stamp it on line 2 of its report (`<!-- challenge-token: VALUE -->`). The auditor verifies all reports carry the **same** token — this is an anti-forgery / anti-replay control proving the reports came from *this* cycle's real phase runs, not fabricated or carried over.

### 5.2 The fault: two minters, no owner

```
BEFORE (cycles 134–136):

  orchestrator.RunCycle()                 bridge.preparePrompt()  [per phase, ×7]
        │ mint A                                  │ mint B, C, D, …
        │ write file = A                          │ OVERWRITE file = B, C, D, …
        ▼                                         ▼
   Context["challengeToken"] = A           challenge-token.txt = (last phase's mint)

  scout reads Context → report has A
  builder reads file   → report has D        ⇒ AUDIT C1: tokens diverge → FAIL
```

Two independent writers, last-writer-wins on the file, no single source of truth. Classic **shared-mutable-state-without-an-owner** anti-pattern.

### 5.3 The fix: primary writer + read-first consumer

```
AFTER (PR 6 + PR 7):

  orchestrator.RunCycle()  ── mint ONCE ──> challenge-token.txt = A   (PRIMARY WRITER)
        │                                          │
        │ Context["challengeToken"] = A            │ (single source of truth on disk)
        ▼                                          ▼
   scout reads Context → A          bridge.preparePrompt() READS file → A   (READ-FIRST CONSUMER)
                                    every phase reuses A; mint ONLY if file absent
                                          ▼
                              all phase reports carry A  ⇒ AUDIT C1 PASS ✅
```

### 5.4 Why not "orchestrator is the sole minter, bridge never mints"?

Considered and **rejected.** The bridge has **dual use**: it runs inside a full cycle (orchestrator seeds the file) *and* standalone via `evolve bridge launch` (no orchestrator). A read-only bridge would crash standalone launches with an empty token. The **read-existing-or-mint** contract is the correct resolution: the *file* is authoritative; the bridge self-heals only when nothing seeded it.

### 5.5 Correctness precondition (verified)

The read-first branch only engages if both writers point at the **same directory**. Confirmed:

```
orchestrator: cs.WorkspacePath = <root>/.evolve/runs/cycle-<N>   (orchestrator.go:332)
runner:       req.Workspace    = cs.WorkspacePath                 (orchestrator.go:432)
bridge:       cfg.Workspace    = req.Workspace                    (runner.go:347)
```

All three are the same path → PR 7's read genuinely finds the orchestrator's file.

### 5.6 The third writer (harmless)

`launch_modes.go:54` (`runDryRun`) also writes the file — but only on `--dry-run`, which invokes no LLM and ships nothing. Left as-is; documented here so a future reader doesn't mistake it for a fourth mint path.

### 5.7 Code shape (`preparePrompt`, post-PR-7)

```go
if strings.Contains(content, "$CHALLENGE_TOKEN") {
    // Read-existing-or-mint: reuse the orchestrator's token written
    // at cycle start (one token per cycle invariant); mint only when
    // the file is absent or empty (e.g. standalone bridge invocation).
    var tok string
    if existing, err := os.ReadFile(filepath.Join(cfg.Workspace, "challenge-token.txt")); err == nil {
        if v := strings.TrimSpace(string(existing)); v != "" {
            tok = v
        }
    }
    if tok == "" {
        minted, err := deps.NewChallengeToken()
        if err != nil { return "", fmt.Errorf("mint challenge token: %w", err) }
        if err := os.WriteFile(filepath.Join(cfg.Workspace, "challenge-token.txt"), []byte(minted+"\n"), 0o644); err != nil {
            return "", fmt.Errorf("write challenge token: %w", err)
        }
        tok = minted
    }
    content = strings.ReplaceAll(content, "$CHALLENGE_TOKEN", tok)
}
```

`TrimSpace` strips the writer's trailing `\n`, so a round-trip read never redundantly re-mints. Phases are sequential single-writer, so no concurrent-launch race exists.

---

## 6. Cycle forensics table (126–137)

| Cycle | CLIs | Phases run | Outcome | Root cause → fix |
|---|---|---|---|---|
| 126 | mixed | — | killed | self-inflicted tree edit during run (tree-diff guard). **Lesson: never edit working tree mid-cycle.** |
| 127 | codex build / agy audit | 5 | FAIL@audit | agy-tmux REPL boot timeout (exit 80) — infra transient. |
| 128 | codex | — | blocked | codex hard ChatGPT rate-limit until ~Jun 4 2026. |
| 129 | claude / agy audit | 5 | FAIL@audit | agy-tmux 10-min stall (exit 81). |
| 130 | all-claude | **7** | FAIL | audit C2: missing eval file → **PR 3**. |
| 131 | all-claude | 7 | FAIL | missing eval + predicate undercount (`^--- PASS:`) → **PR 3**. |
| 132 | all-claude | 7 | FAIL | scout token absent → **PR 4**. |
| 133 | all-claude | — | exit 81 | builder finished but never wrote build-report.md (transient). |
| 134 | all-claude | 7 | FAIL | scout placeholder token + build-report missing header → **PR 5**. |
| 135 | all-claude | 7 | FAIL | scout minted its own token, ignored seeded file → **PR 6**. |
| 136 | all-claude | 4 | FAIL | bridge overwrote orchestrator's token per phase → **PR 7** (architecture root). |
| **137** | all-claude | **7** | **token fix ✅ / FAIL** | Token consistent across all reports (PR 6+7 proven live). Audit FAIL red_count 3 = predicate footgun (Bug B, §7.6), NOT code. Dispatcher also false-flagged "incomplete/infrastructure" = ledger-verify drift (Bug A, §7.6, FIXED). |

---

## 7. Lessons & follow-ups

### Lessons
1. **Shared mutable state needs an owner.** The token bug was not a typo — it was two writers with no designated authority. The fix was *assigning ownership* (primary writer + read-first consumer), not adding another patch.
2. **Symptom patches mask architecture faults.** PRs 3–5 (prompt mandates) made phases *try* to use the token correctly; only PR 6 + PR 7 made the token *be* correct. Distinguish "tell the agent to do X" from "make X structurally guaranteed."
3. **Procedural FAILs ≠ code defects.** 7 cycles failed audit, zero on the operator's feature. The pipeline's own integrity gates were unsatisfiable until the gaps closed.

### Environment findings (cycle-137 prep)
- **111 stale `ai_term_*` tmux sessions + ~43 claude procs** (from May 18) are **NOT** evolve-loop's — evolve names sessions `evolve-bridge-*`. They are *not* the cause of the exit-80/81 transients and must not be killed (could disrupt other Claude sessions).
- **all-claude-tmux override** is the controlled variable for these two clean cycles: it removes the two known transient sources (codex rate-limit, agy boot/stall). The *any-CLI-any-phase* invariant is a separate goal.

### Follow-ups (not yet done)
- **PR 8 — build-phase artifact resilience:** cycles 128/133/136 hit exit-81 build stalls. Builder prompt should mandate "write build-report.md before reporting complete in pane"; driver needs boot-retry for transient infra.
- **Hardening — reject unknown loop flags:** `evolve loop --goal-file …` silently fell through to usage+exit (a `nohup &` launch looked successful but ran nothing). Unknown flags should exit non-zero.
- **Cleanup — `expected_ship_sha` re-pin friction:** rebuilding the binary legitimately trips the self-TOFU integrity pin; the sanctioned clear-and-re-ship works but is manual.

---

## 7.6 cycle-137: two more framework bugs surfaced (one fixed, one open)

cycle-137 ran all 7 phases on claude-tmux. The challenge-token fix held perfectly — scout-report, triage, test, and build reports all carried the orchestrator-seeded token `f73314437229c5fa`, and the eval file + build-report.md were both authored. But the cycle still did not come out clean, exposing two *new* framework bugs — both the **same drifted-logic anti-pattern** as the challenge-token bug:

### Bug A (FIXED, commit pending) — ledger-verify vocabulary drift

**Symptom:** dispatcher logged `cycle 137 incomplete: missing [scout builder auditor] classification=infrastructure`, recorded a spurious RECOVERABLE-FAILURE, and reported `FinalVerdict: SKIPPED_UNKNOWN` + `total_cost_usd: 0` — despite all 7 phase reports being on disk.

**Root cause:** `go/internal/ledgerverify/verify.go` was ported verbatim from the bash dispatcher (`verify_cycle`) and counts only ledger entries with `kind:"agent_subprocess"` and AGENT-name roles (`builder`, `auditor`). The **Go-native orchestrator writes `kind:"phase"` with PHASE-name roles** (`build`, `audit`). cycle-137's ledger held 7 `kind:"phase"` entries and **zero** `agent_subprocess` → the verifier matched nothing on either axis → false "incomplete" for *every native Go cycle*.

**Fix:** `canonicalRole()` folds both vocabularies onto canonical buckets (`build→builder`, `audit→auditor`; scout/intent/memo identical), and `countsTowardVerify()` accepts both `agent_subprocess` and `phase` kinds. Bookkeeping kinds (`agent_fanout`, `routing_decision`, `cycle_terminal`) and non-zero exits remain excluded — no false-positives. 4 regression tests added (`GoNativePhaseVocabulary`, `MixedVocabularies`, `GoNativeIntentAndMemoPhases`, `BookkeepingKindIgnored`). Backward-compatible with bash-era ledgers.

### Bug B (OPEN) — ACS predicate quality footgun recurs

**Symptom:** audit verdict `FAIL, red_count: 3 (predicates 004, 005, 008)`. The auditor explicitly noted: *"All 8 acceptance criteria are met by the implementation. The FAIL verdict is solely due to predicate bugs."*

**Root cause:** predicates 004/005 ran `go test -race -run <Test>` **without `-v`**, so the passing-package output (`ok <pkg>`) contains no `PASS` line for their `grep -q 'PASS'` → false RED. Predicate 008 mis-extracted the coverage field. This is the **same class** as cycle-131's `^--- PASS:` indent-anchor bug — PR 3 patched that one variant in the TDD-engineer prompt, but the agent hand-rolls predicates per cycle and reintroduced a new variant.

**Proposed fix (architecture, not another prompt patch):** a single shared, unit-tested ACS predicate helper lib (`acs/lib/assert.sh`) exposing `assert_go_test_pass <pkg> [regex]` (asserts on `go test` **exit code** + `^ok\b`, never fragile `grep PASS`) and `assert_go_coverage_ge <pkg> <pct>` (correct field extraction). The TDD-engineer prompt mandates *sourcing* the helper instead of hand-rolling — so "assert a Go test passed" is implemented correctly **once**, the same single-source-of-truth principle that fixed the challenge token.

### Why this matters

Both bugs are the recurring lesson of this whole window: **a self-evolving system keeps hand-rolling or drift-duplicating the same primitive (token mint, test-pass assertion, ledger vocabulary) and getting it subtly wrong a new way each cycle.** The durable fix is always *consolidation to one authoritative implementation*, never another per-instance patch.

---

## 8. References

- Commits: `7e0e7d1` (PR1), `e4a9c74` (PR2), `ec3d628` (PR3), `09a4e6c` (PR4), `c4d0f19` (PR5), `24b4e51` (PR6), `73f543a` (PR7).
- Code: `go/internal/bridge/driver_common.go` (`preparePrompt`), `go/internal/core/orchestrator.go:378-406` (mint+write), `go/internal/phases/scout/scout.go:64` (Context read), `go/internal/bridge/launch_modes.go:54` (dry-run writer).
- Tests: `coverage_batch7_test.go:TestPreparePrompt_ReadsExistingChallengeToken`, `runner_perphase_env_test.go`, `manifest_v1_compat_test.go`.
- ADRs: [0022 launch-intent realizer](../architecture/adr/0022-launch-intent-realizer.md) (env-key addendum), agent-templates.md (token contract).
- Prior incidents: [cycle-123](cycle-123-codex-edit-approval-modal-and-empty-fallback-chain.md), [cycle-109–116](cycle-109-116-go-meta-loop-bringup.md).
