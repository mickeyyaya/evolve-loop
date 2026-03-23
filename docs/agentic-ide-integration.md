> **Agentic IDE Integration** — Reference doc on how AI agents integrate with development environments. Covers integration tiers, repository-embedded memory, feature comparison, mapping to evolve-loop, implementation patterns, prior art, and anti-patterns.

## Table of Contents

- [Integration Tiers](#integration-tiers)
- [Repository-Embedded Memory](#repository-embedded-memory)
- [Feature Comparison](#feature-comparison)
- [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
- [Implementation Patterns](#implementation-patterns)
- [Prior Art](#prior-art)
- [Anti-Patterns](#anti-patterns)

---

## Integration Tiers

Classify agentic IDE integrations by depth of environment access, control surface, and deployment model.

| Tier | Description | Examples | Editor Control | Terminal Access | Deployment Model |
|---|---|---|---|---|---|
| **Deep IDE** | Agent embedded in the editor with full control over files, tabs, debugging, and UI | Cursor, Windsurf | Full — open/close files, navigate symbols, control debugger | Built-in terminal | Desktop app with bundled LLM integration |
| **Extension-Based** | Agent runs as an IDE plugin, constrained by the host editor's extension API | GitHub Copilot, Cline, Continue | Partial — insert/replace text, open files; no debugger control | Via extension API or user terminal | Plugin distributed through marketplace |
| **CLI-Native** | Agent runs in the terminal, reads/writes files directly, orchestrates via shell | Claude Code, aider | None — operates on files, not editor UI | Native — agent IS the terminal process | CLI binary or npm package |
| **Headless** | Agent runs as a remote service with no local UI; communicates via API or webhook | Devin, GitHub Copilot Workspace, SWE-Agent | None — uses git and file I/O | Remote sandbox or container | Cloud-hosted, accessed via web UI or API |

### Tier Selection Guidance

| Scenario | Recommended Tier |
|---|---|
| Interactive pair programming with visual feedback | Deep IDE |
| Augment existing editor workflow without switching tools | Extension-Based |
| Autonomous multi-file refactoring or CI-integrated agents | CLI-Native |
| Fully autonomous task completion with no human in the loop | Headless |

---

## Repository-Embedded Memory

Agents read project-level instruction files to maintain persistent context across sessions. These files act as agent memory embedded in the repository.

| File | Agent | Format | Scope | Persistence |
|---|---|---|---|---|
| `CLAUDE.md` | Claude Code | Markdown with structured sections | Project-wide instructions, conventions, task priorities | Committed to repo; survives across sessions |
| `.cursorrules` | Cursor | Plain text or structured rules | Editor behavior, code style, generation preferences | Committed to repo |
| `.github/copilot-instructions.md` | GitHub Copilot | Markdown | Code generation guidance, project conventions | Committed to repo |
| `.aider.conf.yml` | aider | YAML config | Model selection, file filtering, conventions | Committed to repo |
| `.continuerules` | Continue | Markdown | Code generation rules, project context | Committed to repo |
| `.windsurfrules` | Windsurf | Markdown | Agent behavior, project conventions | Committed to repo |

### Memory Layering

| Layer | Location | Override Priority | Example |
|---|---|---|---|
| Global | `~/.claude/CLAUDE.md` or equivalent | Lowest | Personal coding style preferences |
| Project | `./CLAUDE.md` in repo root | Medium | Project-specific conventions and task priorities |
| Directory | `./src/CLAUDE.md` in subdirectory | Highest | Module-specific instructions |

---

## Feature Comparison

Compare agent capabilities across integration tiers and specific tools.

| Feature | Cursor | Windsurf | Copilot | Cline | Claude Code | aider | Devin |
|---|---|---|---|---|---|---|---|
| **File editing** | Multi-file with diff view | Multi-file with diff view | Single-file inline | Multi-file with approval | Multi-file with approval | Multi-file auto-commit | Multi-file autonomous |
| **Terminal access** | Built-in | Built-in | None | Via VS Code terminal | Native shell | Shell commands | Remote sandbox |
| **Web search** | Built-in | Built-in | Limited | Via MCP plugins | Via MCP or tools | None | Built-in browser |
| **MCP support** | Yes | Yes | No | Yes | Yes | No | No |
| **Multi-file editing** | Yes — composer mode | Yes — cascade mode | Limited | Yes | Yes | Yes | Yes |
| **Worktree isolation** | No | No | No | No | Yes — git worktrees | No | Yes — sandboxed env |
| **Autonomous mode** | Partial | Partial | No | Yes — auto-approve | Yes — bypass permissions | Yes — auto-commit | Yes — fully autonomous |
| **Project memory** | `.cursorrules` | `.windsurfrules` | `copilot-instructions.md` | `.clinerules` | `CLAUDE.md` | `.aider.conf.yml` | Session-based |
| **Cost model** | Subscription | Subscription | Subscription | API key (BYOK) | API key or subscription | API key (BYOK) | Subscription |

---

## Mapping to Evolve-Loop

Map evolve-loop concepts to the agentic IDE integration landscape.

| Evolve-Loop Concept | IDE Integration Equivalent | Implementation |
|---|---|---|
| **CLI-native agent** | Integration tier: CLI-Native | Evolve-loop runs as a terminal process via Claude Code; no editor dependency |
| **CLAUDE.md** | Repository-embedded memory | Project instructions, task priorities, and autonomous execution rules persist in repo |
| **Scout agent** | Code search and analysis | Scout reads codebase, identifies improvement opportunities, produces `scout-report.md` |
| **Builder agent** | Code generation and editing | Builder implements changes using file editing tools, produces `build-report.md` |
| **Auditor agent** | Code review and verification | Auditor validates changes against eval criteria, produces `audit-report.md` |
| **Skills** | IDE commands / extensions | Skills (`.claude/skills/`) act as reusable commands, equivalent to IDE command palette entries |
| **Plugin publishing** | Marketplace distribution | Skills published to the Claude Code skill registry mirror extension marketplace distribution |
| **Worktree isolation** | Sandboxed editing | Each evolve-loop cycle runs in an isolated git worktree to prevent conflicts |
| **Phase gates** | Pre-commit hooks / CI checks | `phase-gate.sh` enforces deterministic quality gates at every phase boundary |
| **Gene system** | Configuration presets | Genes configure agent behavior parameters, similar to IDE settings profiles |

---

## Implementation Patterns

### How Agents Read Project Context

| Pattern | Description | Used By |
|---|---|---|
| **File convention** | Read well-known files (`CLAUDE.md`, `.cursorrules`) at session start | All agents with repo memory |
| **Glob scanning** | Scan directory tree for relevant files by pattern | Claude Code, Cline |
| **LSP integration** | Query language server for symbols, types, diagnostics | Cursor, Windsurf, Copilot |
| **Git history** | Read commit messages, diffs, and blame for context | aider, Claude Code |
| **Embedding index** | Build vector index of codebase for semantic search | Cursor, Copilot Workspace |

### How Agents Apply Changes

| Pattern | Description | Trade-offs |
|---|---|---|
| **Exact string replacement** | Find unique string in file, replace with new string | Safe — fails if string not unique; requires reading file first |
| **Full file rewrite** | Write entire file contents | Simple but risky — can lose concurrent edits |
| **Diff/patch application** | Generate unified diff, apply with patch tool | Precise but fragile — context lines must match exactly |
| **AST transformation** | Parse code into AST, modify nodes, regenerate | Robust but complex; language-specific tooling required |
| **Editor API** | Use IDE extension API to insert/replace text | Integrated undo support; constrained by API surface |

### How Agents Verify Results

| Pattern | Description | Reliability |
|---|---|---|
| **Run tests** | Execute test suite after changes | High — catches regressions; requires existing tests |
| **Lint/typecheck** | Run linter or type checker on modified files | Medium — catches syntax and type errors |
| **Build verification** | Run full build to verify compilation | Medium — catches import and dependency errors |
| **Eval scoring** | Run custom eval functions against output | High — domain-specific quality measurement |
| **Snapshot comparison** | Compare output against known-good baseline | High — catches unexpected behavioral changes |
| **Self-review** | Agent reviews its own changes before committing | Low-medium — useful as a sanity check, not a gate |

---

## Prior Art

| Tool | Category | Key Innovation | Limitation |
|---|---|---|---|
| **Cursor** | Deep IDE | Tab-complete with full codebase context; composer mode for multi-file edits | Proprietary; macOS/Windows/Linux desktop only |
| **Copilot Workspace** | Headless | Issue-to-PR pipeline with plan, implement, verify stages | Limited availability; tied to GitHub ecosystem |
| **Claude Code** | CLI-Native | Terminal-first agent with CLAUDE.md project memory, MCP extensibility, worktree isolation | No GUI; requires terminal comfort |
| **aider** | CLI-Native | Git-native workflow with auto-commit; supports many LLM providers | No built-in web search or MCP |
| **Cline** | Extension-Based | VS Code extension with MCP support and auto-approve mode | Depends on VS Code; BYOK cost model |
| **Windsurf** | Deep IDE | Cascade mode for multi-step autonomous editing with memory | Proprietary editor fork |
| **Devin** | Headless | Fully autonomous software engineer with browser, terminal, editor | Expensive; opaque decision-making; variable quality |
| **SWE-Agent** | Headless | Open-source agent framework for software engineering tasks | Research-oriented; requires setup and configuration |
| **Continue** | Extension-Based | Open-source IDE extension supporting multiple LLM providers | Smaller ecosystem than Copilot or Cursor |

---

## Anti-Patterns

Avoid these patterns when building or using agentic IDE integrations.

| Anti-Pattern | Description | Consequence | Mitigation |
|---|---|---|---|
| **IDE lock-in** | Build agent that only works with one specific editor | Users cannot switch tools; limits adoption | Use file-based interfaces (`CLAUDE.md`, CLI) that work across editors |
| **Ignoring project conventions** | Agent generates code without reading project memory files | Style violations, wrong patterns, broken builds | Read `CLAUDE.md` / `.cursorrules` / equivalent at session start |
| **Overwriting uncommitted work** | Agent writes files without checking git status for uncommitted changes | User loses work in progress | Check `git status` before modifying files; use worktree isolation |
| **No undo support** | Agent applies changes with no way to revert | User stuck with broken state if agent makes mistakes | Use git commits as checkpoints; prefer exact replacements over full rewrites |
| **Unbounded autonomy** | Agent runs indefinitely without human checkpoints | Runaway token costs; compounding errors | Implement phase gates, token budgets, and cycle limits |
| **Context window stuffing** | Agent reads entire codebase into context instead of targeted search | Degraded output quality in last 20% of context; high cost | Use search tools (grep, glob, LSP) to find relevant code; read selectively |
| **Ignoring verification** | Agent makes changes without running tests or linting | Silent regressions and broken builds | Run tests and linters after every change; fail loudly |
| **Single-agent monolith** | One agent handles planning, coding, and review | No separation of concerns; harder to debug failures | Use specialized agents (Scout, Builder, Auditor) with clear boundaries |
