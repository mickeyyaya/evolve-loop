# Online Researcher Patterns

> Reference doc for WebSearch/WebFetch query construction and result synthesis in evolve-loop agents.
> This file has no YAML frontmatter (`name:`/`tools:`) — it is a reference document, not an agent definition.
> Architecture context: [docs/architecture/research-tool.md](../architecture/research-tool.md)

## Table of Contents

1. [Good Query Patterns](#good-query-patterns)
2. [Bad Query Patterns](#bad-query-patterns)
3. [When to Synthesize](#when-to-synthesize)
4. [KB-First Reminder](#kb-first-reminder)

---

## Good Query Patterns

Use these patterns when KB hits are sparse (< 3) or evidently outdated.

| Pattern | Example | Why it works |
|---------|---------|--------------|
| **Versioned + specific** | `"claude code hooks PreToolUse JSON schema 2025"` | Pins to a release window; avoids ancient tutorials |
| **Error message verbatim** | `"SIGPIPE error bash pipefail grep -q large string"` | Exact error text surfaces the right SO/GH issues |
| **Repo + feature** | `"anthropic claude-code allowedTools override settings.json"` | Scopes to the right product; avoids generic results |
| **Changelog query** | `"ripgrep 14 breaking changes CHANGELOG"` | Targets authoritative source rather than blog posts |
| **"site:" scoped** | `"site:github.com anthropic/claude-code hooks"` | Forces GitHub issues/docs when community noise is high |
| **Negative filter** | `"bash associative array bash 3 NOT bash 4 NOT declare -A"` | Excludes known-incompatible solutions up front |

---

## Bad Query Patterns

| Anti-pattern | Example | Why it fails |
|--------------|---------|--------------|
| **Vague noun phrase** | `"bash scripting best practices"` | Returns 10 000 generic tutorials; no actionable signal |
| **Year-free version query** | `"Claude API streaming"` | Returns results from every API version; often wrong version |
| **Multi-topic query** | `"how to install ripgrep AND configure hooks AND write predicates"` | Search engine picks one thread; you miss the rest |
| **Implicit assumption** | `"why does ANTHROPIC_API_KEY not work"` | Assumes API key is the auth path; misses subscription-auth docs |
| **Ambiguous acronym** | `"ACS errors"` | Could mean dozens of things; always expand acronyms |

---

## When to Synthesize

Apply these rules when merging multiple WebSearch results with KB hits.

1. **KB result wins on internal conventions.** If KB and web conflict on repo-specific conventions (hook contract shape, profile schema), trust the KB result — it reflects the current codebase state.

2. **Web result wins on external versioning.** If KB and web conflict on an external tool's behavior (ripgrep flags, Claude API parameters), trust the web result — KB may lag.

3. **Cite the winning source.** When the two conflict, note which source you used and why, so a future Auditor or Retrospective agent can validate the choice.

4. **Do not average contradictory sources.** If KB says `rc=2` means deny and a web result says `rc=2` means error, pick one and cite it — do not write "rc=2 may mean deny or error."

5. **Merge additive facts, not conclusions.** If web provides 3 examples and KB provides 2, combine all 5 examples. If web provides a conclusion and KB provides a different conclusion, pick one.

6. **Record the synthesis in your artifact.** The scout-report or build-report should include a "Research Executed" section that lists queries, sources, and which results were used. This enables retrospective validation.

---

## KB-First Reminder

Before issuing any WebSearch or WebFetch call, run:

```bash
bash legacy/scripts/research/kb-search.sh "<your query pattern>"
```

Escalate only when KB hits < 3 or evidently outdated. Full directive and quota table: [docs/architecture/research-tool.md#kb-first-directive](../architecture/research-tool.md#kb-first-directive).
