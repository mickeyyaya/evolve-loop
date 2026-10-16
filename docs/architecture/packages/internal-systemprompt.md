# internal/systemprompt

## Purpose

`internal/systemprompt` resolves the system prompt and rules that an agent receives at launch (facet B). It mirrors the `resolvePolicy` precedence chain, so the per-agent and global environment overrides behave the same as in the interactive policy.

## Design

The precedence chain, highest first:

1. `EVOLVE_<AGENT>_SYSTEM_PROMPT`
2. `EVOLVE_SYSTEM_PROMPT`, read from the request environment only
3. the profile's `system_prompt`
4. the file named by the profile's `digest_file`
5. the file named by the profile's `system_prompt_file`
6. the empty string

- **Digest over rules file.** `digest_file` names a pre-generated, role-scoped digest (the `go/internal/digest` output). When it is set and present on disk, it wins over `system_prompt_file`. When it is unset, or the file is absent, the chain falls through to the next tier.
- **Inline over file.** An inline `system_prompt` wins over both files.
- **Relative paths** resolve against the profile directory. Trailing newlines are trimmed from file content.
- **Missing profile** is not an error. It contributes the empty default.

## Invariants

- **The process environment never overrides the profile.** The chain reads the request environment with `envchain.ResolveNoOS`, so an `EVOLVE_SYSTEM_PROMPT` set in the process does not win over a profile's `system_prompt`. `TestResolve_ProcessEnvDoesNotOverrideProfile` pins this.
- **A profile that never sets `digest_file` keeps the old chain.** `TestResolve_DigestFileUnsetLeavesChainUnchanged` pins this.
- **An unreadable file falls through, not to empty.** `readRelativeFile` reports `ok=false` so that the next tier is tried.

## Findings

- None.
