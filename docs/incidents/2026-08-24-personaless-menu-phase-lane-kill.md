# 2026-08-24 — Persona-less menu phase kills a lane at load-agent (cycle-1551)

## Symptom

soak-20260824a wave 1, cycle-1551: the advisor SELECTed the catalog phase
`defect-disposition-preflight` (optional, on the menu). Its `phase.json`
declares no `agent`, the derived persona
`agents/evolve-defect-disposition-preflight.md` exists nowhere, and no
phase-local prompt source exists either. `BaseRunner.Run` failed at the
load-agent step with `prompts: read agents/evolve-defect-disposition-preflight.md:
no such file or directory`; the cycle died rc=4; the ADR-0072 halt stopped the
batch. A full lane was killed by an optional enrichment phase's missing file.

## Scope at discovery

Four tracked catalog phases carried the defect (persona resolvable nowhere):
`defect-disposition-preflight` and `pre-audit-evidence-check` were ON the
SELECT menu; `defect-disposition-ledger` and `ship-stage-hygiene-check` were
reachable on demand. The class is born whenever a phase directory lands
without a matching `agents/evolve-<name>.md` — including the exact shape
`evolve phases add` scaffolds (it writes a phase-local `agent.md` that NO
dispatch path reads).

## Root cause

The SELECT menu is projected from the catalog with no persona-resolvability
check, and the runner treats a missing persona as an ordinary fatal error, so
an advisor choice of an undispatchable phase is lane-fatal. The asymmetry:
`registerBuiltinSpecRunners` already refused to register persona-less BUILTIN
spec phases (WARN + skip), but the user-phase registration loop had no such
check.

## Fix (three independent layers + hardening)

1. **Prevention at the runtime seam** — `discoverUserSpecsClamped` (the single
   composed discovery path) now demotes any spec whose persona doc does not
   load to `catalog:"on-demand"` IN MEMORY (`demotePersonalessSpecs`,
   `cmd/evolve/phaseroots.go`), with a WARN naming the missing file. Covers
   tracked and untracked specs on any host.
2. **Fail-soft at dispatch** — the runner wraps a genuinely-absent persona in
   `core.ErrAgentDocMissing` (`runner.go` load-agent step);
   `optionalInfraSkip` admits it via the single-source predicate
   `core.IsOptionalSkippableError`. An OPTIONAL phase degrades to a recorded
   WARN skip (own ledger kind `optional_missing_persona_skip`, diagnostic on
   the synthesized response); mandatory/floor phases still fail loud.
3. **Repo hygiene guard** — `TestRepoPhaseCatalog_MenuPhasesResolveAPersona`
   (internal/core) reds any tracked menu phase whose `agents/<agent>.md` is
   absent. The two on-menu offenders' phase.json got `catalog:"on-demand"`.
4. **Hardening** — the prompts zero-loader now carries `prompts.ErrNoSource`
   alongside its documented `fs.ErrNotExist`, and the runner declines the
   missing-doc classification for it: a misresolved prompts root (EVERY doc
   "missing") is a wiring defect and must stay loud, never a mass silent skip.

## Non-fix

The tracked eval `.evolve/evals/user-phase-persona-resolution-core.md`
specifies a phase-local `agent.md` resolution mode (Loader.AgentForPhase).
This incident's fix deliberately does NOT implement it — `agents/<name>.md`
remains the only persona source for disk specs — and the eval needs re-scoping
against the demotion+fail-soft design before any of its modes land (three
mechanisms for one class is the ceiling; see the anti-duplication rule).

## Regression pins

`internal/core/agent_doc_missing_test.go` (admission + mandatory/floor/other-error
negatives + end-to-end ledger-kind replay), `internal/core/optional_skip_details_test.go`,
`internal/core/infra_teardown_single_source_test.go::TestOptionalInfraSkip_GateAgreesWithIsOptionalSkippableError`,
`internal/phases/runner/agent_doc_missing_wiring_test.go` (production load path:
missing / unreadable / nil-source), `internal/prompts` zero-loader both-sentinels pin,
`cmd/evolve/demote_personaless_test.go` (helper + composed discovery path),
`internal/core/advisor_catalog_ondemand_test.go::TestRepoPhaseCatalog_MenuPhasesResolveAPersona`.
Mutation-tested: 11 mutants, all killed.
