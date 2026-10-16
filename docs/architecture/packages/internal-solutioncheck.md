# internal/solutioncheck

## Purpose

`internal/solutioncheck` is the one deterministic, LLM-free judge of the deliverable shape of a document cycle (ADR-0099 slice 2). A `document` cycle delivers `solutions/<slug>/`: at least N candidate options, a recommendation and an assumptions-and-evidence file.

## Design

- **One engine, three projections.** The build handoff floor (`core`), the `evolve solution check` CLI and the audit gate line all call this engine. The CLI is the eval code grader and the self-check of the agent.
- **Contract is config.** The contract is `phase-registry.json:config.deliverable_kinds`. `Spec` aliases `config.DeliverableKindSpec`, so the engine and its consumers share one type. An empty `spec.Root` is a config load warning and never a runtime default.
- **Quality stays out.** Judgement of quality (rubrics, strength of evidence) stays in the audit phase, never here.
- **`Check`.** It returns every violation as a `Failure` (`path: reason`). An absent solution directory is one loud failure. Every other violation is reported on its own, so one re-dispatch can fix them all.
- **`checkDocument`.** Per file: non-empty, required sections present, and no placeholder left. When an evidence file is named, the file must cite it, so every number traces to an assumption or an evidence entry. The evidence file itself cites nothing.
- **`hasHeading`.** A section heading is level two or deeper. The H1 is the title. Matching ignores case and surrounding whitespace.
- **Placeholders.** Each forbidden token is a whole-word, case-insensitive match.
- **`Describe`.** One renderer over the registry spec gives the Task Contract block to the builder and the auditor. The words the builder is told and the shape the floor judges cannot drift. A zero spec renders `""`.

## Invariants

- **The slug names one directory.** It must match the id vocabulary that every id-consuming site enforces (`triagecap`, `evalgate`, `phasespec`): lowercase, digits, hyphens. A path segment, `..`, or an uppercase or underscore id is a contract violation. The single engine reports it, so no caller has to remember to confine it. A slug outside the vocabulary is never joined into a path.
- **Code is not a heading.** Lines inside fenced code and indented code (four spaces or a tab) are not headings. The words of a required section pasted inside a fence must not satisfy the structural check.
- **Fences close on their own character.** Per CommonMark, a fence closes only on a run of the same character, at least as long as the opener. Anything else is fence content. `fenceRun` returns the character and the run length, and a length below 3 is not a fence.
