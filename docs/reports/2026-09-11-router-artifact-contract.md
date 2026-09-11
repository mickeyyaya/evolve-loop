# Router multi-artifact contract recovery

**Date:** 2026-09-11

**Scope:** `PhaseAdvisor`, the bridge request boundary, deliverable contracts, and the router profile

## Finding

The router is one model/profile/telemetry identity with three output protocols:

| operation | artifact | top-level JSON shape | completion |
|---|---|---|---|
| Plan | `routing-plan.json` | array | artifact |
| RePlan | `routing-replan.json` | array | artifact |
| Propose | `routing-proposal.json` | object | stdout |

The implementation previously resolved a deliverable contract only from
`BridgeRequest.Agent`. All three operations use `Agent="router"`, so every dispatch received the
`router` contract even when `ArtifactPath` pointed to the replan or proposal file. The injected
path and JSON wording could describe one artifact while the injected self-check validated another.

Cycle 1623 preserves the failure in
`.evolve/runs/cycle-1623/tmux-final-scrollback.txt`. The proposal prompt names
`routing-proposal.json` at line 237, embeds `phase="router"` at line 239, and tells the model to run
`evolve phase verify router` at line 242. The model then invoked that wrong verifier eight times
(lines 253, 258, 268, 275, 291, 297, 309, and 326), edited both proposal and plan files, and moved
the plan aside while trying to understand the contradiction. This was useful work displaced by a
host-generated instruction conflict. It increased model turns, tokens, file I/O, and wall time
without improving the routing decision.

The contract also described every keyless JSON artifact as an object. Plan and RePlan consumers
parse arrays. The verifier accepted any valid JSON value when a contract had no required keys, so
an object, scalar, or `null` could pass a plan contract despite being unusable by the consumer.
Finally, the router profile allowed writes to plan and proposal files but omitted the replan file,
so a correct replan prompt could still be denied by the Claude tool policy.

## Design

The recovery separates identity from protocol at the existing bridge boundary:

- `BridgeRequest.Agent` remains `router`. It continues to select the profile, CLI/model policy,
  skill overlays, interactive policy, and telemetry attribution.
- The additive `BridgeRequest.Contract` selects the deliverable protocol. Empty defaults to
  `Agent`, preserving every existing caller. An explicitly named unknown contract fails before
  dispatch with its identifier in the error.
- `PhaseAdvisor` chooses the contract next to the artifact and completion mode. Plan uses `router`,
  RePlan uses `router-replan`, and Propose uses `router-proposal`. Filename inference is avoided.
- The contract registry contains three records with the same `AgentName="router"` and distinct
  artifact names.
- `Contract.JSONShape` is the shared top-level shape rule used by prompt rendering and the
  verifier. Its zero value means any valid JSON value for backward compatibility. Required keys
  still imply an object for legacy keyed contracts.
- The router profile projects Write/Edit permissions for all three registered artifacts.

This keeps one operational identity while giving each producer/consumer protocol an exact,
independently verifiable contract. Proposal completion remains stdout-based; changing that
transport is outside this repair and would mix a separate reliability decision into the batch.

## TDD evidence

The implementation was driven in isolated red/green steps:

1. Registry tests failed because `router-replan` and `router-proposal` were absent.
2. Renderer tests failed because plan and replan prompts said “JSON object.”
3. Verifier tests failed because plan objects/`null`, replan scalars, and proposal arrays/`null`
   passed. A sibling-artifact test proved a valid plan cannot satisfy a missing proposal.
4. The bridge RED test initially decoded the future `contract` wire field before it existed, then
   was simplified after GREEN to construct the typed request directly. It failed because a proposal
   received the plan contract and an unknown explicit contract silently dispatched.
5. PhaseAdvisor tests failed because all three requests left `Contract` empty while their artifact
   and completion fields differed.
6. The real router-profile test failed specifically on missing Write/Edit permissions for
   `routing-replan.json`.

The green suite covers the registry, prompt prefix and tail, shared verifier, bridge injection,
advisor Plan/RePlan/Propose wiring, CLI self-checks, and the tracked router profile. Package-level,
race, vet, repository-wide, and commit-gate results are recorded in the PR after final review.

## Operational result

Each router prompt now gives one coherent instruction triple: exact artifact path, exact JSON
shape, and a self-check that verifies that same artifact. The change removes the observed source
of repeated verification/edit loops without adding a model call, retry, parser dependency, or
runtime scan. Shape checking is linear in the artifact bytes already read by the verifier. Existing
latency and token telemetry remains attributed to `router`, allowing pre/post comparisons without
splitting one logical model role into artificial identities.

## Review boundaries

Review should reject the change if any of these invariants fail:

- Agent identity changes away from `router` for any of the three operations.
- A contract is inferred from `ArtifactPath` instead of selected explicitly.
- An explicit unknown contract degrades to the agent contract.
- Plan/RePlan accept a non-array or Proposal accepts a non-object.
- `null` satisfies an object or array contract.
- Proposal completion changes from stdout as a side effect.
- The profile omits Write or Edit permission for any registered router artifact.
