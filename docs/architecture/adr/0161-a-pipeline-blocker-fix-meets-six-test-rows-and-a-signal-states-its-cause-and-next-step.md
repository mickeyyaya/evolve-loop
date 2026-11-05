# ADR-0161: A pipeline blocker fix meets six test rows, and a signal states its cause and next step

- **Status:** Proposed (2026-11-05).
- **Plan:** [pipeline-blockers-2026-11](../../plans/pipeline-blockers-2026-11.md). **Research:** [pipeline-blockers-2026-11](../../research/pipeline-blockers-2026-11.md). **Design:** [signal-accuracy](../signal-accuracy.md). **Incident:** [2026-11-05 wave 155](../../incidents/2026-11-05-wave-155-false-signals-and-the-logout.md).
- **Relates to:**
  - [ADR-0162](0162-the-console-reads-through-a-tested-read-only-debug-family-and-writes-no-scratch-scripts.md): it owns the verb for one cycle, `evolve debug cycle N`.
  - [ADR-0159](0159-the-loop-keeps-its-integration-train-and-takes-six-practices-from-the-2026-harness-projects.md): this ADR adds a design check and a signal rule to the journey proof of that ADR. It changes none of its decisions.
  - [ADR-0126](0126-every-iterative-loop-converges-or-escalates.md): the redesign of decisions 9 and 10 is a rung of its one escalation path (ADR-0126 decisions 12 to 17). ADR-0126 owns the count.
  - [ADR-0156](0156-the-cli-is-the-service-interface-and-the-pipeline-reads-no-ambient-control-flags.md): the debug verbs are verbs of the CLI contract, with the agent class A.
  - [ADR-0101](0101-signal-center.md): the signal contract is a rule for the text of the signals that the Signal Center carries.
- **Evidence:**
  - Seven fixes got 38 findings (11 CRITICAL, 27 HIGH) from the architecture reviewer, and four fixes were blocked twice (research F1.14).
  - 12 of the 38 findings are a false or missing cause, or a next step that fails (research F1.15, F1.16).
  - A test name in a failure text gave the class `infra-error` to a gate block (research F1.10).
  - A logout near 01:00 was followed by a stop reason of `quota-pause` at 56 percent usage (research F1.3, F1.12). The code path is not traced.
  - One fact had two readers in B4, B5 and B8, and a fix reached one of several entries in B3 and B6 (research F1.17).

## Context

The operator gave four orders and asked one question on 2026-11-04 and 2026-11-05. This ADR records them in short form, and the plan quotes each one:

- Each blocking issue gets a complete test plan.
- The signal is accurate, with clear guidance.
- A task that is blocked "over 3 times" goes to a redesign.
- Design and architecture review come before the build.
- The operator asked: "Why do we have bash script?", and "Could we just build the debug cli in evolve loop for your use and cover with tests?"

Wave 155 showed the cost of the missing rules. Its status said "0 of 11" for a wave with two ships. A test name changed a failure class. A logout read as quota.

Seven fixes were built before a design review, and the review found the design faults after the build. The operating policy has the fix sequence and the convergence ladder. It has no design step, no signal rule and no content for the strategy change of a blocker.

## Decision

1. A pipeline blocker fix starts with a design card, and an architecture review reads it before any test.
2. A blocker fix meets six rows: red proof, journey, preservation, wiring, mutation and signal.
3. The wiring row has one test for each entry that reads the fact.
4. A signal states the operation, the item, the cause or "unknown", an evidence handle, the impact and a next step.
5. A signal never states a cause that the program does not know.
6. The writer of a failure sets its class from a host fact, never from words in text.
7. Each fact has one reader: one count, one parser and one rule for the tree.
8. The test of a signal reads the whole text with equality.
9. More than three blocks on a subject stops its fix rounds and starts an architecture redesign.
10. A redesign writes a new design card, and the same review reads it before the build.
11. A logout is its own cause, and an unknown cause never defaults to quota.
12. The console writes no scratch script. A step done by hand twice becomes a tested verb.
13. Each new debug verb is class A and read-only.

## Consequences

**Good**
- A design fault shows in a card review of minutes, not in a code review after hours of build (research F1.15).
- A signal gives an agent the cause, the evidence and a step, so the agent does not guess (research P2.1).
- A class cannot change because a test has a word in its name (research F1.10).
- A task with more than three blocks gets a different approach, and the operator sees the count.
- The audit of a wave uses tested verbs, so the result is repeatable.

**Bad**
- Each blocker fix pays for a card and a card review before its first test.
- The equality test of a message makes each text change touch a test.
- A redesign at the top tier costs more than a fourth patch. The cost is fixed by the rule and is not by choice.
- Two debug verbs need code, tests and upkeep.

**Risk**
- A card can state "does not apply" for a row with no fact. Control: a review seat reads each reason.
- The rule "unknown" can hide a quota wall. Control: a probe that shows the wall keeps the class `quota`.

**What changes this decision**
- We revisit decision 1 if, over ten fixes, the card review finds fewer design faults than the code review finds after the build.
- We revisit decision 9 if a redesign does not lower the blocks of the next ten escalated tasks.
- We revisit decision 12 if a debug verb is used less than twice in a month. Then we remove it.

## Rejected

| Option | The fact that rules it out |
|---|---|
| Patch until the review is green | Four fixes were blocked twice, and the findings were design faults, not slips (research F1.14, F1.15). |
| One redesign of the whole signal layer | It drops seven staged fixes, and the faults are in five different files (plan section 4). |
| Mechanical gates only, with no design step | A mutation gate on a whole module takes hours (ADR-0159, F8.8), and a gate cannot find a wrong design. |
| A runbook link for each signal code | The sources are weak, and `signal-codes.md` already states the meaning of each code (research P2.6). |
| A message lint tool | The equality test proves the same text with no new tool (research §3). |
| Redesign at the third block | The operator said "over 3 times". An earlier draft read this as the third block and stated that the operator decided it, which was false. |
| Keep the scratch scripts and document them | A script has no test, and the operator asked for verbs with tests (plan section 1). |
