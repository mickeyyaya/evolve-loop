# Build Explanation — Cycle 2643

## Build Binding
- Cycle: 2643
- Base SHA: 21d6e4ae4b3ed6df1bf56d1b949bb59f8984c697

## Summary
Package `stageevidence` now declares `unhashedRecordFields` in `score.go` alongside `hashedRecord`. Every field of `Record` has an explicit hash decision, and the guard fails when a new field lacks one.

## Rationale
`hashedRecord` and `computeRecordHash` already existed in `score.go`, and only one `DecisionVersion` caller remained at `cmd_stage.go:91`. Adding `unhashedRecordFields` as the single unhashed-field list avoids duplicating field lists across tests and lets the schema guard enforce complete coverage of `Record`.

## Changed Areas
- `go/internal/stageevidence/score.go` — declares `var unhashedRecordFields = []string{"Hash"}` right after `hashedRecord` so every `Record` field has an explicit inclusion or exclusion decision.

## Design Decisions
`unhashedRecordFields` lives in `go/internal/stageevidence/score.go` directly beside `hashedRecord` instead of inside the test file. A single source in production code keeps the hash schema definition unified and lets the reflection and mutation guards detect undecided, contradictory, or stale fields.

## Verification
`gofmt -l ./internal/stageevidence`, `go vet ./internal/stageevidence`, `go test -count=1 ./internal/stageevidence`, and `go test -tags acs -count=1 -v ./acs/cycle2643` verify both the schema guard and mutation checks.

## Compatibility
The `Record` struct, JSON tags, `hashedRecord` fields, and `computeRecordHash` output stay identical. Existing sealed chains keep the exact same hashes.

## Limitations
Only `Record.Hash` is excluded from the hash payload. Any future `Record` field must enter either `hashedRecord` or `unhashedRecordFields` before tests pass.
