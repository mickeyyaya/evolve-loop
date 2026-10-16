# Comment history: `internal/phases/intent`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d10

### `go/internal/phases/intent/intent.go:52` — above `goal := req.Context["goal"]`

```text
// The persona's template instructs it to "parse the user's goal" —
// that requires the actual TEXT, not just the hash. When the
// operator passes --goal-text "...", the dispatcher routes it
// through Context["goal"] so intent.md gets structured around the
// real goal instead of inferred from leftover workspace artifacts.
// ADR-0050 §3.10 Slice 2: read from the typed envelope at enforce, the legacy
// Context map below it (byte-identical — Active() is false unless enforce).
```

### `go/internal/phases/intent/intent.go:67` — above `b.WriteString("- mode: delta (the goal changed since the last batch; emit intent-delta.md describing what is new or chan…`

```text
// The unchanged-goal case is decided deterministically in ShouldSkip
// BEFORE dispatch (ADR-0062/T1.5, Core Rule 5), so by the time the LLM
// runs the goal HAS changed — ask only for the delta, not a hash compare.
```

### `go/internal/phases/intent/intent.go:90` — above `func (hooks) ShouldSkip(req core.PhaseRequest) (bool, string, string, []core.Diagnostic) {`

```text
// ShouldSkip implements runner.Skipper: it makes the delta-mode "is the goal
// unchanged?" decision DETERMINISTICALLY in code (Core Rule 5 / ADR-0062 T1.5),
// instead of delegating a goal-hash comparison to the LLM prompt. When this
// cycle's goal hash matches the current batch's recorded hash, the goal is
// unchanged → skip intent (and its LLM dispatch entirely) and route to scout.
// Any uncertainty (full mode, blank hash, unreadable state) FAILS OPEN: intent
// runs. When the goal changed, intent runs so the LLM synthesizes the delta.
```

### `go/internal/phases/intent/intent.go:132` — above `ContractVerifier func() runner.ContractVerifier`

```text
// ContractVerifier is the deliverables gate's verifier accessor for the
// verdict engine (runner.Options.ContractVerifier): one verifier for gate
// and engine (research F22). nil = the catalog-aware default.
```

### `go/internal/phases/intent/intent_phaseio_test.go:11` — above `func TestIntent_ComposePrompt_TypedGoalEqualsMap(t *testing.T) {`

```text
// ADR-0050 §3.10 Slice 2: intent reads the goal from the typed envelope at enforce
// (req.Input.Active()) and from the legacy Context["goal"] below it.
```

### `go/internal/phases/intent/intent_test.go:337` — above `func TestComposePrompt_InjectsGoalTextFromContext(t *testing.T) {`

```text
// TestComposePrompt_InjectsGoalTextFromContext is the regression test
// for cycle-108 bug #3. When the operator passed --goal-text "foo",
// the dispatcher routes it through Context["goal"]; the intent
// persona's prompt MUST surface it in the Cycle Context block so the
// persona structures intent.md around the real goal rather than
// inferring from workspace leftovers.
```
