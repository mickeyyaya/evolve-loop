# Package design notes

One page per Go package directory, holding what the package's code cannot say: its purpose, why it is shaped the way it is, its invariants with their reasons, and what it taught. Each page is named by the directory under `go/` with `/` replaced by `-`. The rule and its reasons are in [code-comments](../../conventions/code-comments.md). The comment-reduction workstream fills these pages as it removes knowledge from comments ([plan](../../plans/comment-reduction-2026-09.md)).

| Package | What it is | Notes |
|---|---|---|
| `internal/bridge/panestream` | reads tmux pane snapshots of an interactive LLM REPL | [internal-bridge-panestream.md](internal-bridge-panestream.md) |
| `internal/config` | resolves the routing configuration once, at the composition root | [internal-config.md](internal-config.md) |
| `internal/evalgate` | the verified checks that replace prose contracts between phases | [internal-evalgate.md](internal-evalgate.md) |
| `internal/fleet` | plans and runs concurrent, file-disjoint cycle lanes | [internal-fleet.md](internal-fleet.md) |
| `internal/guards` | the in-process trust kernel: the six guards `evolve guard` runs | [internal-guards.md](internal-guards.md) |
| `internal/inboxbatch` | the inbox item model and the deterministic half of task selection | [internal-inboxbatch.md](internal-inboxbatch.md) |
| `internal/looppreflight` | the readiness gate `evolve loop` runs before the first wave | [internal-looppreflight.md](internal-looppreflight.md) |
| `internal/phasecoherence` | drift reports between the hand-edited surfaces that define a phase | [internal-phasecoherence.md](internal-phasecoherence.md) |
| `internal/policy` | loads `.evolve/policy.json` into resolved configuration | [internal-policy.md](internal-policy.md) |
| `internal/profiles` | loads the agent profiles in `.evolve/profiles/` | [internal-profiles.md](internal-profiles.md) |
| `internal/prompts` | loads agent personas and skill docs | [internal-prompts.md](internal-prompts.md) |
| `internal/recovery` | the decisions of the Phase Recovery Pipeline | [internal-recovery.md](internal-recovery.md) |
| `internal/router` | the deterministic phase-routing kernel | [internal-router.md](internal-router.md) |
| `internal/triagecap` | bounds the coverage floors triage may commit per cycle | [internal-triagecap.md](internal-triagecap.md) |
| `internal/cli/phasecmd` | the `evolve phase` and `evolve phases` commands | [internal-cli-phasecmd.md](internal-cli-phasecmd.md) |
