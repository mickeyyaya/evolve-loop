# Evolve Loop Research Report
**Cycle**: 1
**Date**: 2026-03-13
**Agent**: Researcher
**Project**: evolve-loop (Claude Code plugin — AI agent orchestration)

---

## Executive Summary

The AI agent orchestration space is undergoing rapid maturation. 2025 was the "year of agents"; 2026 is the "year of agent harnesses." The market has shifted from standalone tool experimentation to production-grade infrastructure concerns: reliability, eval-driven quality, cost optimization, and security hardening. The evolve-loop project sits at the intersection of two high-signal trends: self-improving agents and the Claude Code plugin ecosystem, both of which are experiencing significant community investment and commercial interest.

---

## 1. AI Agent Orchestration Framework Landscape

### Market Leaders (2026)

| Framework | Paradigm | Differentiator | Best For |
|-----------|----------|----------------|----------|
| LangGraph | Graph-based state machine | Explicit control, audit trails | Complex branching, regulated industries |
| CrewAI | Role-based collaboration | Rapid prototyping, 60%+ Fortune 500 adoption | Task-oriented team workflows |
| AutoGen / Microsoft Agent Framework | Conversation-driven | Code execution, October 2025 merge with Semantic Kernel → GA Q1 2026 | Creative/dev automation |
| Google ADK | Full-stack open source | Built-in eval, step-trajectory tracking | Google Cloud-native deployments |

### Key 2026 Trends

1. **Graph-based convergence**: Every major framework now adopts graph/DAG execution models; LangGraph pioneered, others follow.
2. **Enterprise maturity**: CrewAI processed 1.1 billion agent actions in Q3 2025 alone. The shift from prototype to production is complete.
3. **No-code democratization**: Visual platforms (Vellum, Lindy, O-MEGA) enable non-technical users to build agents. Market expanding beyond developers.
4. **Mega-vendor consolidation**: Microsoft merging AutoGen + Semantic Kernel, Google shipping ADK, AWS investing in DevOps agents — signals lock-in risk and opportunity for open-source alternatives.
5. **Multi-modal expansion**: Vision, voice, sensor inputs joining text reasoning pipelines.

### Market Gaps / Opportunities

- **Standardization**: No universal API for interoperable agent components across frameworks — opportunity for a bridge layer.
- **Reliability**: Hallucination propagation in multi-agent chains remains unsolved.
- **Cost efficiency**: Multi-turn API call overhead is a major production pain point.
- **Explainability**: Regulators demanding decision audit trails that current frameworks do not provide natively.
- **Agent marketplaces**: Reusable, pre-vetted agent components are an emerging economy (directly relevant to evolve-loop's plugin distribution model).
- **Memory systems**: Sophisticated long-term knowledge management balancing retention with context relevance — an area where current frameworks are weakest.

---

## 2. Claude Code Plugin Ecosystem

### Current Capabilities (as of Claude Code v1.0.33+)

The Claude Code plugin system supports:
- **Skills** (`skills/SKILL.md`): Model-invoked context-aware capabilities, auto-triggered by task context
- **Commands** (`commands/`): User-invoked Markdown-defined slash commands
- **Agents/Subagents** (`agents/`): Isolated AI instances with own system prompt, tool permissions, and model selection
- **Hooks** (`hooks/hooks.json`): PreToolUse, PostToolUse, Stop event handlers — both `type: prompt` (single LLM call) and `type: agent` (multi-turn with tool access)
- **MCP servers** (`.mcp.json`): External tool integration
- **LSP servers** (`.lsp.json`): Real-time code intelligence
- **Default settings** (`settings.json`): Can set a default agent for the main thread

### Ecosystem Momentum

- Community projects: 100+ subagent collections, 120+ plugin repositories, 35+ curated skill packs
- Commercial ecosystem emerging: Composio, Firecrawl, and others publishing top-plugin lists
- Official marketplace via `claude.ai/settings/plugins/submit` and `platform.claude.com/plugins/submit`
- Plugin namespacing prevents conflicts; `--plugin-dir` enables local dev/test without reinstall

### Key Best Practices Identified

1. **Start standalone, promote to plugin**: Iterate in `.claude/` first, convert to plugin when sharing-ready
2. **Agent hooks over prompt hooks** when verification requires file inspection or command execution
3. **Harness-first thinking**: The harness (lifecycle, approval gates, cost controls) determines success more than model selection
4. **Model routing for cost**: Haiku 4.5 for exploration subagents + Sonnet 4.6 for implementation = 40-50% cost reduction
5. **One objective per session**: Explicit session goals dramatically improve agent coherence
6. **Hooks for mandatory behavior**: Use hooks (not prompts) for anything that must always execute

### Notable Community Projects (direct competitors/comparators)

| Project | Key Differentiator | Relevant to evolve-loop |
|---------|-------------------|------------------------|
| `affaan-m/everything-claude-code` | Instinct-based learning, `/evolve` command, eval harness, 65+ skills, 997 tests | HIGH — closest analog; already implements similar concepts |
| `closedloop-ai/claude-plugins` | Plan-first SDLC, LLM-as-judge quality eval, self-learning pattern capture | HIGH — eval-driven loop with judge mechanism |
| `miles990/self-evolving-agent` | Autonomous goal achievement through iterative learning | HIGH — direct overlap |
| `ruvnet/ruflo` | Multi-agent swarms, distributed intelligence, RAG integration | MEDIUM — orchestration layer |
| `wshobson/agents` | Multi-agent orchestration for Claude Code | MEDIUM — pattern reference |
| `rohitg00/awesome-claude-code-toolkit` | 135 agents, 42 commands, 120 plugins — comprehensive catalog | LOW — reference/discovery |

---

## 3. Multi-Agent Pipeline Patterns

### Canonical Patterns (Azure Architecture Center, 2026-02-12)

1. **Sequential/Assembly Line**: Linear, deterministic, easy to debug. Agents pass output downstream.
2. **Coordinator/Dispatcher**: One orchestrator dispatches to specialists. Best for routing by intent/domain.
3. **Parallel Processing**: Independent agents work simultaneously; merge results. Best for speed on decomposable tasks.
4. **Group Chat**: Multiple agents converse toward consensus. High quality, high cost.
5. **Handoff/Handshake**: Agent A explicitly transfers control to Agent B with context packet.
6. **Magentic/Magnetic**: Dynamic agent recruitment based on task needs — most flexible, least predictable.

### Eval-Driven Development (EDD)

EDD mirrors TDD:
1. Define eval scenario (expected behavior)
2. Run agent — it fails (RED)
3. Modify prompts/tools/routing until it passes (GREEN)
4. Add to eval suite
5. Regressions caught automatically on future changes

**Key metric categories for agent evals**:
- **Trajectory evals**: Did the agent take the correct steps, not just produce the right output?
- **Final response quality**: LLM-as-judge scoring against rubric
- **Tool use accuracy**: Were the right tools called with correct parameters?
- **Cost efficiency**: Token spend per successful task
- **pass@k**: Success rate over k attempts (reliability measurement)

**Google ADK** (Agent Development Kit) provides built-in eval for both final response quality and step-by-step execution trajectory — the most complete open-source eval framework as of Q1 2026.

### Self-Improvement Architectures

- **Gödel Agent**: Recursively updates both policy and meta-learning mechanisms via LLM-proposed code/strategy modifications
- **AlphaEvolve** (Google DeepMind, May 2025): Evolutionary coding agent — mutate/combine algorithms, select best candidates for further iteration
- **Instinct-based learning** (`everything-claude-code`): Sessions generate confidence-scored "instincts" that cluster into reusable skills via `/evolve`
- **ICLR 2026 Workshop on Recursive Self-Improvement**: Academic community formalizing this as a research discipline

---

## 4. Competitor Intelligence

### Claude Code vs. Cursor vs. Windsurf (2026 State)

| Dimension | Claude Code | Cursor | Windsurf |
|-----------|------------|--------|----------|
| Context window | 150K+ tokens | 60-80K tokens | 60-80K tokens |
| IDE integration | None (CLI only) | VS Code native | VS Code fork |
| Agent/automation | Strongest (harness model) | Good (Composer) | Good (Cascade/Flows) |
| Market pricing | $50-200/month (API) | $20/month | $15/month |
| Best use case | Complex multi-file, enterprise | Autocomplete + small edits | Persistent session context |

**Claude Code's gaps**: No autocomplete, no inline hover, no IDE integration. These are known and intentional but represent market perception risk.

**Claude Code's moat**: 150K+ token context window, harness-extensibility, plugin ecosystem, and the evolve-loop opportunity (no competitor has a self-improving harness with comparable depth).

### Cursor's Hidden Differentiator
`.mdc files take precedence over .cursorrules` — Cursor's rule system is evolving similarly to Claude Code's `.claude/` directory patterns. Competitors are converging on the same configuration-as-code paradigm.

### Devin (Cognition AI)
Positioned as fully autonomous software engineer. Primary threat vector: enterprise buyers who want zero-human-in-the-loop for routine tasks. Claude Code counters with superior context handling and the plugin/harness extensibility that Devin lacks.

### Aider
CLI tool with strong git integration and multi-model support. Strength for cost-sensitive developers who want to swap underlying models. Claude Code counters with superior native Claude integration and richer plugin ecosystem.

---

## 5. Security Intelligence

### Threat Landscape (OWASP 2025 Top 10 for Agentic Applications)

1. **Agent Goal Hijack** — Indirect prompt injection via external data (emails, tickets, web pages)
2. **Tool Misuse** — Overly permissive tool grants exploited by injected instructions
3. **Memory Poisoning** — Malicious data persisted to influence future agent sessions
4. **Data Exfiltration** — Sensitive data leaked through tool calls and outputs
5. **Denial of Wallet** — Unbounded loops causing runaway API costs
6. **Cascading Failures** — Compromised sub-agents propagating attacks through multi-agent chains
7. **Privilege Escalation** — Agents acquiring permissions beyond their operational scope
8. **Supply Chain Compromise** — Malicious plugins/skills introduced via marketplaces

### Critical 2025-2026 CVEs Relevant to Plugin Systems
- **CVE-2025-3248** (Langflow): Unauthenticated code injection → RCE
- **CVE-2025-64496** (Open WebUI): Malicious model server → arbitrary JS → token theft
- **Cline npm incident**: Prompt injection via CI/CD pipeline with shell access → unauthorized package publishing

**Implication for evolve-loop**: Any plugin that accepts external input (research queries, web-fetched content, PR comments) into its pipeline is a potential injection vector.

### The "Lethal Trifecta"
Three conditions that together create critical vulnerability:
1. Sensitive data in context
2. Untrusted content ingested
3. External communication channels available

A self-improving agent with web search, file write access, and ledger persistence satisfies all three conditions. This is the primary threat model for evolve-loop.

### Risk-Based Action Approval Framework

| Risk Level | Action Type | Control |
|-----------|-------------|---------|
| LOW | Read-only operations | Auto-approve |
| MEDIUM | Write/API calls | Log + threshold limits |
| HIGH | Financial, deletion, external comms | Human-in-loop gate |
| CRITICAL | Irreversible, security-sensitive | Block + alert |

### Five-Boundary Security Model (2026)
1. **Identity**: Token/credential scope and rotation
2. **Execution**: Tool and runtime isolation constraints
3. **Persistence**: State/memory/config modification controls
4. **Instruction**: Separation of trusted directives from untrusted data flows
5. **Supply Chain**: Plugin/skill/extension provenance verification

---

## 6. Recommendations

### Quick Wins (Cycle 1-2)

1. **Add eval baseline infrastructure**: Create a minimal `evals/` directory structure with at least one eval scenario per agent role. Even a single passing eval provides the regression safety net needed for safe self-modification.

2. **Implement denial-of-wallet guardrails**: Add hard limits on tokens-per-cycle, API calls-per-hour, and max concurrent subagents. The "Denial of Wallet" attack is a direct risk for any evolve loop.

3. **Adopt risk-based action approval**: Classify all agent tool calls by the four-level framework (LOW/MEDIUM/HIGH/CRITICAL). Hook the HIGH/CRITICAL categories for human-in-loop or dry-run-first behavior.

4. **Add instruction/data separation**: Any content fetched from the web, read from PRs, or received from external tools must be tagged as "untrusted data" and never directly appended to the system prompt or agent instructions without sanitization.

5. **Model routing optimization**: Use Haiku 4.5 for the Researcher role's exploratory web searches; reserve Sonnet 4.6 for synthesis and code generation. Estimated 30-40% cost reduction at no quality loss.

### Strategic Moves (Cycle 3-6)

6. **Implement trajectory evals, not just output evals**: Log every tool call and decision step. Build evals that check whether the agent took the right path, not just whether the output looks correct. This is what separates production-grade self-improvement from brittle prompt hacking.

7. **Build a standardization bridge**: The market gap in cross-framework interoperability is real. If evolve-loop can expose its agent definitions in a format compatible with LangGraph/CrewAI/AutoGen, it can become a hub rather than an island.

8. **Instinct export/import as a social feature**: Following `everything-claude-code`'s pattern, evolve-loop's learned instincts and skill improvements should be exportable. This creates a community flywheel: users share battle-tested instincts, the plugin improves for everyone.

9. **Publish to Anthropic official marketplace**: Submission path exists at `platform.claude.com/plugins/submit`. High discoverability impact for a well-documented, security-reviewed plugin.

10. **Memory integrity verification**: As the evolve loop accumulates instincts and state across cycles, treat the `state.json` and `ledger.jsonl` as security surfaces. Add cryptographic integrity checks (hash-on-write, verify-on-read) to prevent memory poisoning.

### Risk Mitigations (Immediate)

11. **Sandbox web-fetched content**: The Researcher role fetches arbitrary web content. Treat all fetched content as untrusted, run it through a content classifier before allowing it to influence agent instructions or code generation.

12. **Audit plugin supply chain**: Any external MCP server, skill, or plugin loaded by evolve-loop should be pinned to a specific version/hash. Dynamic loading from unverified sources is a supply chain attack vector.

13. **Rate-limit the self-modification surface**: The evolve loop can write new skills, modify hooks, and update agent definitions. Gate these writes behind a validation step — at minimum, a syntax check and a security scan for injected instructions.

14. **Egress allowlisting for subagents**: Subagents with web access should have an explicit allowlist of approved domains. Unbounded web access is the primary exfiltration vector.

### Watch List

- **Microsoft Agent Framework GA (Q1 2026)**: Combined AutoGen + Semantic Kernel with production SLAs. If enterprise adoption accelerates, it may commoditize the orchestration layer evolve-loop relies on. Monitor for API compatibility or integration opportunities.
- **ICLR 2026 Recursive Self-Improvement Workshop**: Academic papers due to publish; may contain novel techniques directly applicable to the evolve loop's self-improvement mechanism.
- **`everything-claude-code` v3 release**: The closest analog project is actively evolving. Track their instinct-clustering and verification loop improvements.
- **OWASP LLM Top 10 2026 update**: OWASP is updating the list specifically for agentic applications; new vulnerability categories expected.
- **Anthropic plugin marketplace growth**: Early marketplace presence = significant discovery advantage. Track approval timelines and submission requirements.
- **Cursor `.mdc` rule system evolution**: If Cursor adds a plugin/marketplace model, it becomes a potential distribution channel for evolve-loop skills.

---

## 7. Raw Intelligence Appendix

### Sources Consulted
- Anthropic Claude Code plugin docs (`code.claude.com/docs/en/plugins`)
- Microsoft Azure Architecture Center — AI Agent Orchestration Patterns (updated 2026-02-12)
- OWASP AI Agent Security Cheat Sheet (2025)
- Penligent AI Agents Hacking in 2026 article
- O-Mega.ai LangGraph vs CrewAI vs AutoGen Top 10 Frameworks 2026
- DEV Community: Cursor vs Windsurf vs Claude Code 2026 honest comparison
- GitHub: `closedloop-ai/claude-plugins`, `affaan-m/everything-claude-code`, `miles990/self-evolving-agent`, `ruvnet/ruflo`
- ICLR 2026 Workshop on AI with Recursive Self-Improvement
- Glean Best Practices for AI Agent Security 2025
- Obsidian Security: Prompt Injection — The Most Common AI Exploit 2025

### Research Queries Executed (for TTL caching)
1. `AI agent orchestration frameworks 2025 2026 trends LangGraph AutoGen CrewAI` — expires 2026-03-20
2. `Claude Code plugin development best practices 2025 2026` — expires 2026-03-20
3. `multi-agent pipeline patterns eval-driven development AI 2025` — expires 2026-03-20
4. `Devin Cursor agent mode Windsurf aider AI coding assistant comparison 2025 2026` — expires 2026-03-27
5. `AI agent security best practices prompt injection jailbreak 2025 2026` — expires 2026-03-20
6. `Claude Code evolve-loop self-improving agent plugin GitHub 2025 2026` — expires 2026-03-20
7. `AI agent self-improvement meta-learning recursive optimization 2025 2026` — expires 2026-03-27
8. `Claude Code subagents hooks system eval harness 2025 2026 best practices production` — expires 2026-03-20
