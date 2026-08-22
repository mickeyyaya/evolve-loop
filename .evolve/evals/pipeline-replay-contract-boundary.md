---
score_cap:
  - criterion: "Transient recognition is resolved from the LAUNCHED family's manifest — the live cycle-1523 claude pane classifies transient for claude-tmux only, never for codex/agy/ollama"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestArtifactTimeoutTransient_LivePaneIsFamilyScopedNotHardCodedText$' ./internal/bridge"
  - criterion: "Provider error text the agent merely echoed from its prompt never classifies as transient, for any CLI family"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestArtifactTimeoutTransient_EchoedProviderTextNeverClassifiesForAnyFamily$' ./internal/bridge"
  - criterion: "The landed #478 boundary tests still hold — the marker reports transient on the live pane, false on a silent wedge, and an unknown driver fails open"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^(TestRunTmuxREPL_ArtifactTimeout_MarkerFlagsTransientOnLivePane|TestRunTmuxREPL_ArtifactTimeout_SilentPaneIsNotTransient|TestClassifyTransientPane_UnknownDriverFailsOpen)$' ./internal/bridge"
---

# Eval: captured-pane transient boundary is manifest-scoped

> Pins the anti-gaming boundary around #478's transient disclosure. The landed
> family test proves every manifest DECLARES a `transient_regex`; it does not
> prove recognition is SOURCED from the launched family's manifest. An
> implementation that hard-codes `529` or any provider prose, or that scans the
> raw pane instead of the agent-stripped one, passes the landed tests and breaks
> both the family-agnostic contract and the F1 indirect-prompt-injection
> boundary. Cross-family disjointness on one live captured pane is the cheapest
> assertion that discriminates. Source incident: cycle 1532; ADR-0090
> alternatives + decision 6; incident 2026-08-18 §3a/§4.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| family-scoped-recognition | Live 529 pane is transient for claude-tmux only | 8/10 | `go test -run TestArtifactTimeoutTransient_LivePaneIsFamilyScopedNotHardCodedText` |
| echoed-text-rejected | Echoed prompt/provider text never classifies, any family | 8/10 | `go test -run TestArtifactTimeoutTransient_EchoedProviderTextNeverClassifiesForAnyFamily` |
| landed-boundary-intact | #478's marker/wedge/fail-open tests unweakened | 7/10 | `go test -run '(MarkerFlagsTransientOnLivePane\|SilentPaneIsNotTransient\|UnknownDriverFailsOpen)'` |
