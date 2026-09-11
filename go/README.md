# evolve-loop Go binary

The Go binary is the sole evolve-loop runtime. It contains the orchestrator,
phase implementations, provider bridge, trust kernel, cycle/loop dispatch,
verification commands, and release pipeline. There is no shell runtime
fallback; see [migration-from-bash.md](../docs/migration-from-bash.md) for the
historical port.

## Build

```bash
make build         # ./bin/evolve
./bin/evolve version
```

## Test

```bash
make test          # go test -race -cover ./...
make cover         # writes coverage.html + per-pkg %
make lint          # go vet + gofmt -d
```

## Layout

```
cmd/evolve/        # CLI entrypoint; one file per subcommand
internal/
  core/            # orchestrator, cycle state machine, and consumer-side ports
  phases/          # concrete phases plus the shared phase runner
  bridge/          # provider drivers, tmux REPL lifecycle, completion evidence
  adapters/        # filesystem / subprocess / sandbox impls
  guards/          # trust kernel (ship, phase, role, docdelete, quota, chain)
  log/             # slog wrappers + abnormal-events sidecar
  projecthash/     # 8-char SHA256 multi-project namespace
  acsrunner/       # go test -json driver for ACS predicates
pkg/
  version/         # build-stamped version string
  acsassert/       # ACS predicate assertion DSL
  phaseproto/      # JSON subprocess phase override protocol (Phase 2)
testdata/          # golden fixtures
```

## Architecture

`cmd/evolve` is the composition root and may import concrete phase packages.
`internal/core` dispatches phases through its ports and owns lifecycle safety;
it does not depend on concrete phase implementations. See
[phase-architecture.md](../docs/architecture/phase-architecture.md) for the
current runtime, phase, Audit, Ship, resume, and resource-ownership contracts.
