# Build Explanation — Cycle 2139

## Build Binding
- Cycle: 2139
- Base SHA: 85a54168239047f6b427339800d20a256dafa2f6

## Summary
Every agy launch passes the fast model of the agy manifest when invoked without a tier or with an unresolved tier token.

## Rationale
Without an explicit model flag, agy uses its local default and depletes the Argon pool. A dispatch launch with an unresolved tier token must carry the manifest fast model. The CLI-health canary must keep launching agy with no model flag, because it tests the CLI default. The first design sniffed the binary name in the realizer and defaulted an empty tier to fast. That broke the canary and duplicated manifest data. The final design adds an `unresolved` field to the model_tier parameter spec, so the manifest names the fallback tier and the realizer reads it.

## Changed Areas
- `go/internal/bridge/driver_agy.go` — removes the agy model name literal from the message to satisfy the literal ban.
- `go/internal/bridge/manifests/agy-tmux.json` — declares `unresolved: fast` on the model_tier parameter.
- `go/internal/bridge/realizer.go` — reads the manifest `unresolved` tier for unresolved tokens and keeps an empty tier model-less.

## Design Decisions
The `unresolved` field of the parameter spec is the single home of the fallback. Any manifest can opt in. A manifest without the field omits the model for an unresolved token, as before. An empty tier adds no model.

## Verification
Targeted predicates verify that untiered launches, unresolved tokens, and doctor probes carry the fast model. Non-agy CLIs remain unaffected.

## Compatibility
Existing resolved tier mappings and explicit raw model identifiers remain unchanged and pass through verbatim.

## Limitations
This policy applies to agy launches only and does not change model fallback behavior for other CLIs.
