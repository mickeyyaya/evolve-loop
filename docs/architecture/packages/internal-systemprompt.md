# internal/systemprompt

## Purpose

`internal/systemprompt` resolves the system prompt and rules that an agent receives at launch (facet B). The agent profile is the only source. X18c removed the environment overrides ([ADR-0156](../adr/0156-the-cli-is-the-service-interface-and-the-pipeline-reads-no-ambient-control-flags.md) I-16).

## Design

The precedence chain, highest first:

1. the profile's `system_prompt`
2. the file named by the profile's `digest_file`
3. the file named by the profile's `system_prompt_file`
4. the empty string

- **Digest over rules file.** `digest_file` names a pre-generated, role-scoped digest (the `go/internal/digest` output). When it is set and present on disk, it wins over `system_prompt_file`. When it is unset, or the file is absent, the chain falls through to the next tier.
- **Inline over file.** An inline `system_prompt` wins over both files.
- **Relative paths** resolve against the profile directory. Trailing newlines are trimmed from file content.
- **Missing profile** is not an error. It contributes the empty default.

## Invariants

- **No environment overrides the profile.** `EVOLVE_SYSTEM_PROMPT` and `EVOLVE_<AGENT>_SYSTEM_PROMPT` have no reader. `TestResolve_ProcessEnvDoesNotOverrideProfile` and `TestResolve_TheRequestEnvironmentNeverOverridesTheProfile` pin this.
- **A profile that never sets `digest_file` keeps the old chain.** `TestResolve_DigestFileUnsetLeavesChainUnchanged` pins this.
- **An unreadable file falls through, not to empty.** `readRelativeFile` reports `ok=false` so that the next tier is tried.

## Findings

- None.
