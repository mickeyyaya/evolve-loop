# Initialization (once per session)

1. **Create directories and run ID:**
   ```bash
   mkdir -p .evolve/workspace .evolve/history .evolve/evals .evolve/instincts/personal .evolve/instincts/archived .evolve/genes .evolve/tools .evolve/runs
   RUN_ID="run-$(date +%s%3N)-$(openssl rand -hex 2)"
   mkdir -p ".evolve/runs/$RUN_ID/workspace"
   WORKSPACE_PATH=".evolve/runs/$RUN_ID/workspace"
   find .evolve/runs/ -maxdepth 1 -type d -name 'run-*' -mtime +2 -exec rm -rf {} \; 2>/dev/null
   ```

2. **Read or initialize state.json** — see [memory-protocol.md](../memory-protocol.md) for full schema.

3. **Detect domain:** Check `.evolve/domain.json` first, then auto-detect:
   - `package.json`/`go.mod`/`Cargo.toml` → coding
   - Mostly `.md`/`.txt` → writing
   - Citation patterns → research
   - Default: coding

   | Domain | evalMode | shipMechanism | buildIsolation |
   |--------|----------|---------------|----------------|
   | coding | bash | git | worktree |
   | writing | rubric | file-save | file-copy |
   | research | hybrid | file-save | file-copy |
   | design | rubric | export | file-copy |

4. **Pre-flight:** `git status --porcelain` must be clean.

5. **Shared values:** Include the `sharedValues` block from SKILL.md in every agent context. See [memory-protocol.md § Layer 0](../memory-protocol.md).
