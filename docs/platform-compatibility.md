# Platform Compatibility Guide

The evolve-loop is designed to be **platform-agnostic** — it runs on any LLM CLI or agent framework that supports file I/O, shell execution, and subagent spawning. This document maps evolve-loop concepts to platform-specific implementations.

## Core Requirements

Any host platform must provide these capabilities:

| Capability | Purpose | Required? |
|-----------|---------|-----------|
| **file-read** | Read project files, state.json, workspace reports | Yes |
| **file-write** | Write workspace files, state.json, instincts | Yes |
| **file-edit** | Modify existing files (Builder) | Yes |
| **shell** | Run bash commands (evals, git, scripts) | Yes |
| **search** | Search file contents (grep) and file names (glob) | Yes |
| **web-search** | Scout web research (external knowledge) | Optional |
| **web-fetch** | Fetch web pages for research | Optional |
| **subagent** | Launch independent agent sessions (Scout, Builder, Auditor, Operator) | Recommended |

**Minimum viable platform:** Any LLM with file I/O and shell access can run the evolve-loop. Subagent support enables parallel execution and context isolation but is not strictly required — a single-session orchestrator can run phases sequentially.

## Tool Mapping

Agent definitions include a `capabilities` field (platform-agnostic) and platform-specific `tools-*` fields:

### File Operations

| Capability | Claude Code | Gemini CLI | Codex CLI | OpenAI Agents | Generic |
|-----------|------------|-----------|-----------|--------------|---------|
| file-read | `Read` | `ReadFile` | `read_file` | `file_search` | `read_file` / `cat` |
| file-write | `Write` | `WriteFile` | `write_file` | `code_interpreter` | `write_file` / shell |
| file-edit | `Edit` | `EditFile` | `edit_file` | `code_interpreter` | `edit_file` / `sed` |

### Search

| Capability | Claude Code | Gemini CLI | Codex CLI | OpenAI Agents | Generic |
|-----------|------------|-----------|-----------|--------------|---------|
| search (content) | `Grep` | `SearchCode` | `grep` | `file_search` | `rg` / `grep` |
| search (files) | `Glob` | `SearchFiles` | `glob` | `file_search` | `find` / `fd` |

### Shell & Web

| Capability | Claude Code | Gemini CLI | Codex CLI | OpenAI Agents | Generic |
|-----------|------------|-----------|-----------|--------------|---------|
| shell | `Bash` | `RunShell` | `shell` | `code_interpreter` | system shell |
| web-search | `WebSearch` | `WebSearch` | N/A | `web_search` | `curl` + API |
| web-fetch | `WebFetch` | `WebFetch` | N/A | `web_search` | `curl` |

### Agent Orchestration

| Capability | Claude Code | Gemini CLI | Codex CLI | Cursor/Windsurf | Generic |
|-----------|------------|-----------|-----------|----------------|---------|
| spawn subagent | `Agent` tool | `spawn_agent` | N/A (single-session) | N/A | new LLM session |
| worktree isolation | `isolation: "worktree"` | manual `git worktree` | manual | manual | manual `git worktree` |
| parallel agents | multiple `Agent` calls | multiple `spawn_agent` | sequential | sequential | multiple API calls |

## Model Tier Mapping

The evolve-loop uses a 3-tier model abstraction. Map to your provider's models:

| Tier | Purpose | Anthropic | Google | OpenAI | Mistral | DeepSeek | Open-Weight |
|------|---------|-----------|--------|--------|---------|----------|------------|
| **tier-1** | Deep reasoning, complex tasks | Claude Opus 4.6 | Gemini 3.1 Pro | GPT-5.4 / o3-pro | Mistral Large 3 | DeepSeek R1 | Llama 4 Behemoth |
| **tier-2** | Standard development work | Claude Sonnet 4.6 | Gemini 3 Flash | GPT-5.3 Instant | Mistral Small 4 | DeepSeek V3 | Qwen 3.5 (397B MoE) |
| **tier-3** | Lightweight tasks, monitoring | Claude Haiku 4.5 | Gemini 3.1 Flash-Lite | GPT-5.4 nano | Ministral 3 (14B) | DeepSeek V3 (cached) | Qwen 3.5 (9B) |

**Override via `.evolve/models.json`:**
```json
{
  "tier-1": "your-provider/your-strongest-model",
  "tier-2": "your-provider/your-standard-model",
  "tier-3": "your-provider/your-lightweight-model"
}
```

## Platform-Specific Setup

### Claude Code
```bash
# Install via plugin marketplace (preferred)
/plugin evolve-loop

# Or manual install
bash install.sh
```
Full native support — all features work out of the box including parallel agents and worktree isolation.

### Gemini CLI
```bash
# Copy agent and skill files to Gemini's config
cp -r agents/ ~/.gemini/agents/
cp -r skills/ ~/.gemini/skills/
```
- Use `GEMINI.md` or `AGENTS.md` for project instructions (equivalent to `CLAUDE.md`)
- Subagent spawning via `spawn_agent` tool
- Manual worktree creation required for Builder isolation

### Codex CLI / Single-Session Platforms
For platforms without subagent support:
1. The orchestrator runs all phases sequentially in a single session
2. Each "agent launch" becomes a section of the conversation with the agent's prompt prepended
3. Worktree isolation is manual (`git worktree add` before Builder phase, cleanup after)
4. Parallel execution is not available — tasks run sequentially

### API-Only (Custom Orchestrator)
For building a custom orchestrator that calls LLM APIs directly:
1. Read `SKILL.md` for the full orchestration protocol
2. Each agent prompt is in `agents/evolve-*.md` — send as the system/user prompt
3. Context blocks are JSON — construct and pass as described in `phases.md`
4. Use your API's prompt caching (Anthropic: `cache_control`, Google: `cachedContent`, OpenAI: `store`)
5. Implement worktree isolation via `git worktree` commands

## Portability Checklist

When adapting the evolve-loop to a new platform:

- [ ] Map `capabilities` in agent frontmatter to platform tools
- [ ] Set up model tier mapping in `.evolve/models.json`
- [ ] Verify shell execution works (eval graders, git commands, scripts)
- [ ] Test file read/write/edit operations
- [ ] Configure subagent mechanism (or plan for single-session sequential mode)
- [ ] Set up worktree isolation (automatic or manual)
- [ ] Test prompt caching if available (static-first context ordering)
- [ ] Verify web search works (or disable research in Scout by setting long cooldowns)

## What's Truly Platform-Agnostic

These components work identically across all platforms:
- `.evolve/` directory structure (state.json, ledger.jsonl, instincts, genes, evals)
- Eval definitions and graders (bash commands)
- Scripts (`scripts/eval-quality-check.sh`, `scripts/cycle-health-check.sh`, `scripts/verify-eval.sh`)
- Agent prompts (the markdown body of each agent file)
- Phase logic (the orchestration protocol in SKILL.md and phases.md)
- Instinct extraction, memory consolidation, meta-cycle review
- Multi-armed bandit task selection
- All safety mechanisms (challenge tokens, hash-chain ledger, canary files)
