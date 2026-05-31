# Research: Concurrent-Worker & Swarm-Subagent Design (2026-05)

> Captured for ADR-0032 (multi-tmux-LLM-CLI swarm harness). This is the durable record so future
> devs don't re-research. Two streams: (A) how the three LLM CLIs run concurrent subagents; (B) how
> OSS multi-agent coding tools isolate + merge parallel workers. Each finding cites a source.
> **Confidence:** CLI vendor docs + Anthropic engineering + man7/tmux are high-confidence;
> throughput/disk figures from blogs are directional.

## A. The three LLM CLIs

### Claude Code — subagents / Task tool
- **Orchestrator-worker, fan-out by issuing multiple Task calls in one response.** Each subagent runs
  in **its own context window** with its own system prompt + tools; **only the final message returns**
  to the parent; the lead **synthesizes** the summaries. (code.claude.com/docs/sub-agents;
  anthropic.com/engineering/multi-agent-research-system)
- **Writers isolate via worktrees; readers don't need to.** Claude Code's isolation decision tree gates
  on "do the tasks touch the same files?" → worktree. Agent *teams* don't auto-isolate teammates, so
  the docs say **"partition the work so each teammate owns a different set of files"** — a requirement
  that exists ONLY for writers. (code.claude.com/docs/agents)
- **Read overlap is harmless** — two research subagents reading the same files just spend duplicate
  tokens; duplication is filtered at synthesis, never a correctness problem.
- **Anthropic is explicit that write-heavy work is a poor multi-agent fit**: "domains that require all
  agents to share the same context or involve many dependencies between agents are not a good fit…
  Coding, debugging fail this test. Research passes it." (anthropic.com/engineering)
- **Concurrency ~10 with rolling refill** (cuong.io subagent deep-dive); a fixed batch size is slower
  (waits for the whole wave). **No built-in resource cap** — 24 subagents froze a machine; users want
  `maxParallelAgents` (GitHub anthropics/claude-code#15487).
- **Token economics:** agents ≈ 4× chat tokens, multi-agent ≈ **15×**; token volume explained ~80% of
  BrowseComp variance — multi-agent "works mainly because it spends enough tokens." (anthropic.com)
- **Verification pass:** a separate CitationAgent re-checks the synthesized result. (anthropic.com)

### OpenAI Codex CLI
- CLI itself is single-agent; parallelism = **N instances each in its own git worktree** (community
  "agentmaxxing" idiom). The **Codex app** makes this first-class: worktrees under
  `$CODEX_HOME/worktrees`, **detached-HEAD by default** "so Codex can create several worktrees without
  polluting your branches", auto-prunes to ~15, snapshots before delete. (developers.openai.com/codex/app/worktrees)
- **Codex Cloud** runs tasks in parallel, each in an **isolated container with network disabled during
  execution**; output is a diff/PR a manager validates/merges. (developers.openai.com/codex/cloud)
- Sandbox: 3 modes (`read-only`/`workspace-write`/`danger-full-access`) × 4 approval policies; OS-native
  (macOS Seatbelt via `sandbox-exec`, Linux bubblewrap+Landlock+seccomp); `.git` stays read-only even
  in workspace-write. (developers.openai.com/codex/concepts/sandboxing)

### Gemini CLI
- **Subagents** (Apr 2026): Markdown+YAML in `~/.gemini/agents`, each own context/tools/MCP, `@agent`
  invocation. (developers.googleblog.com/subagents-have-arrived-in-gemini-cli)
- **Parallel writers explicitly cautioned**: the tracking issue states v1 "does not solve… agents
  making changes that interfere"; docs warn "exercise caution with parallel subagents for tasks that
  require heavy code edits — agents overwriting one another." **No built-in worktree isolation.**
  (github.com/google-gemini/gemini-cli#17749) — this validates our worktree-per-writer design as the
  gap-filler.
- Sandbox: 6 macOS Seatbelt profiles + container (`GEMINI_SANDBOX=docker|podman|sandbox-exec|...`).
  Docker/Podman auto-remove; LXC/runsc do not. (geminicli.com/docs/cli/sandbox)

## B. OSS parallel-coding-agent tools

- **uzi** (devflowinc, Go) — the closest analog. `uzi prompt --agents claude:3,codex:2` spawns N
  agents, each with **auto worktree + tmux session + a port from a configured range** (`$PORT` in the
  dev command); `uzi checkpoint <agent>` commits then **rebases the worktree into the current branch**;
  `uzi kill all`/`reset` teardown. (github.com/devflowinc/uzi) *[rebase conflict/authorship semantics
  unverified — not in README.]*
- **container-use** (dagger) — each agent in a **fresh container backed by a git branch**; `cu watch`
  gives a real-time command log ("what agents did, not what they claim"); `cu terminal/log/diff/merge/
  delete` lifecycle. (github.com/dagger/container-use)
- **claude-squad** (smtg-ai, Go TUI) — tmux session + worktree + branch per agent; **human-in-the-loop
  merge only**. The direct architectural twin of our tmux+worktree+multi-CLI choice. (github.com/smtg-ai/claude-squad)
- **claude-swarm** (stevegeek, Ruby) — a **YAML tree of agents over MCP**, isolation **per-directory**
  not per-worktree. (github.com/stevegeek/claude-swarm)
- **vibe-kanban** — Kanban over worktrees; one-click merge "rebases onto main, merges, cleans up the
  worktree." (vibekanban.com)
- **Convergent lessons:** worktree-or-container per writer is universal; tmux is the standard session
  substrate; orphan cleanup needs an explicit `clean`/`kill`/`reset` verb with unmerged-work
  protection (scottberrevoets.com).

### Merge-train / integration patterns
- **Parallel writers need a merge QUEUE, not just rebase** — "A green, B green, A+B red": rebase
  repairs text, not architectural intent. Recommended: (1) mechanical gate, (2) **authoring agent**
  rebases+resubmits, (3) **combined verification before advancing**. (ctx.rs/blog/merge-queue-for-agents)
- A 200-agent system uses an **intent-aware merger agent** gated by merge→integration-test→checkpoint.
  (agentfield.ai/blog/beyond-vibe-coding)
- **Test-gate-before-merge cut "agent broke something" ~80%.** (dev.to/battyterm)
- GitHub merge-queue / Mergify are the mature serialized-merge prior art — both gate on tests.

## Top optimizations → adopted vs deferred (for OUR harness)

| # | Optimization | Status |
|---|---|---|
| 1 | Serialized merge-train **gated on the acceptance check**, not just `git merge` | **Adopted** (v4 merge-train design) |
| 2 | **Authoring worker resolves its own conflicts** (re-dispatch once, then FAIL) | **Adopted** (v4) |
| 3 | **Writer/reader split** — readers no worktree, overlap OK; writers strict-disjoint | **Adopted** (v1 validator core) |
| 4 | **Default to single-agent; swarm opt-in per task value** (15× token cost) | **Adopted** (planner biased to N=1; budget caps) |
| 5 | **Provider rate-limit is the real ceiling** → reuse transient/quota classifier, back off | **Adopted** (reuse shipped `usage limit`/`429` markers) |
| 6 | **Detached-HEAD worktrees** to avoid branch-collision/pollution | **Considered** — we use named `cycle-<N>-w<i>` for ship's symbolic-ref; revisit if collisions appear |
| 7 | Bounded session pool + **snapshot-before-delete** teardown verb | **Adopted** (manifest + `evolve swarm reap`) |
| 8 | **Per-worker port** (branch-hash) for writers running servers | **Deferred** (flagged risk; not needed until a worker runs a server) |
| 9 | **Container-per-worker** stronger sandbox tier | **Deferred** (vN; tmux+worktree+sandbox-exec is the v1 tier) |
| 10 | `cu watch`-style live per-worker command log | **Partially** — reuse existing phase-observer; richer log is vN |

## Sources
Anthropic multi-agent-research-system; code.claude.com/docs {sub-agents, agents, agent-sdk/subagents};
theaiengineer.substack.com (architecture); cuong.io (parallelism cap); github anthropics/claude-code#15487;
developers.openai.com/codex {app/worktrees, cloud, concepts/sandboxing}; developers.googleblog.com
(gemini subagents); github google-gemini/gemini-cli#17749; geminicli.com/docs/cli/sandbox;
github {devflowinc/uzi, dagger/container-use, smtg-ai/claude-squad, stevegeek/claude-swarm}; vibekanban.com;
ctx.rs/blog/merge-queue-for-agents; agentfield.ai/blog/beyond-vibe-coding; dev.to/battyterm;
scottberrevoets.com; GitHub merge-queue / Mergify docs.

## Reflections (compounding — append per shipped increment)
- **v1 (2026-05-31):** the writer/reader asymmetry is the load-bearing insight; building the pure
  validator FIRST (reject-to-N=1 on any writer overlap) de-risks the whole feature before any
  concurrency exists. The hardest future risk is the merge-train (item 1/2) — keep it serialized.
