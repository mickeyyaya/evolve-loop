# Split cmd/evolve — assumptions and evidence

Every number in the options and the recommendation cites an entry here: `A<n>` for an assumption, `E<n>` for evidence. The census ran on 2026-11-17 at base `27353b988`.

## Assumptions

- **A1** — A lane waits when another lane holds a file in the same Go package, because the package is one admission cell. Source: parent plan `concurrent-edit-layout-2026-11` (commit `0ea7f8c1b`), decision D5 (operator-stated).
- **A2** — Files that change in the same commit belong in the same package. Source: parent plan D5, "split along change coupling".
- **A3** — A move is safe only at a wave boundary with no open lane in the group. Source: parent plan D5.

## Evidence

- **E1** — 302 commits touched `go/cmd/evolve/` in 30 days. Group touches: CORE 183, cycle 74, loop 74, wave 47, inbox 33, gc 15, integration 12, boundary 12, pr 11, quality 11. Citation: `git log --since=30.days --no-merges --name-only --format='@@%h' -- go/cmd/evolve/`, grouped by the token after `cmd_`.
- **E2** — `cycle` and `loop` share 24 commits. `wave` shares 7 with `boundary` and 8 with `quality`. Citation: same command as E1.
- **E3** — Coupling probe: inbox 21 files, 12 outbound symbols, 9 inbound functions. gc 10, 13, 9. wave 11, 47, 5. ci 3, 8, 1. pr 6, 7, 6. Citation: `go build -gcflags=-e` of each group in a scratch package, then a name-match script.
- **E4** — Six groups already follow the shim and `Deps` pattern. Citation: `go/internal/cli/`, `go/cmd/evolve/cmd_cloud.go:10`, `go/cmd/evolve/cmd_selfheal_propose.go:43-46`.
