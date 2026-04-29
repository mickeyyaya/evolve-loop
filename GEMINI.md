# Project Instructions (Gemini CLI)

All project instructions are in [CLAUDE.md](CLAUDE.md). The content is platform-agnostic despite the filename — it covers autonomous execution rules, task priority, and pipeline integrity requirements.

Platform-specific tool mappings are in [docs/platform-compatibility.md](docs/platform-compatibility.md).

If you are activating the `evolve-loop` skill from Gemini CLI, read these in order before acting:

1. [skills/evolve-loop/reference/platform-detect.md](skills/evolve-loop/reference/platform-detect.md) — confirm platform = gemini
2. [skills/evolve-loop/reference/gemini-tools.md](skills/evolve-loop/reference/gemini-tools.md) — translate Claude Code tool names to Gemini equivalents
3. [skills/evolve-loop/reference/gemini-runtime.md](skills/evolve-loop/reference/gemini-runtime.md) — invocation pattern; **note**: the hybrid driver requires the `claude` binary on PATH
