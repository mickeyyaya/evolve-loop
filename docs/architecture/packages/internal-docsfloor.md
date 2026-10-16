# internal/docsfloor

> Related: [ADR-0077](../adr/0077-docs-floor-for-architecture-changes.md) (the design). This page keeps the package-level detail that the code cannot say.

## Purpose

`internal/docsfloor` mechanizes the documentation half of rule 2 of section 3 of the operating policy: an architecture-labeled change must touch documentation. It is a pure decision function. The build handoff floor surfaces its verdict.

## Design

- **A rule that had no compiled enforcement.** The prose rule says that architecture changes also get an adversarial architect review (`always_full_documentation`, `doc_stewardship_policy`). A change can rewrite the trust kernel and ship with no ADR and no design doc. This package is the counterpart of the spine floor for that rule.
- **Stage-dialed.** The stage comes from `.evolve/policy.json` (`docs_floor.stage`, compiled default `enforce`). `off` disables the gate. `shadow` and `enforce` both evaluate, and the action is WARN at both. The split exists so a later promotion to a blocking verdict has a dial to turn. An empty stage means `enforce`, as `policy.DocsFloorConfig` does.
- **Warn, never reject.** An editor decides if a doc is adequate. A machine can check that a doc exists. The gate reports the mechanical half loudly and leaves the judgement to the auditor.
- **Zero intra-repo imports.** Every layer (core, the policy consumers, the CLI) can depend on the package without an import cycle.
- **The label comes from the diff.** A rushed change omits a self-reported label. So the package reads the diff. An architecture change is a change to an architecture surface.

  The surfaces are the trust kernel (orchestrator state machine, policy resolution, routing config) and the contract registry that every phase vocabulary derives from. `LabelArchitecture` is exported. A caller with a better signal, such as an explicit operator label, can supply its own.
- **Two classifiers.** `LabelArchitecture` stays narrow and drives the WARN-only verdict on the broad question "did this touch the kernel". `IsArchitectureClass` is the blocking-grade classification. Its list adds the surfaces that carry no vocabulary but decide what ships: the guard chain, the ship path and the fleet scheduler and composer. It pays for them with three precision rules.

## Invariants

- **Three precision rules of `IsArchitectureClass`.** Each removes a false positive that blocks an ordinary cycle.
  1. A `_test.go` file never labels. A test-only change to policy adds no vocabulary. An ADR for it taxes every regression test.
  2. The namesake file of a package (`go/internal/<pkg>/<pkg>.go`) labels, also outside the declared surfaces. A path classifier cannot see that a package is new. The namesake file is created once, when the package is born, so it is the deterministic proxy.
  3. A phase spec under `phases/` labels. Specs are vocabulary: a minted phase with no doc is the same gap as an undocumented policy key.
- **Fail open.** Anything not matched is not architecture-class, so bugfix, test-only and docs-only cycles are untouched.
- **Decision order of `Evaluate`.** Stage off gives SKIP. Not architecture-labeled gives SKIP. An empty change set gives SKIP, because an unreadable or empty diff must not manufacture a warning. At least one documentation touch gives PASS. Otherwise the result is WARN.
- **A gate that cannot apply reports SKIP.** It never reports a vacuous pass, so "not judged" is never read as "judged clean".
- **A WARN always has a reason.** An unexplained warning is unactionable. The reason names what is missing and cites the policy clause (`TestEvaluateWarnReasonNamesTheRule`).
- **Two documentation prefix lists, on purpose.** `docPrefixes` is narrow. ADRs, design docs and operating policy all live under `docs/`, so one prefix covers them. An incidental README edit elsewhere cannot satisfy the floor.

  `DocsRoots` (the blocking-grade floor) is narrower: the architecture docs tree and the operator runtime reference. A release note or a README edit is not a record of an architecture decision. Pinned by `TestHasDocsDelta`.
- **`Verdict` is a plain value type**, so a caller can build and compare one (`TestVerdictIsAValue`).
