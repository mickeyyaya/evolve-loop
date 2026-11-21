# Build Explanation — Cycle 2584

## Build Binding
- Cycle: 2584
- Base SHA: 23c32f42926c3dcde2c2da08401c897afd66275a

## Summary
The command `evolve logs` reads the log layers D0 to D3 with a hard line budget. The key `log_access` of `phase.json` is accepted, and a read outside it records a signal in shadow mode.

## Rationale
Agents read whole log files and filled their context window. One bounded reader is the smallest change that stops this. A new package holds the logic, so `cmd/evolve` stays a thin parser.

## Changed Areas
- `go/internal/logread/logread.go` — the verbs, the budgets, the truncation line and `Reader.Signals`.
- `go/internal/logread/access.go` — the shadow record emitted on `Reader.Signals` and the prompt block.
- `go/internal/logread/logread_test.go` — unit tests of each verb.
- `go/internal/logread/apicover_named_test.go` — names each exported symbol.
- `go/cmd/evolve/cmd_logs.go` — the parser of `evolve logs`, wired with `newRoleSignalCenter`.
- `go/cmd/evolve/cmd_root_guard_test.go` — registers `cmd_logs.go:readLogs` in the root signal center allowlist.
- `go/cmd/evolve/registry.go` — registers the verb `logs`.
- `go/internal/phasespec/phasespec.go` — the type `LogAccess` and the field `log_access`.
- `go/internal/phasespec/validate.go` — validates `log_access`.
- `go/internal/phasespec/logaccess_test.go` — tests the validation.
- `go/internal/signalcenter/event.go` — the module `log` and the code constant.
- `go/internal/signalcenter/registry.go` — registers `LOG_ACCESS_SHADOW`.
- `go/internal/signalcenter/event_test.go` — counts the new module.
- `go/internal/signalcenter/shadow_codes_test.go` — names and checks `CodeLogAccessShadow`.
- `go/internal/phases/runner/preparation.go` — calls `withLogDigest` on the composed prompt.
- `go/internal/phases/runner/logdigest.go` — adds the D0 block only for a phase that declares `log_access`.
- `go/.apicover-enforce` — enrols `./internal/logread`.
- `docs/architecture/packages/internal-logread.md` — the package page.
- `docs/architecture/signal-codes.md` — the generated code table.
- `CHANGELOG.md` — the release note.

## Design Decisions
A file window reads line by line and cuts each line at 1024 bytes. A line budget alone cannot bound a file of one huge line. The shadow signal uses a new module `log`, because a code prefix must match its module.

## Verification
The 16 predicates of cycle 2584 pass. Unit tests cover the filters, the windows and the shadow record.

## Compatibility
A phase without `log_access` gets the same prompt. The new field is optional and is omitted when empty.

## Limitations
Shadow mode records a read outside the access and does not block it. Enforcement is the item H5. The agent class table does not list `evolve logs`, and an agent cell cannot run it until the console adds the row.
