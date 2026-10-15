# internal/phases/scout

## Purpose

`internal/phases/scout` is the discovery and planning phase. It runs the scout agent, which turns the backlog and the operator's goal into the selected tasks for the cycle. The phase boilerplate lives in `internal/phases/runner`. This package holds only the scout-specific variation points: the prompt, the verdict mapping and the map-reduce helpers for the scan.

## Design

- **Verdict mapping.** The artifact decides the verdict. Convergence with no task section maps to SKIPPED, and it is the only path to SKIPPED. A non-empty `Proposed Tasks` or `Selected Tasks` section maps to PASS. A missing or empty artifact, or an empty task section, maps to FAIL. An empty backlog is a real failure, because triage will have nothing to work on. 
- **Typed envelope, then map.** At enforce, the strategy, the goal and the challenge token are read from the typed input. Below enforce, they are read from the legacy context map. The two sources render the same prompt, so the migration changes no bytes.
- **Challenge token.** The orchestrator mints the token. Scout uses the token it is given and never invents one. When the token is empty, the prompt omits the line, so the agent reads the token file or fails loudly.
- **Goal as constraint.** An operator goal reaches scout as a constraint. Backlog selection must treat it as canonical.
- **Lane scope.** When a fleet lane supplies a scope, the prompt names only that lane's todo ids. It tells the agent to read each live record by path. It forbids a search by name, because a name search can hit a consumed namesake. It also asks the Decision Trace to echo the pinned goal hash, so the scout-to-triage coherence gate can fire. Without a scope, the prompt has no lane line, so sequential cycles stay byte-identical.
- **Recent outcomes.** The recent-outcomes digest is appended after the stable prefix lines, which keeps the prompt cache warm. An absent digest keeps the prompt byte-identical.
- **Partition.** The map step splits the sorted package list into at most `n` contiguous slices. Related packages stay together, so cross-package findings stay within one worker. The core hub is excluded from the map step, because the reduce step scans it alone, so each hub edit has one writer.
- **Digest merge.** Candidate tasks dedupe by a fingerprint of the slug and the sorted files. Each keeps the highest confidence and counts its occurrences. A pattern that two or more slices flag becomes a confirmed cross-cutting item.
- **Digest vocabularies.** A finding has kind gap, hotspot or debt. Its severity is LOW, MEDIUM or HIGH. A candidate task has complexity S, M or L. The merge does not check these values, so the scout agent must emit them.
- **Config only.** The compact-prompt flag and the deliverable kind come from policy and the domain file. No phase sets them to a literal.

## Invariants

- Only convergence maps to SKIPPED, and an empty backlog otherwise fails. Pinned by the classify tests in `scout_test.go`.
- At enforce the typed envelope is read, even with no context map. Pinned by `scout_phaseio_test.go`.
- The prompt renders the challenge token and the goal, and it omits the token line when the token is empty. Pinned by `TestComposePrompt_InjectsChallengeTokenFromRequest`, `TestComposePrompt_OmitsChallengeTokenLineWhenEmpty` and `TestComposePrompt_InjectsGoalTextFromContext`.
- A lane sees only its own todo ids, and a sequential cycle sees no lane line. Pinned by `lanescope_prompt_test.go`.
- An absent recent-outcomes key keeps the prompt byte-identical. Pinned by `recent_outcomes_prompt_test.go`.
- The partition keeps slices sorted and contiguous, and it compacts the empty trailing slots. Pinned by `partition_test.go`.
- A pattern in one slice is not cross-cutting. Pinned by `digest_test.go`.
