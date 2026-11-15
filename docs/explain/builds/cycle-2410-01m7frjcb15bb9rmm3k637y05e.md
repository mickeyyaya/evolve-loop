# Build Explanation — Cycle 2410

## Build Binding
- Cycle: 2410
- Base SHA: 5ce7e48e242115eb98c6deab81e97ea5ad159cf0

## Summary
The isolation specification, test convention, and gittest package note now document AD1 credential isolation as built in ASD-STE100.

## Rationale
Aligning the documentation with the implemented Go packages (`ipcenv`, `gittest`, `bridge`) closes the AD1-9 documentation gap without changing production code or weakening existing sandbox guarantees.

## Changed Areas
- `docs/architecture/llm-isolation.md` — updates §8.5, §9, §9.2, and §16.2 (`ipcenv.CredentialIsolation`, `process`, `pid`, `read.home.secret`, `refused`, `~/.git-credentials`, `~/.config/git/credentials`, `prStatusFooterEnabled`, `claude-tmux.json`, and `ISOLATION_I5_VIOLATION`).
- `docs/conventions/hermetic-driver-tests.md` — adds the host-credential isolation rule requiring `gittest.IsolateHostCredentials()` and `restore` in `TestMain`, lists all `ipcenv.CredentialIsolation` keys, forbids hand-written `GIT_CONFIG_GLOBAL` lines, and links `../architecture/llm-isolation.md`.
- `docs/architecture/packages/internal-gittest.md` — links `../../conventions/hermetic-driver-tests.md` from the invariants section.

## Design Decisions
Kept `docs/architecture/packages/internal-bridge.md` and `docs/architecture/test-system.md` untouched because `internal-bridge.md` carries pre-existing STE findings outside this scope and `internal-ipcenv.md` already documents `CredentialIsolation`.

## Verification
All 13 ACS predicates in `go/acs/cycle2410` pass and `evolve docs ste-lint --strict` reports 0 findings across the three changed files.

## Compatibility
All relative links resolve, existing preservation clauses (`Deps.LookupEnv`, keychain limits, `ISOLATION_I4_VIOLATION`, `ISOLATION_I6_VIOLATION`) remain intact, and no Go source outside `go/acs/cycle2410/` changed.

## Limitations
`docs-review` is run by the console on the final diff; `docs/architecture/packages/internal-bridge.md` still retains its pre-existing STE findings.
