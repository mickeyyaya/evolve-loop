# Cycle 1 Scan Report

## Summary
- Files scanned: 35 (11 agents, 4 skills, 3 docs, 2 shell scripts, 2 JSON configs, 1 examples, 5 root docs, 1 gitignore, 1 license, 2 evolve state files)
- Issues found: 15 (0 critical, 4 high, 7 medium, 4 low)
- Dependency vulnerabilities: N/A (no package manager)
- Hotspots identified: 3

---

## Tech Debt

| File | Issue | Severity | Details |
|------|-------|----------|---------|
| `docs/writing-agents.md` | Outdated guidance | HIGH | "Creating an ECC Wrapper" section (line 62) instructs contributors to "Copy the full content of the ECC agent file" — but v3.1 switched to thin context overlays. This contradicts the current architecture and will cause contributors to create bloated agents. |
| `CONTRIBUTING.md` | Repo URL mismatch | HIGH | Line 8 uses `github.com/danleemh/evolve-loop` while README (line 52), plugin.json, and marketplace.json all use `github.com/mickeyyaya/evolve-loop`. Contributors following CONTRIBUTING.md will clone the wrong repo. |
| `CHANGELOG.md` | Missing 3.1.0 entry | HIGH | plugin.json and marketplace.json declare version 3.1.0, but CHANGELOG.md only documents 3.0.0 and 2.0.0. The plugin support changes (commit `613abf1`) are undocumented in the changelog. |
| `README.md` | Phase count inaccuracy | HIGH | Claims "8-phase pipeline" (lines 3, 10) but enumerates 10 distinct phases: 0, 1, 2, 3, 4, 4.5, 5, 5.5, 6, 7. The pipeline bullet point even lists MONITOR→DISCOVER→PLAN→DESIGN→BUILD→CHECKPOINT→VERIFY→EVAL→SHIP→LEARN (10 phases). |

---

## Documentation Inconsistencies

| File | Issue | Severity | Details |
|------|-------|----------|---------|
| `README.md` | Agent count contradiction | MEDIUM | Line 3 and line 9 claim "13 specialized agents" but: (a) there are 11 agent files in `agents/`, (b) line 187 correctly says "11 agent definition files", (c) the Eval Runner is not an agent. The "13" count appears to include the Eval Runner (orchestrator-executed) and possibly double-counts. |
| `README.md` | Operator misclassified | MEDIUM | Line 188 labels `evolve-operator.md` as "ECC wrapper" but it is a full custom agent (has model frontmatter, `subagent_type: general-purpose`). The ECC content was copied in, not delegated via `subagent_type`. architecture.md (line 55) correctly notes "the Operator uses `general-purpose` since no ECC subagent_type exists." |
| `agents/evolve-operator.md` | Hybrid pattern confusion | MEDIUM | Contains an `## ECC Source` section (lines 32-35) claiming it was "Copied from: `everything-claude-code/agents/loop-operator.md`" — yet it is launched via `subagent_type: general-purpose`, not an ECC subagent_type. This makes it unclear whether ECC is actually required for the Operator to function, or whether the content is stale. |
| `skills/evolve-loop/SKILL.md` | Agent count in description | MEDIUM | Front matter description says "13 role-specialized agents" — same inaccuracy as README. Should be 11 agent files (plus Eval Runner as orchestrator logic). |
| `.claude/evolve/ledger.jsonl` | Field naming divergence | MEDIUM | The existing ledger entry uses `"timestamp"` but memory-protocol.md specifies `"ts"` as the field name. Future cycles will produce inconsistent ledger entries, breaking any tooling that parses the ledger. |
| `agents/evolve-operator.md` | Model frontmatter but overlay-style body | MEDIUM | Has `---\nmodel: sonnet\n---` frontmatter (custom agent style) but the body contains an `## ECC Source` section (overlay style). The two patterns are mixed, creating ambiguity for contributors about which pattern the Operator follows. |
| `install.sh` | Non-interactive CI incompatibility | MEDIUM | Line 37 uses `read -p "Continue anyway? [y/N] "` with no CI/non-interactive fallback. No `FORCE`, `--yes`, or `NONINTERACTIVE` environment variable check. Running `./install.sh` in a CI pipeline will hang indefinitely if ECC agents are absent. |

---

## Dead Code / Unused References

| Location | Issue | Severity |
|----------|-------|----------|
| `docs/writing-agents.md` lines 58-69 | "Creating an ECC Wrapper" section describes a workflow (copy full ECC content) that was explicitly deprecated in commit `ecbd5b2` ("refactor: replace ECC content copies with thin context overlays"). This section is dead/misleading guidance. | LOW |
| `.gitignore` | Does not exclude `.claude/evolve/workspace/`, `.claude/evolve/history/`, `.claude/evolve/instincts/`, or `.claude/evolve/evals/`. These are runtime artifacts that likely should not be committed to end-user projects. However, the evolve-loop repo itself uses `.claude/evolve/` for bootstrap state, so this may be intentional for the repo itself. | LOW |
| `~/.claude/homunculus/instincts/personal/` references | Three files reference this path for instinct promotion: `phases.md:206`, `memory-protocol.md:131`, `docs/configuration.md:110`. This path does not appear to be a Claude Code standard directory — may be aspirational or unstable API. No validation that this path exists before writing. | LOW |

---

## Test Coverage Analysis

This project has no automated test infrastructure (no package.json, pytest, go.mod, etc.). The "test command" is `./install.sh`, which is an integration smoke test only.

| Area | Coverage | Critical Path? |
|------|----------|----------------|
| `install.sh` correctness | Manual/smoke only | YES — primary distribution mechanism |
| `uninstall.sh` correctness | Manual/smoke only | YES — users rely on clean uninstall |
| Agent file completeness | None | YES — missing fields cause runtime failures |
| Ledger entry format consistency | None | MEDIUM — parsing failures across cycles |
| Cross-reference integrity | None | LOW — dead links confuse contributors |

**Gap:** No automated validation that agent files have required sections (`## Inputs`, `## Output`, ledger entry format). A simple shell script linter could catch malformed agent files before publish.

---

## Hotspots (High Churn + High Complexity)

| File | Changes (all history) | Complexity | Recommendation |
|------|-----------------------|------------|----------------|
| `agents/evolve-e2e.md` | 3 commits | LOW (64 lines, overlay only) | Recently stabilized after ECC refactor + dead reference fix. Monitor if ECC e2e-runner API changes. |
| `agents/evolve-security.md` | 3 commits | LOW (70 lines, overlay only) | Same as e2e — was touched 3 times in initial commits for ECC integration cleanup. |
| `README.md` | 3 commits | MEDIUM (286 lines, dense) | Most frequently changed doc. High risk for accumulating staleness. Architecture diagram is duplicated verbatim in SKILL.md — changes must be synchronized manually. |

---

## Size Violations

No file exceeds 800 lines. All files are within acceptable bounds.

| File | Lines | Status |
|------|-------|--------|
| `skills/evolve-loop/phases.md` | 250 | OK |
| `README.md` | 286 | OK |
| `skills/evolve-loop/memory-protocol.md` | 167 | OK |
| `agents/evolve-researcher.md` | 157 | OK |
| Largest file overall | 286 lines | Well within 800-line limit |

No function-length violations (shell scripts have no functions >50 lines; `install.sh` is 87 lines, entirely procedural).

---

## Shell Script Analysis

### `install.sh` (87 lines)

**Strengths:**
- Uses `set -euo pipefail` — proper error handling
- All variable expansions are double-quoted
- `SCRIPT_DIR` uses portable `${BASH_SOURCE[0]}` pattern
- Checks for existing files before overwriting

**Issues:**
1. **[MEDIUM]** Line 37: `read -p "Continue anyway? [y/N] " -n 1 -r` — no CI/non-interactive fallback. Will hang in pipelines.
2. **[LOW]** Line 79: Agent count comment hardcoded as "(6 custom + 5 ECC context overlays)" — the dynamic `wc -l` count is accurate but the parenthetical description could become stale if agents are added/removed.

### `uninstall.sh` (37 lines)

**Strengths:**
- Uses `set -euo pipefail`
- Properly uses `rm -rf "$SKILLS_DIR"` (quoted)
- Clear user messaging about what is/isn't removed

**Issues:**
1. **[LOW]** No `--dry-run` option. Users cannot preview what will be deleted.
2. **[LOW]** Does not verify the files being removed are actually evolve-loop agents (just matches `evolve-*.md` glob). If a user has unrelated files matching that pattern in `~/.claude/agents/`, they would be deleted.

---

## Dependency Vulnerabilities

No package manager files present (no `package.json`, `go.mod`, `requirements.txt`, `Cargo.toml`). No dependency audit applicable.

---

## Additional Findings

### Architecture Diagram Duplication
The full ASCII architecture diagram appears verbatim in both `README.md` (lines 80-90) and `skills/evolve-loop/SKILL.md` (lines 36-46), and the data flow diagram appears in `README.md` (lines 94-126), `SKILL.md` (lines 36-46 context), and `docs/architecture.md` (line 29 partial). Any pipeline change requires updating 2-3 locations simultaneously — a maintenance burden and source of drift.

### Operator Agent Classification Ambiguity
`evolve-operator.md` sits in an unclear category: it has model frontmatter (custom agent style) but an ECC Source section (wrapper style). README line 188 calls it "ECC wrapper." SKILL.md line 97 lists it in the custom agents table. `docs/architecture.md` line 55 explicitly says it uses `general-purpose` since no ECC subagent_type exists. Three different documents give three different characterizations.

### Instinct Promotion Path Risk
The `~/.claude/homunculus/instincts/personal/` path referenced in phases.md, memory-protocol.md, and configuration.md appears to be a non-standard path. If this directory doesn't exist or the path changes in future Claude Code versions, instinct promotion will silently fail. No existence check or error handling is specified.
