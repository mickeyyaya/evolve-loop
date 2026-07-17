# Part 5 — Token-Optimization Campaign: Implementation Record (2026-07-17)

> **What this is.** A complete engineering record of the token-optimization campaign
> executed 2026-07-17: the problems found, root causes, the fixes and the reasoning
> behind them, the measured results, and the operational GOTCHAs. Companion to
> [part4-per-phase-boot-context.md](part4-per-phase-boot-context.md) (the design) and
> parts 1–3 (the research). Written so a future engineer can reconstruct *why* each
> decision was made, not just *what* changed.

## 0. The operator's ask & the method

> "Claude seems to burn tokens at a much faster rate. Consider starting the LLM CLI
> in a safe/clean-start mode that loads no context. Research the 2026 literature on
> lowering the context window — less comment/prompt, each phase precise and focused."

**Method that governed the whole campaign: measure-first.** You cannot lower a
context window you cannot see. Every step was gated on real telemetry, and — twice —
measurement *reversed* a plan that looked right on paper (the inert first fix; the
"build per-CLI telemetry" decision). The recurring lesson: **verify against real
on-disk data, never against assumptions encoded in unit fixtures or record counts.**

Five ships landed, all config/code, all reviewed + ship-gated:

| commit | what |
|---|---|
| `c41fa94b` | telemetry attribution by ArtifactPath (shipped **inert** — see §1) |
| `95f3e79f` | firstUserText reads bare-string content (the **real** telemetry fix) |
| `315175bd` | clean-boot B-v1 (3 heavy phases) |
| `42fe5244` | clean-boot B-v2 (6 more phases) |
| `94f5d84b` | B-v3 per-phase `--tools` + skill de-risk |

---

## 1. Problem: token telemetry was blind (recorded all-zeros)

### 1.1 Symptom
Every phase's usage sidecar and the `llm-calls.ndjson` telemetry spine recorded
input-side **zero** for claude phases: `{"input":0,"output":<round est>,
"cache_read":0,"cache_write":0}`, `source:"none"` or `"scrollback_peak"` — never
`"transcript"`. The operator's felt "fast token burn" was real but **invisible**: the
context-window cost lives in `cache_read`, and it read as 0.

### 1.2 Root-cause investigation (and two misdiagnoses)
The token-telemetry campaign (S1–S7) had shipped, and an earlier memo blamed
"`Deps.TokenResolver` wired nowhere." **That was stale** — the resolver *is* wired at
two composition roots (`adapters/bridge/bridge.go:135`, `subagent/validateprofile.go:343`).
So the resolver ran, but produced `source:none`.

The resolver (`go/internal/tokenusage`) uses a **per-driver Strategy** chain
(`defaultresolver.go`): claude → transcript JSONL (carries input/cache) → events-log →
scrollback (output-only). Real claude transcripts existed on disk
(`~/.claude/projects/<worktree-slug>/*.jsonl`) with a peak **`cache_read` of 173,121
tokens/turn** — but the transcript tier returned `SourceNone`, so the chain fell
through to `scrollback_peak` (output-only → input/cache structurally 0).

**Why the transcript tier missed:** `scanner.go attributes()` gated on an exact
`ln.Cwd == w.Worktree` match. On the tmux driver path `w.Worktree` is **lossy** —
`WORKTREE_PATH` degrades to `req.ProjectRoot` (the repo root) at `subagent/run.go:290`,
and the agent `cd`s into `./go` — so the transcript's recorded cwd never equalled
`w.Worktree`. Diagnosis proven by running the *real* resolver against real `~/.claude`
for cycle-874: cwd-keyed match = 0 hits; the transcript was there but never considered.

### 1.3 Fix #1 (`c41fa94b`) — correct idea, shipped INERT
Promote the launch-unique **ArtifactPath** to the primary attribution key: the general
bridge `Window` already carries it, it is stamped verbatim into the launch's first user
message, and it is cycle+phase unique. `attributes()` → `strings.Contains(firstUserText
(lines), w.ArtifactPath)`. Passed TDD, both reviewers, and a "production-fidelity"
reasoning check. **It did not work in production** — cycles 870–875 kept recording
zeros.

**Why it was inert (the critical lesson):** `firstUserText` decoded only the
*array-of-blocks* content form (`Message.Content []struct{Type,Text}`). **Real Claude
Code transcripts store the first user message `content` as a bare JSON string** (the
Deliverable-Contract phase prompt). So `firstUserText` returned `""` and the artifact
never matched. **The unit fixtures all used the block form**, masking the exact gap.
Also learned here: the real dispatch path is `phases/runner/runner.go` →
`adapters/bridge` (Deliverable-Contract prompts via `phasecontract/render.go`), **not**
the `subagent/run.go` + `assembleV2Prompt` path originally traced — the `"Artifact
path:"` marker the robustness argument relied on does not exist in production prompts
(the absolute artifact path *is* present, so `Contains` still matches).

**How it was caught:** a throwaway `TestZDiag_RealResolve` ran the *real*
`ScanConfigRoot` against the *real* `~/.claude` for cycle-874 → ArtifactPath hits = 0
(broken), cwd-match hits = 9 (worked, but summed the whole cycle → over-attribution).

### 1.4 Fix #2 (`95f3e79f`) — the real fix
`transcriptLine.Message.Content` → `json.RawMessage`; new `contentText()` helper
decodes **both** forms (unmarshal-as-string first, then as `[]struct{Text}`).
**Verified against real data before shipping:** re-ran the diagnostic — ArtifactPath
hits = **1** (correct per-phase), `source=transcript`, per-phase `cache_read=1,699,278`.
Then confirmed **live**: cycle-877 recorded `source:transcript, cache_read:19799,
cache_write:32787`. Only then was it called done.

### 1.5 The load-bearing lesson
**A parsing/telemetry fix is not verified until it runs against real on-disk data.**
Unit fixtures encode your assumptions; this bug *lived* in an assumption (content
shape). Write the throwaway real-data probe every time.

---

## 2. Problem: excessive per-turn boot context (the clean-boot campaign)

### 2.1 What the now-working telemetry revealed
Baseline (cycles 876–879): **~13M cache_read/cycle** (later measured higher with full
coverage), concentrated in fault-localization (4.3M/run), bug-reproduction (3.98M),
coverage-gate, build, tdd.

### 2.2 The analysis — and a false dichotomy I corrected mid-campaign
Naïve read: compare phase *totals* — scout (310K) vs fault-localization (4.3M) — and
conclude the difference is *accumulated* context, so clean-boot (which trims the fixed
boot) is only a ~10% lever. **The per-turn data refuted this.** A fault-localization
transcript: context starts at a **~81K fixed base** (claude system prompt + full tool
schemas + MCP + skills + CLAUDE.md) and only grows +56K over 43 turns. The base is
**re-read on every turn** (as `cache_read`). So:

```
cache_read  ≈  turns × per-turn-base
```

Heavy phases burn most *because they take the most turns* (22–128), and each turn
re-pays the base. **Cutting the base IS how you prune heavy phases** — the "clean-boot
vs prune-heavy-phases" dichotomy was false. The +56K growth is the smaller, riskier
factor (that's Slice C).

### 2.3 Empirical flag measurement (measure the lever itself)
Rather than assume, probed the real `claude` 2.1.212 with `claude -p "…"
--output-format json`:

| flags | per-turn base | cut | risk |
|---|---:|---:|---|
| DEFAULT (what phases got) | ~64K | — | — |
| `--tools Bash Read Grep Glob` (4 tools) | ~22K | **65%** | ⚠️ breaks phases needing an unlisted tool |
| `--tools <all built-ins>` | ~39K | 34% | medium |
| `--disable-slash-commands --strict-mcp-config` | ~46K | **~22%** | 🟢 none — keeps all built-in tools |

Key facts: `--tools` **subsumes** `--disable-slash-commands`/`--strict-mcp-config` (it
whitelists tools, excluding skills & MCP anyway); the `--allowedTools` list is
harness-side (zero model tokens) — not worth trimming; claude has **no** context-
compaction flag (only `--max-budget-usd`).

### 2.4 The mechanism — config-injected, no Go changes
The seam already existed: profiles carry `extra_flags_by_cli.<cli>` (read by
`extractAdapterOverrides` → appended to the launch command by the tmux driver), and
**8 profiles already ran the exact clean-boot block** (scout, auditor, orchestrator,
reflector, …) — a *proven in-repo pattern*, not net-new. So the whole campaign is
**config-only** (`.evolve/profiles/*.json`, hand-authored), honoring
`phases_are_config_only` / `no_feature_flags`. The block:
```json
"extra_flags_by_cli": { "claude-tmux": [
  "--strict-mcp-config", "--exclude-dynamic-system-prompt-sections",
  "--disable-slash-commands", "--setting-sources", "project" ] }
```

### 2.5 B-v1 → B-v2 → B-v3
- **B-v1 (`315175bd`)**: applied the block to fault-localization (the #1 burner, had
  none), secret-leak-scan (none), tdd-engineer (had it minus `--strict-mcp-config`).
- **B-v2 (`42fe5244`)**: extended to coverage-gate, error-handling-scan,
  flake-rerun-scan, bug-reproduction, adversarial-review, builder (these run codex/agy
  primary but fall to claude when benched, where they burned the full base). Coverage
  8 → 23 profiles.
- **B-v3 (`94f5d84b`)** — two parts:
  1. **`--tools <observed set>`** on the *simple-tool* phases for a further ~37% base
     cut: secret-leak-scan/error-handling-scan = `Bash Read Write Grep Glob`;
     fault-localization = `+Edit ToolSearch`. Derived from **observed transcript usage,
     NOT `allowed_tools`** (fault-localization uses `Write`/`Edit` absent from its
     declared `allowed_tools` under `--dangerously-skip-permissions`). *Not* applied to
     orchestration-heavy phases (coverage-gate/build/scout use Agent/Monitor/Task*/
     ScheduleWakeup — a tight whitelist would break them and cut little anyway).
  2. **Skill de-risk**: removed `--disable-slash-commands` from bug-reproduction and
     adversarial-review — they invoke real skills (`superpowers:systematic-debugging`,
     `security-patterns-code-review`), and "Disable all skills" would degrade them
     (rare — 1/138, 4/320 runs — but a security-review gate isn't worth it). Caught by
     inspecting *actual tool usage*, not assuming.

### 2.6 Validation (quality, not just tokens)
Every ship was checked for **quality**, not just token drop — the lesson from §1.
B-v1 fault-localization at the reduced base produced valid, contract-compliant
deliverables (cycle-880: 8,177 bytes with `## Suspect Ranking` + `## Edit Locations`;
cycle-881 likewise). Control phases (build/audit, already clean-booted) held flat —
proving causality.

### 2.7 Measured results (live telemetry, adjacent-cycle comparison)
| | value |
|---|---|
| **Aggregate cache_read/cycle** | 36.6M (pre) → **22.2M** (post) = **−39% ≈ 14.4M tokens/cycle** |
| fault-localization per-turn base | 81.6K → 50K (clean-boot) → **32K** (`--tools`) = **−61%** |
| error-handling-scan base | 82.8K → **33.5K** = **−60%** |
| coverage-gate / flake-rerun / bug-repro base | ~82K → ~50–52K = **−33 to −38%** (clean-boot only) |
| build / audit (control) | ~44K/40K → unchanged (**0–1%**) ✓ |

**Caveats (honest):** the aggregate 39% is noisier than the per-turn base (cache_read
scales with per-task turn count); the per-turn base (−61%) is the clean, task-
independent proof. `cache_read` bills at ~0.1× input rate, so the *dollar* saving is
smaller than 39% — but the **context-window / quota** reduction (the operator's actual
concern) is the full ~39%.

---

## 3. Investigation: per-CLI telemetry (Slice A2) — INVESTIGATED, deliberately NOT built

**Question:** is telemetry accurate across all LLM CLIs? **Data:** real input/cache
fidelity is **claude-only** — agy: 0/1437 records with real input/cache; codex: 0/44.
By design (`isClaudeDriver` gates the transcript tier; others get output-only
scrollback → `source:none`, honestly marked "unmeasured", not faked-zero).

**But the deeper measure-first check flipped the conclusion:** every recent agy launch
exited **85 (quota-bench)** or 10 (probe-fail) — avg 4–11s, **zero exit-0** — and codex
is benched (`strikes=21`). agy/codex **never complete real work**; they quota-abort in
seconds and the fleet falls back to **claude (measured, 242s/launch)**. So their zeros
are *accurate* — the telemetry *does* capture the true token burn. Building codex/agy
collectors would be **unnecessary** (they consume ~0 real tokens) *and* **infeasible
from disk** (codex `token_count.info` is null in `~/.codex/sessions/*/rollout-*.jsonl`;
agy is a native binary with no session log and does not write `~/.claude` transcripts).

**Decision:** don't build collectors. Filed the right-sized guard instead —
`telemetry-coverage-tripwire-nonclaude-success` (0.6): WARN if a non-claude CLI ever
records a *successful* (exit 0, >60s) run as `source:none` — the tripwire for the
*latent* blind spot going live if quotas free up. **Lesson:** raw record COUNT (1437
agy) misleads; token weight = duration × success is what matters.

---

## 4. Slice C (turn-reduction) — designed + queued, deliberately NOT rushed

`cache_read = turns × base`; the base-cut wins are banked (~70% per-turn), so **turn
count is the frontier**. Evidence: a fault-localization transcript's Bash was **15/19
explore/search** (grep/find/ls) vs 2 read + 2 git — ~80% is code-hunting a **repo-map**
would shortcut. But: no existing codemap infra (it's a real Go build — `go/parser` AST
symbol extraction + relevance ranking + tree-SHA cache + prompt injection); the
realistic gain is **modest (~15–20% of turns** — the map shortens the *find* step, not
read/understand/iterate); and it carries **quality risk** (a stale/wrong map
*misdirects* the agent and hurts the deliverable). So the spec was sharpened and queued
(`tokenopt-repo-map-artifact` 0.77 → **0.85**, un-gated, with a hard quality-validation
acceptance criterion) rather than rushed at session tail. Build it as a focused effort
with TDD + multi-cycle validation of *both* turn-reduction *and* unchanged audit PASS
rate.

---

## 5. Operational GOTCHAs (hard-won this campaign)

- **Verify telemetry/parsing fixes against real on-disk data** — unit fixtures hid the
  string-vs-block content bug (§1.3). Write a throwaway real-`~/.claude` probe.
- **churn-discard nukes untracked main-tree files mid-loop.** A research doc written to
  `knowledge-base/` while the loop ran *vanished* — the loop's ship churn-discard cleans
  untracked files from the main tree. Only `.evolve/inbox/` is safe for loose writes;
  commit anything else promptly. (Extends `boundary_only_main_tree_writes` to untracked.)
- **`COMMIT_GATE_STALE` has two causes:** (a) `cmd | tail` masks the gate's non-zero
  exit so `&&` lets `ship` run against a stale attestation — use `gate=$?` after an
  un-piped command; (b) SIGTERM the loop then *immediately* ship → the graceful-shutdown
  checkpoint writes files between attest and ship — **wait for full loop exit (`pgrep`
  empty) before commit-gate**.
- **commit-gate reviewer selection is diff-type-aware:** `go-reviewer` satisfies the
  "review" capability only when Go files are in the diff; a **JSON/config-only diff needs
  `code-reviewer`**. Use `--reviewers "code-simplifier,code-reviewer"` for config ships.
- **Ship dance for a code change:** edit → `make -C go build` (AFTER final edits; `go
  build ./...` does NOT write `bin/evolve`) → `reset-sha -operator` (rebuild changes the
  SHA → re-pin) → `git add <explicit paths>` → `commit-gate run --reviewers …` →
  `EVOLVE_SHIP_AUTO_CONFIRM=1 ship --class manual "<positional msg>"`. `go/bin/evolve`
  and `.evolve/state.json` are gitignored (no commit bloat; pin not in the attested tree).
  Config-only ships skip the build/reset-sha (binary unchanged → SELF_SHA still valid).
- **Derive `--tools` from OBSERVED transcript usage, not `allowed_tools`** — the latter
  is a permission list that phases exceed under `--dangerously-skip-permissions`.
- **`--disable-slash-commands` disables the *skills* a phase may invoke via the Skill
  tool** — check actual Skill usage before applying it to a phase.

---

## 6. Queued follow-ups (in `.evolve/inbox/`)

| item | weight | why |
|---|---|---|
| `tokenopt-repo-map-artifact` | 0.85 | Slice C — turn-reduction via repo-map (§4) |
| `telemetry-coverage-tripwire-nonclaude-success` | 0.6 | latent per-CLI blind spot guard (§3) |
| `tokenopt-attribution-marker-anchored` | 0.80 | anchor attribution to the assembler marker to prevent cross-phase over-attribution |
| `worktree-path-propagation-fallback` | 0.55 | the `WORKTREE_PATH → ProjectRoot` degrade itself (§1.2), audit other consumers |

Remaining base-cut opportunity: extend `--tools` to more phases (deriving each set from
transcripts); extend clean-boot to the ~36 light claude phases not yet covered.

## 7. Sources
Live evolve-loop telemetry (post-fix); `claude` 2.1.212 `--help` + `-p --output-format
json` probes; parts 1–4 of this research dir; the resolver Strategy in
`go/internal/tokenusage`.
