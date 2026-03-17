# Cycle 1 Backlog

## Selected Tasks (Priority Order)

### Task 1: Fix `wt` CLI references with Claude Code worktree tools

**Slug:** `fix-wt-worktree-references`
**Priority:** CRITICAL
**Estimated Scope:** Small (2 files, ~10 lines changed)

**Problem:**
The Developer agent (`evolve-developer.md` line 33) references `wt switch --create feature/<name>` and the Deployer agent (`evolve-deployer.md` line 43) references `wt merge`. These reference a non-existent `wt` CLI tool. Claude Code provides built-in worktree management via the `EnterWorktree` and `ExitWorktree` tools — the agent instructions should reference these instead.

Without this fix, BUILD (Phase 4) and SHIP (Phase 6) will fail or produce confusing behavior every cycle.

**Acceptance Criteria:**
1. `evolve-developer.md` replaces `wt switch --create feature/<name>` with instructions to use the `EnterWorktree` tool (Claude Code built-in)
2. `evolve-deployer.md` replaces `wt merge` block with instructions to use `ExitWorktree` tool and standard `git merge` commands
3. No other files reference `wt` as a CLI command (grep confirms zero hits outside workspace/briefing)
4. The merge workflow in the Deployer still describes squash-rebase-merge behavior using standard git commands

**Why this task:**
- Blocks the entire pipeline — every cycle that reaches BUILD or SHIP will encounter undefined `wt` commands
- Small, well-scoped change with no architectural risk
- Unblocks future cycles from functioning end-to-end

---

### Task 2: Fix documentation inconsistencies (writing-agents.md + ledger schema)

**Slug:** `fix-docs-and-ledger-consistency`
**Priority:** HIGH
**Estimated Scope:** Small-Medium (3-4 files, ~30 lines changed)

**Problem (A — writing-agents.md):**
`docs/writing-agents.md` lines 58-69 ("Creating an ECC Wrapper" section) instructs contributors to "Copy the full content of the ECC agent file." This contradicts the v3.1 context-overlay architecture where ECC wrappers are thin overlays launched via `subagent_type`. Contributors following this guide will create bloated, duplicated agent files.

**Problem (B — ledger schema):**
The ledger has inconsistent field naming. The Operator's pre-flight entry (line 1) and research entry (line 5) use `"timestamp"` while the PM and Scanner entries (lines 2-3) use `"ts"`. The spec in `memory-protocol.md` defines `"ts"` as canonical. Inconsistency will break any tooling that parses the ledger.

**Acceptance Criteria:**
1. `docs/writing-agents.md` "Creating an ECC Wrapper" section is rewritten to describe the context-overlay pattern (thin overlay + `subagent_type` delegation), not the copy-full-content pattern
2. `memory-protocol.md` canonical field name `"ts"` is confirmed as the standard
3. `agents/evolve-operator.md` ledger entry format uses `"ts"` (not `"timestamp"`)
4. Existing ledger entries in `.claude/evolve/ledger.jsonl` are normalized to use `"ts"` consistently
5. No file in the repo specifies `"timestamp"` as a ledger field name (grep confirms)

**Why this task:**
- writing-agents.md is the contributor onboarding doc — wrong guidance compounds across all future contributions
- Ledger inconsistency is a data integrity issue that worsens every cycle
- Both are small, well-scoped fixes with clear before/after states

---

## Deferred Tasks (Future Cycles)

| Task | Priority | Reason for Deferral |
|------|----------|-------------------|
| Fix README phase count ("8" vs 10 actual) | HIGH | Lower impact than pipeline-blocking issues |
| Fix CONTRIBUTING.md repo URL | HIGH | Important but doesn't block pipeline execution |
| Add CHANGELOG 3.1.0 entry | HIGH | Important but doesn't block pipeline execution |
| Fix agent count discrepancy (13 vs 11) | MEDIUM | Cosmetic, no runtime impact |
| Add CI/CD workflow | MEDIUM | Requires design decisions, too large for cycle 1 |
| Document instinct system in README | MEDIUM | User-facing but not blocking |
| Add install.sh CI/non-interactive mode | MEDIUM | Blocks CI but CI doesn't exist yet |
| Eval baseline infrastructure | MEDIUM | Strategic, better after pipeline is unblocked |
| Denial-of-wallet guardrails | MEDIUM | Security hardening, not blocking |
