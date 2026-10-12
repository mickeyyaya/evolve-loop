# Build Explanation — Cycle 1866

## Build Binding
- Cycle: 1866
- Base SHA: 164bea7f503be47ca8dd89e0db4db9decf34db6b

## Summary
The phase registry is now the single source of the phase-to-agent mapping. The registry names the agent of `tdd`, `build` and `audit`, and `internal/cliroute` no longer reads the built-in `phasecontract` table to map a phase key to an agent.

## Rationale
Before this build, the registry left `agent` empty for three spine phases, so `PhaseSpec.AgentName()` returned `evolve-tdd`, `evolve-build` and `evolve-audit`. No profile and no persona has those names. `cliroute` hid the gap with a second lookup in `phasecontract.For`, and `phaseinventory` showed the wrong names. A fix in the registry corrects every reader of `AgentName()` at once; the second lookup then has no purpose.

## Changed Areas
- `docs/architecture/phase-registry.json` — sets `agent` to `evolve-tdd-engineer`, `evolve-builder` and `evolve-auditor`, which match the tracked profiles and personas.
- `go/internal/cliroute/compile.go` — removes the `phasecontract.For` step from `agentOfPhase`, so a phase key resolves through the catalog alone.
- `go/internal/cliroute/helpers_test.go` — the synthetic catalog names the same agents as the real registry, because the catalog is now the only source.
- `go/internal/cliroute/compile_test.go` — the nil-catalog test now pins the new rule: without a catalog, a phase key that is not a profile is refused; with the catalog, `audit` resolves to the auditor.
- `docs/architecture/packages/internal-cliroute.md` — the package page states the single-source rule in place of the removed contract step.
- `go/internal/phasespec/catalog_profile_realregistry_test.go` — the TDD phase's test: every llm phase of the real registry resolves to a tracked profile and persona.
- `go/acs/cycle1866/predicates_test.go` — the TDD phase's four acceptance predicates for this task.
- `.evolve/evals/phase-agent-mapping-single-source.md` — the TDD phase's eval for this task.

## Design Decisions
The agent values carry the `evolve-` prefix, like `evolve-architecture-design`. Profile resolution strips the prefix (`internal/phaseconfig/phaseconfig.go:82`), and persona resolution uses the full name (`go/cmd/evolve/cmd_cycle.go:688`), so both resolve. The rejected option was `"agent": "builder"`: the persona `agents/builder.md` does not exist. A second rejected option was a one-phase catalog for the per-launch router. `RefuseLaunchRouter` already refuses a declared table on that path, so no `agents` key reaches it.

## Verification
The four cycle predicates and the real-registry test pass. The `cliroute`, `phaseinventory`, `phasespec`, `phasecontract`, `phaseconfig`, `phaseregistrar` and `config` suites pass with `-count=1`. The ACS suite reports `red=0`.

## Compatibility
The production router compiles with the merged catalog, so `agents.audit` still resolves to `auditor`. The `phasecontract` built-in contracts keep their `AgentName` values; this build does not change them.

## Limitations
A caller that compiles a declared table without a catalog now gets a "neither" finding for a phase key. No production caller does this.
