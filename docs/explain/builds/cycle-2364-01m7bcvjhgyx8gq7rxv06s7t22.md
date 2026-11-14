# Build Explanation — Cycle 2364

## Build Binding
- Cycle: 2364
- Base SHA: ba124fd277f74d0e4303fab35738c3be21bad361

## Summary
`evolve gc` now recycles orphaned Go test temp directories and spares each directory that a live process holds. It states a free-disk floor and warns before and after that floor. The seat brief forbids deleting shared temp by hand and names `evolve gc` as the one route.

## Rationale
The old sweep matched pipeline names only and had no liveness proof, so `Test*` and `gittest*` directories filled the disk. A name and an age do not prove that a directory is free. A liveness proof from `lsof` does, and it lets the name shape widen safely.

## Changed Areas
- `go/internal/gc/tempdir.go` — adds the `Test…` and `gittest…` name shapes (6 or more digits), the `SparingHeld` option and the `Held` count. A held directory is spared. An unknown holder state spares the directory and reports an error.
- `go/internal/gc/openpaths.go` — adds `ListOpenPaths` and `LiveHolderProbe`: one lazy `lsof` snapshot per run, matched against each candidate and its resolved path.
- `go/internal/gcpolicy/gcpolicy.go` — adds `disk_floor_gb` and `disk_warn_gb` with the defaults 10 GB and twice the floor.
- `go/cmd/evolve/cmd_gc.go` — the temp step uses the probe and reports spared directories. The full sweep calls the disk signals.
- `go/cmd/evolve/cmd_gc_scope.go` — the cache scope calls the disk signals before any step.
- `go/cmd/evolve/cmd_gc_disk.go` — states the floor on stdout and writes the `WARN` lines on stderr.
- `go/internal/bridge/phaseidentity/identity.go` — adds the seat-brief line that forbids deleting shared temp by hand.
- `docs/architecture/policy-config.md` — documents the two keys.
- `docs/architecture/packages/internal-gc.md` — documents the liveness proof and the new invariant.

## Design Decisions
`ReapPipelineTemp` keeps its three arguments and takes the probe as a variadic option, so existing callers and the frozen reproduction test compile. The probe is a closure over one `lsof` listing, so the cost is one call per run. Both the full sweep and the cache scope reach the same `pipelineTemp` seam. Alternative rejected: a per-candidate `lsof +D`, which is slow on large trees.

## Verification
`go test -tags acs -count=1 ./acs/cycle2364` passes 9 of 9. Unit tests cover the probe, the held and unknown-state cases, the name shapes, the policy defaults and the disk signals.

## Compatibility
No policy key changes meaning. Two keys are new and optional. Without them the floor is 10 GB.

## Limitations
The probe sees only processes of the same user. A directory held by another user is not seen.
