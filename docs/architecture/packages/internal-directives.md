# internal/directives

## Purpose

`internal/directives` loads two operator-authored markdown files, a global layer for all loops and a per-loop layer keyed by the runscope lane. A loop re-reads them at the start of each cycle and injects the rendered block into every phase agent's prompt. All loops thus converge on an edit at the next cycle boundary, without a restart. Spec: [the directives cascade design](../../superpowers/specs/2026-06-20-runtime-directives-cascade-design.md).

## Design

- **Pure and environment-agnostic.** `Load` takes explicit paths and never reads `os.Getenv`, `LookupEnv` or `Environ`. The composition root resolves the home directory and the lane through `Resolve` and passes the paths in. The package thus stays unit-testable without `t.Setenv` (the flag-to-parameter conversion standard).
- **Fail open.** A missing, unreadable or empty file gives an absent `Layer` (`Body == ""`). A directive can never block a cycle.
- **Precedence by labelled concatenation.** `render` writes the global layer first, then the per-loop layer, under a safety preamble. When both layers are absent the block is empty.

## Invariants

- **Directives are guidance, never a gate.** The preamble states that global directives are authoritative for safety and that guidance never overrides the binary gates. The gates stay the only hard trust boundary. Pinned by the preamble test in `directives_test.go`.
- **A lane cannot forge structure.** `sanitizeLane` strips line breaks from the caller-supplied lane before it enters the markdown heading, so a lane cannot add lines or a fake heading. A literal `## Injected` in mid-line is harmless in markdown and survives.
- **The version follows content.** The version is the sha256 of the merged block. A global-only block is lane-independent, so the lane does not change the version. With a per-loop layer the lane is rendered and does change it.
