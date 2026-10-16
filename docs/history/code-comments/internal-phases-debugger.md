# Comment history: `internal/phases/debugger`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d10

### `go/internal/phases/debugger/debugger.go:159` — above `b.WriteString("\n## Ship Failure Envelope\n")`

```text
// Ship-failure envelope, carried in via PhaseRequest.Context by the
// orchestrator (string keys: ship_error_code/class/stage/debug). Each key is
// optional; only render what is present so a partial envelope still produces
// a clean prompt. ADR-0050 §3.10 Slice 2: at enforce the typed ErrorContext
// channel replaces those keys (byte-identical — Active() is false unless
// enforce, and the zero ErrorContext renders nothing, matching an empty map).
```

### `go/internal/phases/debugger/debugger_phaseio_test.go:11` — above `func TestDebugger_ComposePrompt_TypedEqualsMap(t *testing.T) {`

```text
// ADR-0050 §3.10 Slice 2: debugger reads the ship-failure envelope from the typed
// ErrorContext at enforce (req.Input.Active()) and the legacy ship_error_* Context
// keys below it.
```

### `go/internal/phases/debugger/debugger.go:200` — above `ContractVerifier func() runner.ContractVerifier`

```text
// ContractVerifier is the deliverables gate's verifier accessor for the
// verdict engine (runner.Options.ContractVerifier): one verifier for gate
// and engine (research F22). nil = the catalog-aware default.
```
