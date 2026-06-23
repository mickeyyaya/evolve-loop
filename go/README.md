# evolve-loop Go binary

Single-binary Go rewrite of the evolve-loop orchestrator and trust kernel.
See [plan](../scripts/) (parent plan at `~/.claude/plans/this-is-a-big-parallel-locket.md`).

This module is intentionally separate from the repo root so the bash codebase
(`scripts/`, `agents/`, `skills/`) continues to operate during the rewrite.

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
  core/            # orchestrator + ports; zero infra imports
  phases/          # one package per phase (Phase 2)
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

## Phase 1 scope (this branch)

`go-rewrite-phase-1` covers scaffolding + trust kernel + ledger + ACS runner +
proof-of-concept 50 ACS predicates. Bridge integration, phase implementations,
and `evolve loop`/`evolve cycle run` come in Phase 2.
