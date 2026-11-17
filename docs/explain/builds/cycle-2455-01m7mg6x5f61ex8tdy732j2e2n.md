# Build Explanation — Cycle 2455

## Build Binding
- Cycle: 2455
- Base SHA: ed3bf7d91654154ae0d35b697bd017017f0cf911

## Summary
This build delivers a design for the coarse `cmd/evolve` admission cell. It scores four approaches and chooses file keys for leaf packages. It changes no code.

## Rationale
Admission holds two items that edit different files of `cmd/evolve`, because both edit one package. The inbox item asks for a design first. A document cycle delivers options and a recommendation, so the build adds them next to the plan.

## Changed Areas
- `solutions/admission-splits-the-coarse-cmd-evolve-cell/assumptions-and-evidence.md` — lists the assumptions and the cited evidence that the options use.
- `solutions/admission-splits-the-coarse-cmd-evolve-cell/options/1-file-keys-for-leaf-packages.md` — the chosen approach: file keys for leaf packages.
- `solutions/admission-splits-the-coarse-cmd-evolve-cell/options/2-one-package-per-verb.md` — the rejected approach that moves each verb to its own package.
- `solutions/admission-splits-the-coarse-cmd-evolve-cell/options/3-conflict-rule-reads-symbols.md` — the rejected approach that reads edited symbols.
- `solutions/admission-splits-the-coarse-cmd-evolve-cell/options/4-self-registering-verbs.md` — the staged complement that removes the `registry.go` hot file.
- `solutions/admission-splits-the-coarse-cmd-evolve-cell/recommendation.md` — compares the options, names the winner and the runner-up, and says what evidence flips the choice.

## Design Decisions
File keys apply to leaf packages only, because a leaf has no reach. The rollout uses one new policy list, `fleet.orchestrator.file_cells`, that holds the leaf packages with file keys. It is configuration data, so no flag is added. The existing key `fleet.orchestrator.admission` is not a stage switch, because it sets the rule of the whole fleet.

## Verification
`evolve solution check` accepts the directory. `evolve docs ste-lint --strict` reports 0 findings on the changed set.

## Compatibility
No code, schema or policy changes. Admission behavior stays the same.

## Limitations
The share of waiting items that avoid `registry.go` is not measured. The plan measures it first. The docs review runs at the console.
