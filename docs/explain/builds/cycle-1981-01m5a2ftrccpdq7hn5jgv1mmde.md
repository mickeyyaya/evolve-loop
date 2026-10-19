# Build Explanation — Cycle 1981

## Build Binding
- Cycle: 1981
- Base SHA: fa6b21ec97215af9a00f921cdf46d88bdfad5fff

## Summary
An exhausted per-model usage window (claude's Opus or Fable week) now benches that one model, not the family. The bench is stored beside the family benches and stays out of every family view.

## Rationale
Before, such a window only logged a WARN, and a phase on the drained model failed at run time. A model entry in the existing bench file reuses the lock, the reset margin and the reset cap. The new key `family\x00model` cannot collide with a family or a scope key.

## Changed Areas
- `go/internal/clihealth/clihealth.go` — adds `Entry.Model`, `BenchModelUntil` and `ActiveModels`; `Active` and `Expired` skip model entries, so the router and the canary see no new family.
- `go/internal/usageprobe/windows.go` — a drained per-model window benches (family, model) until its reset, with one bench per model; the WARN stays.
- `docs/architecture/packages/internal-clihealth.md` — records the model entry and its views.
- `docs/architecture/packages/internal-usageprobe.md` — records the new probe behavior.

## Design Decisions
One file and one key space, not a second store. Rejected: a new Expired view for models, because the canary would probe a model key as a family. A window with no mapped family benches nothing and keeps the WARN.

## Verification
`go test -count=1` passes for clihealth, usageprobe and the canary test in cmd/evolve. `go test -tags acs ./acs/cycle1981/` passes.

## Compatibility
Existing family and scope keys and the bench file schema stay valid; `model` is an omitted-when-empty field.

## Limitations
Acceptance items 2, 4 and 5 (routing demotion, the canary model probe, the manifest model map) need steps 3-5 of the plan. No router code reads `ActiveModels` yet.
