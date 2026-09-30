# Comment history: `acs/cycle613`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle613/predicates_test.go:3` — above `package cycle613`

```text
// Package cycle613 materialises the cycle-613 acceptance criteria for the
// single triage-committed top_n task, advisor-skill-selection (weight 0.92,
// .evolve/inbox/2026-07-07T18-30-00Z-advisor-skill-selection.json): the
// advisor gains authority to PROPOSE per-phase skill sets — following the
// exact {cli,tier} soft-overlay precedent (runner.go:429-447,
// router.ClampPlanModelRouting) — clamped by the kernel against the
// filesystem skills registry, never trusted as free text.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…574
// precedent) — each predicate shells `go test -run` over the RED unit tests
// authored this cycle in internal/policy (advisor_skill_overlay_test.go).
// None is a source-grep; every one exercises the system under test
// (Policy.ClampAdvisorSkills, Policy.ResolveOverlaysWithAdvisor,
// policy.SkillRegistryFromFS over a real temp-dir filesystem walk) and
// asserts on its result. RED now: internal/policy does not compile
// (AdvisorOverlayPolicy / AdvisorSkillRejection / ClampAdvisorSkills /
// ResolveOverlaysWithAdvisor / SkillRegistryFromFS all undefined). GREEN
// once Builder implements the clamp/merge/registry contract.
//
// Scope: the dispatch wiring into Engine.Launch / PhaseRequest / the advisor
// prompt's registry section, the advisor-rejections.json artifact plumbing,
// and the StaticPrefix cache-contract round-trip are dispositioned
// manual+checklist in test-report.md, not predicated here — see that file's
// Coverage Map for the rationale and the Auditor checklist.
```
