# Extension cookbook — how to extend the system

The architecture docs explain **what the system is and why**. These guides are the
missing **how-to** layer: concrete, step-by-step recipes for the changes a future
contributor actually makes — add a phase, add a CLI, write a predicate, run a cycle
locally, follow the contributor loop. Every recipe names real files, types, and
functions in `go/internal/<pkg>` and ends with **the tests that pin it**.

If you are new, read [00-overview/system-in-one-page.md](../00-overview/system-in-one-page.md)
and [architecture/phase-pipeline.md](../architecture/phase-pipeline.md) first — these
guides assume that mental model and link back into it.

## When to use each guide

| Guide | Use it when you want to… |
|---|---|
| [add-a-phase.md](add-a-phase.md) | Add a new pipeline phase (a new stage with its own persona + artifact). Worked example: the debugger phase. |
| [add-a-cli-driver.md](add-a-cli-driver.md) | Teach the bridge a new `--cli` target (a new LLM CLI) so any phase can run on it. |
| [write-an-eval-and-predicate.md](write-an-eval-and-predicate.md) | Author an EGPS acceptance predicate (the executable test the auditor runs) — bash `acs/cycle-N/*.sh` or the Go trust-kernel alternative. |
| [run-and-debug-locally.md](run-and-debug-locally.md) | Run a cycle or loop on your machine, find the artifacts, and read what happened. |
| [the-dev-workflow.md](the-dev-workflow.md) | Understand the full contributor loop: worktree → TDD → build → commit-gate → ship, and the guards that gate each step. |

## How these fit the rest of the base

- The **why** behind each mechanism lives in [architecture/](../architecture/).
- The **failure modes** these recipes help you avoid live in [incidents/pattern-library.md](../incidents/pattern-library.md).
- The **decisions** you should not silently reverse live in [evolution/decision-digest.md](../evolution/decision-digest.md).
