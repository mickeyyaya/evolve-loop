# Project Instructions (Generic AI Agents)

All project instructions are in [CLAUDE.md](CLAUDE.md). The content is platform-agnostic despite the filename — it covers autonomous execution rules, task priority, and pipeline integrity requirements.

Platform-specific tool mappings are in [docs/platform-compatibility.md](docs/platform-compatibility.md).

If you are activating the `evolve-loop` skill from a CLI without a tested adapter, read these in order before acting:

1. [skills/evolve-loop/reference/platform-detect.md](skills/evolve-loop/reference/platform-detect.md) — identify your platform (or set `EVOLVE_PLATFORM` to override)
2. [skills/evolve-loop/reference/generic-runtime.md](skills/evolve-loop/reference/generic-runtime.md) — what works and what doesn't on unsupported CLIs
3. [docs/platform-compatibility.md](docs/platform-compatibility.md) — adapter contract if you intend to add support
