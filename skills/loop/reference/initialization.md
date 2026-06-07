---
name: reference
description: Reference doc.
---

# Session Initialization

> Read this file once at the start of each `/evolve-loop` invocation.

## 1. Create Directories and Run ID

```bash
mkdir -p .evolve/workspace .evolve/history .evolve/evals .evolve/instincts/personal .evolve/instincts/archived .evolve/genes .evolve/tools .evolve/runs
RUN_ID="run-$(date +%s%3N)-$(openssl rand -hex 2)"
mkdir -p ".evolve/runs/$RUN_ID/workspace"
WORKSPACE_PATH=".evolve/runs/$RUN_ID/workspace"
find .evolve/runs/ -maxdepth 1 -type d -name 'run-*' -mtime +2 -exec rm -rf {} \; 2>/dev/null
```

## 2. Read or Initialize State

- If `.evolve/state.json` exists → read it, verify `version` field exists
- If missing → initialize with default schema (see [memory-protocol.md](../memory-protocol.md))
- Set `remainingCycles` from parsed arguments

## 3. Detect Domain

Check `.evolve/domain.json` first, then auto-detect from project files:

| Signal | Domain | evalMode | shipMechanism | buildIsolation |
|--------|--------|----------|---------------|----------------|
| `package.json`, `go.mod`, `Cargo.toml` | coding | bash | git | worktree |
| Mostly `.md`/`.txt` (>60%) | writing | rubric | file-save | file-copy |
| Citation patterns, `references/` dir | research | hybrid | file-save | file-copy |
| `.figma`/`.sketch`/`.svg` majority | design | rubric | export | file-copy |
| Default | coding | bash | git | worktree |

## 4. Pre-Flight Check

```bash
git status --porcelain   # must be clean
git worktree list        # verify worktree support
```

## 5. Shared Values

Include the `sharedValues` block in every agent context (see [memory-protocol.md § Layer 0](../memory-protocol.md)):
- Anti-bias protocols (verbosity, self-preference, blind-trust)
- Required protocols (challenge token, ledger entry, mailbox check)
- Quality thresholds (800 lines, 50-line functions, MEDIUM+ blocking)
- Integrity rules (protected files, no eval weakening, no gaming)
