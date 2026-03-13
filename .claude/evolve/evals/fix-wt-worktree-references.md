# Eval: fix-wt-worktree-references

## Grader Type: grep + manual

## Checks

### 1. No `wt` CLI references in agent files
```bash
grep -rn "^wt \|run \`wt " agents/
```
**Expected:** Zero matches. No agent file should reference `wt` as a CLI command.
**Verdict:** PASS if zero matches, FAIL if any match found.

### 2. Developer agent references EnterWorktree
```bash
grep -l "EnterWorktree" agents/evolve-developer.md
```
**Expected:** Match found — the developer agent instructs use of the `EnterWorktree` tool.
**Verdict:** PASS if match found, FAIL if not.

### 3. Deployer agent references ExitWorktree or standard git merge
```bash
grep -l "ExitWorktree\|git merge\|git rebase" agents/evolve-deployer.md
```
**Expected:** Match found — the deployer agent uses `ExitWorktree` and/or standard git commands for merge workflow.
**Verdict:** PASS if match found, FAIL if not.

### 4. No broken workflow references
```bash
grep -rn "wt merge\|wt switch" agents/ skills/ docs/
```
**Expected:** Zero matches outside workspace/ and briefing files.
**Verdict:** PASS if zero matches, FAIL if any match found.

## Overall Verdict
PASS if all 4 checks pass. FAIL if any check fails.
