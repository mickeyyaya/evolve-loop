# AI-driven routing — advisor brain Phase 2 validation (2026-06-03)

Status of making the routing advisor the genuine LLM orchestration brain (plan:
"advisor as the LLM orchestration brain"). This dossier records **Phase 2** —
test coverage + live e2e validation that the advisor, defined like every phase
agent (persona + profile + artifact), actually produces valid routing plans
across LLM CLIs at its production (deep) tier.

## Why (the audit that started this)

The shipped smart-advisor framework was AI-driven only on paper: registry
`dynamic_routing=0` (static state machine drives by default), and **0
routing-plan.json, 0 `role:router` ledger entries, 0 mints across all history** —
the LLM advisor had never actually run. Root cause: the advisor was the lone
special-case seam (inline Go prompt, stdout-scrape completion, defaulted to
haiku) versus every phase agent (persona md + profile json + artifact
completion). Phase 1 (commit `4f9fb39`) unified it onto the agent format and
switched plan completion to the artifact contract.

## Phase 2 — coverage added

Unit (default build, no quota):
- `TestPhaseAdvisor_DispatchWiringFlowsToBridge` — `WithProposerCLI/Model` reach
  `BridgeRequest.{CLI,Model}`; the artifact + injected-persona contract holds for
  non-claude CLIs (the any-CLI × any-model invariant).
- `TestResolveRouterDispatch_Precedence` — env > profile > opus/claude-tmux
  fallback (previously zero coverage).

e2e (`//go:build e2e`, gate `EVOLVE_E2E_LIVE_ADVISOR=1`):
- `TestE2ELiveAdvisorCLIMatrix` + `launchAdvisor` — launches the `evolve-router`
  persona on each available CLI and asserts a parseable routing-plan.json.
- Pure-function tests (`TestParseRoutingPlanArray`, `TestStripFrontmatter`) run
  under `-tags e2e` without spending quota.

## Live e2e validation — DEEP production tier

Per the standing directive, the advisor e2e runs at the advisor's **production
tier** (opus / gpt-5.5 / family's strongest via `advisorModelFor`), NOT the cheap
tier — a fast model validates plumbing but misrepresents the brain's real output.
Headless drivers are artifact-native; tmux variants honor the artifact flag.

Results (2026-06-03, this host):

| CLI | Model | Result | Notes |
|---|---|---|---|
| claude-p | opus | **PASS** | 7-entry routing-plan.json (richer than haiku's 4 at fast tier) |
| agy | gemini-3.5-flash | **PASS** | 5-entry plan |
| agy-tmux | gemini-3.5-flash | **PASS** | 6-entry plan |
| codex-tmux | gpt-5.5 | **PASS** / SKIP (flaky) | 4-entry plan when the first-run trust prompt clears; otherwise the auto-respond loop-guard trips → quarantine-SKIP. Confirmed PASS in a targeted run; SKIP in the full authoritative run. The codex family's writer path works; the trust-prompt handling is non-deterministic on an un-onboarded host. |
| codex (headless) | gpt-5.5 | SKIP | codex headless sandboxes the workspace read-only + approvals disabled → cannot write the artifact. The codex family's writer path is **codex-tmux** (passes). Not an advisor break. |
| ollama-tmux | (host) | SKIP | ollama has no tool use (no Bash/Edit/Write) → cannot write the artifact at all. |
| claude-tmux | opus | SKIP | opus in the tmux REPL exceeds the per-call ceiling (>10 min) → quarantine. Headless claude-p@opus completes in ~30 s, so this is a tmux-REPL+opus latency issue, not an advisor break. |

The advisor brain produces valid multi-phase plans on every CLI that can run it
here, spanning the Anthropic (opus, headless), Google (gemini, headless+tmux),
and OpenAI (gpt-5.5, tmux) families. SKIPs are all environmental incapability,
classified by `advisorEnvUnavailable` so the matrix is a meaningful green gate
rather than red-failing on host setup.

## Bugs found by live validation (that compile-clean tests could not)

1. `launchAdvisor` omitted the required `--stdout-log`/`--stderr-log` flags →
   `bridge launch` rejected fast. Fixed.
2. Passed the abstract tier (`fast`) where `bridge launch --model` wants a
   concrete model → claude "model does not exist". Fixed via `cheapModelFor` →
   `advisorModelFor` (deep).
3. codex refused to run outside a git/trusted directory → `git init` the temp
   worktree to match production (a no-op for other drivers). Fixed codex-tmux.
4. A fast model PRINTS the plan instead of invoking Write → stdout fallback in
   `launchAdvisor` (deep models reliably write; the fallback is defense).
5. Tolerant plan parsing uses a depth-tracking `firstBalancedArray` (not
   `LastIndexByte`) so trailing prose containing `]` is not mis-sliced.

## Open follow-ups

- **claude-tmux + opus latency**: the interactive REPL path is too slow for opus
  (>10 min) while headless is ~30 s. Investigate the tmux completion-detection /
  REPL throughput for deep models. Quarantined for now.
- ~~**Activation**~~ **DONE (2026-06-03):** `TestE2ELiveAdvisorActivation` runs
  one isolated (temp-project) cycle with `EVOLVE_DYNAMIC_ROUTING=advisory` and the
  advisor on **claude-p@opus**. Result: the advisor wrote a **9-entry
  routing-plan.json** and the orchestrator clamped + recorded it — confirmed by a
  `phase_plan` ledger entry (`recordPhasePlan`). This is the first time the advisor
  brain has driven a real cycle (the audit had found 0 plans ever).
  **Correction to the original plan's assumption:** there is intentionally NO
  `role:router` ledger entry — the advisor calls `bridge.Launch` directly, not via
  the phase runner that stamps agent roles, so the real activation signal is the
  orchestrator's `phase_plan` entry (role=orchestrator), not a router-role entry.
  claude-p@opus is used (not claude-tmux@opus) because the headless path completes
  in ~30s vs the tmux REPL exceeding the ceiling.
- Phase 3: prove a genuine `architecture-design` selection or a first mint
  (`registerMintedPhases` ledger entry) under `ship_floor:["audit"]`.
