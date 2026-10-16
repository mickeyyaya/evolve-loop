# internal/bedrock

## Purpose

`internal/bedrock` ports the bash script `legacy/scripts/dispatch/build-invocation-context.sh`. It emits the static trust-boundary prefix of a subagent invocation: a common header, plus the operating notes of the role when it has some.

## Design

- **Byte-identical output.** The output for a role is the same across every invocation. There are no timestamps, no environment data and no random salts. This matters for Anthropic prompt-cache reuse, which needs a prefix of at least 1024 tokens with a 5-minute TTL per model. Any drift silently invalidates the cache and inflates cost.
- **Common bedrock, then role extension.** `commonBedrock` applies to every role. `roleExtensions` holds the suffix for the four roles that have operating notes: auditor, builder, scout and retrospective.
- **Unknown roles fall through.** A role with no extension emits only the common bedrock, matching the bash `case` statement's silent default.
- **Empty role is an error.** `Emit` returns `ErrMissingRole` for an empty or whitespace-only role, matching the bash usage error.
- **The common header is a contract.** It must stay byte-identical to the `cat <<EOF` block of the bash script. The challenge-token line is what ship-gate checks reports against.

## Invariants

- **Parity with bash.** `TestEmit_ParityWithBash` compares the output with the bash script for every known role. It skips when bash or the script is absent.
- **The common header is always present.** `TestEmit_CommonBedrockAlwaysPresent` pins this for every role.
- **Only the matching extension is added.** `TestEmit_UnknownRoleOnlyEmitsCommon` pins that an unknown role carries none of the four markers.
- **Deterministic.** `TestEmit_Deterministic` pins identical output for identical input.

## Findings

- The adversarial-audit section of the auditor's extension is removed when `ADVERSARIAL_AUDIT=0`. The orchestrator strips it, not this package.
