# 2026-09-14 — every routing proposal was "unparseable": the model wrote the artifact its prompt asked for, the kernel read the scrollback

**Status:** fixed (this change) · **Severity:** P2 pipeline (no cycle failed — the proposer degrades to static routing — but one advisor launch per cycle was spent for nothing, and the Signal Center reported `ADVISOR_RESPONSE_UNPARSEABLE` on 9 of the verification wave's advisory calls and again on wave 2's first lanes; research finding F4) · **Found by:** the wave-2 Signal Center feed, then the pane transcript of cycle 1677's proposal launch.

## What happened

The advisor's post-build **proposal** decision (`decisionProposal`, contract `router-proposal`) still completed on **REPL-idle stdout** (ADR-0027): the bridge waited for the pane to go idle, captured the scrollback, and `ParseProposal` took its last balanced JSON object. The two plan decisions had long since moved to the uniform **artifact** contract (the brain writes `routing-plan.json` / `routing-replan.json`, the bridge reads the file back).

The proposal's prompt, however, carries the same deliverable contract the plans do — appended by the bridge from the contract registry:

```
DELIVERABLE PATH: <workspace>/routing-proposal.json
<deliverable-contract phase="router-proposal">
  <format>a single valid JSON object — write nothing else to this file</format>
```

So the model did what it was told: cycle 1677's Gemini pane shows `Thought for 10s … ● Create(…/routing-proposal.json)`, and cycles 1673 and 1674 left valid `routing-proposal.json` files on disk. The scrollback contained no answer, only the prompt's echoed example (`{"next_phase":"<phase>","insert_phases":["<phase>",...],…}`), which is what `ParseProposal` then parsed — hence `invalid character '.'` on every launch. The submit-verify ledger's `not_verified` rows on the same launches are a **separate** defect on the same surface as the codex prompt-submit wedge (submit-line verification at paste time — `interaction.ResultNotVerified` is "could not check", not "no reply"); this change does not touch them and they stay open.

## Fix

| Layer | Change | Test (red first) |
|---|---|---|
| `core/advisor` | `decisionProposal` completes on the **artifact** contract like the plans: the bridge waits for `routing-proposal.json` and hands its content to `ParseProposal` (which keeps its last-balanced-object tolerance for a fenced or prose-wrapped answer) | `TestDecision_TableProjectsTheContractRegistry`, `TestLaunch_ThreadsWorktreeArtifactContractCompletionAgentCycleEnv`, `TestPhaseAdvisor_…` (the three completion pins moved from `stdout` to `artifact`) |

## Why the class, not the symptom

The prompt and the kernel stated two different completion contracts for one decision. The fix removes the second belief rather than teaching the parser to read a file when the scrollback is empty: one contract, stated once in the registry, rendered into the prompt and read back by the bridge. The legacy stdout contract now has no producer in the advisor.

## What the operator will see

`ADVISOR_RESPONSE_UNPARSEABLE` stops firing on proposals; a proposal launch ends when `routing-proposal.json` lands, and the router's proposal shapes the next phase instead of static routing. A model that prints its answer instead of writing the file now times out honestly as `BRIDGE_EXIT_ARTIFACT_TIMEOUT` — an alert keyed to the old code goes quiet on a different failure than the one fixed. Follow-up (architecture review): the completion mode is still restated in `decisionRows` beside the contract registry that implies it (`WriteTarget`); deriving it from the registry is the structural close.
