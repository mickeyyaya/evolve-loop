# Eval: interaction-rollup-tmp-collision
## Graders
- [code] `cd go && go test -count=1 -race ./internal/interaction/...` exits 0, including a concurrent WriteRollup test (two writers, valid JSON result, no stray .tmp) and a concurrent Record test (N goroutines, N whole parseable ledger lines).
- [code] `grep -n '\.tmp"' go/internal/interaction/rollup.go` returns nothing (no fixed tmp path).
- [code] `cd go && go run ./cmd/commentaudit comments -base HEAD~1 internal/interaction` exits 0.
