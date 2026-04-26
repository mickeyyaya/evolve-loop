#!/usr/bin/env bash
#
# gemini.sh — STUB adapter for Google Gemini CLI.
#
# Not yet implemented. Exits with code 99 so the runner fails loudly rather than
# silently degrading to a less-isolated invocation. Implement by mapping evolve-
# loop profile fields to gemini's CLI equivalents (model, allowed/blocked tools,
# working directory, budget, etc.) once the gemini CLI's flag surface stabilises.
#
# Required env (when implemented): same as claude.sh — PROFILE_PATH,
# RESOLVED_MODEL, PROMPT_FILE, CYCLE, WORKSPACE_PATH, STDOUT_LOG, STDERR_LOG,
# ARTIFACT_PATH, optional WORKTREE_PATH.

set -euo pipefail

cat >&2 <<'EOF'
gemini.sh: ERROR (exit 99): Gemini CLI adapter not yet implemented.

The evolve-loop currently supports the 'claude' provider only. To use a
different provider, implement scripts/cli_adapters/gemini.sh against the
gemini CLI's flag surface and update the profile's "cli" field.

Implementation guidance:
  - Translate .allowed_tools and .disallowed_tools to gemini's permission flags
  - Translate .add_dir to gemini's working-directory restriction
  - Translate .max_budget_usd to gemini's cost cap
  - Stream stdout to STDOUT_LOG and stderr to STDERR_LOG
  - Exit with the underlying CLI's exit code

Until then, set the profile's "cli" field to "claude".
EOF
exit 99
