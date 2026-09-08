# ADR-0099 — Deliverable kinds: solution cycles on the same spine

- **Status:** Accepted (2026-09-09). Slice 1 (this ADR's kernel half) landed with the
  `deliverable_kind` / `goal_type` signals, the AND-able conditional rule and the solution
  recipes; slices 2–3 (the deliverable contract + floor + CLI, the prompt-layer skills) follow
  under the same number.
- **Driving evidence:** the operator's 2026-09-09 directive — the factory must produce non-code
  solutions *in this repo, through the same pipeline*: intent and scout unchanged in role, build
  delivering several candidate strategies, audit reviewing them, the **advisor** deciding the
  cycle's shape, and TDD recognised as the code-specific phase. Exploration (2026-09-09) found the
  spine, trust kernel, ledger, ship-as-commit, dossier and lesson schemas already
  deliverable-agnostic; what is hard-bound to code is the *evidence layer* (tdd writes Go
  predicates, the build floor runs `go test`, the ACS gate reads `go test -tags acs`, the
  commit-gate lanes are Go-only, the builder/tdd/auditor personas are 55–90 % code wording).
  Two dormant assets shaped the design: the 15 shipped domain phases of
  [domain-phase-catalog.md](../domain-phase-catalog.md), whose `scout.goal_type` triggers had
  **no producer** (handoff JSON has been extinct since ~cycle 215 and nothing wrote the signal to
  a report header), and the `.evolve/domain.json` adapter documented in
  `docs/reference/configuration.md` with **zero Go readers**.
- **Related:** [ADR-0060](0060-data-driven-transition-kernel.md) (the floor is quantified over
  roles, never phase names — this ADR adds one more config-evaluated condition to it),
  [ADR-0098](0098-task-contract-block.md) (the task contract carries the kind in slice 2),
  [ADR-0084](0084-gate-integrity-invariants.md) (machine-graded contracts: the document floor
  is deterministic and the rubric judgment stays in audit).

## Decision

1. **One new kernel-trusted signal.** `deliverable_kind ∈ {code, document}` is declared by scout
   (`deliverable_kind:` header line in `scout-report.md`, next to a `goal_type:` line naming a
   `phase-registry.json:config.goal_recipes` key) and refined by triage (`deliverable_kind:` next
   to `cycle_size_estimate:` in `triage-report.md`; a mixed `top_n` is `code`). The kernel parses
   both from the report headers (`router.scoutFromReportFallback`,
   `router.triageFromReportFallback`) — the same trusted path `cycle_size_estimate` takes —
   and lifts them onto `RoutingSignals` (typed `Scout.GoalType`, `Scout.DeliverableKind`,
   `Triage.DeliverableKind`; routable as `scout.goal_type`, `scout.deliverable_kind`,
   `triage.deliverable_kind` and the projected `deliverable_kind`). `RoutingSignals.DeliverableKind()`
   is triage > scout > **`code`**: the absent default is the conservative side, so a rule
   `deliverable_kind != document` holds at plan time and is released only by a digested document
   declaration (the post-scout RePlan). An unrecognised kind word normalises to undeclared
   (`router.NormalizeDeliverableKind`) and can never release anything.
2. **The advisor shapes the cycle; config releases the pin.** `config.CondRule` gains AND-ed
   clauses (`a!=b && c!=d`; the head clause keeps the legacy shape so every existing consumer is
   byte-identical; a malformed clause fails the whole rule). The registry's
   `conditional_mandatory.tdd` becomes `cycle_size!=trivial && deliverable_kind!=document`.
   `router.tddPinned` and `shouldRun` read it unchanged; the advisor's rubric renders one
   exemption line per clause. Three solution recipes (`strategy-options`, `business-plan`,
   `partnership-deal`) compose the document-side acceptance from the existing catalog —
   `scope-baseline` (deliverables · acceptance criteria · exclusions) before build,
   `adversarial-review` / `premise-challenge` after — so the advisor selects and justifies
   rather than a Go switch deciding.
3. **Domain phases fire.** With a producer for `scout.goal_type`, the shipped overlays'
   `insert_when` triggers evaluate live (`TestDomainPhaseTrigger_FiresOnScoutGoalType` reads the
   real `forces-analysis` overlay). D2 fail-closed semantics are preserved: an undeclared goal
   type is *absent* on the generic plane, so an `ne` trigger never fires on it.
4. **Slice 2 (deliverable contract, floor, CLI, task contract).** For `document` cycles the
   deliverable is `solutions/<slug>/` with `options/<n>-<name>.md` (≥ 2), `recommendation.md`
   and `assumptions-and-evidence.md`, declared as config
   (`phase-registry.json:config.deliverable_kinds.document`) and checked by ONE deterministic,
   LLM-free engine (`internal/solutioncheck`) projected three ways: a build-floor check composed
   into `DefaultBuildFloorChecks`, `evolve solution check <dir>` (the eval `[code]` grader and
   the agent's self-check), and an audit gate line. `inboxbatch.Item.DeliverableKind` rides the
   ADR-0098 task contract into the tdd/build/audit prompts; `.evolve/domain.json` gets its first
   Go reader (project default kind). Ship is unchanged: commit + push under a `solution(<slug>)`
   prefix scoped to `solutions/**`; the EGPS gate stays satisfied by the always-present
   regression suite, and the solution floor + contract gate are the deterministic acceptance.
5. **Slice 3 (prompt layer).** `solution-scout`, `solution-build` (diverge into ≥ 2 distinct
   options with the inspirer lenses → comparison matrix → recommendation; every number sourced)
   and `solution-audit` (rubric-judge per option with `path:line` evidence, numbers traced to
   `assumptions-and-evidence.md`, adversarial steelman of the non-recommended options) are
   skills selected by a policy overlay rule extended with a `when` signal selector; the advisor
   may add more. The scout's Implementation-First rule becomes kind-conditional.
6. **No flags.** The kind is a typed value in config and report headers dispatched through
   existing seams (the conditional rule, the overlay rule, the floor engine composition).

## Rejected

- A global `EvidenceStrategy` mode switch in Go — contradicts "the advisor decides" and would
  be a second router.
- Go-test predicates over documents — forces the acceptance runtime onto solution authors and
  contradicts "TDD is the code-specific phase".
- LLM-rubric-only acceptance — no machine-graded contract (ADR-0084 invariant 2).
- Separate solution repos — the operator chose in-repo; the project-shape detection that would
  have needed is deferred with them.

## Consequences

- `router`, `config`, `phaseio`, `core` (advisor digest, shadow rows), `routingtest` fixtures
  carry the two new fields; `TestEvalCondition_PresentEmptyString` and
  `TestTriggerFires_AbsentFieldFailsClosed` still hold (typed value when declared, generic-plane
  semantics otherwise).
- `docs/architecture/phase-registry.json` is the registry the loop loads (`cmd_cycle.go`); the
  runtime plane's ignored `.evolve/phase-registry.json` copy is a dead June artifact.
- Persona edits (scout/triage header lines and task bullets) land with slice 3; until then the
  kernel's default keeps every cycle `code`, byte-identical to today.
