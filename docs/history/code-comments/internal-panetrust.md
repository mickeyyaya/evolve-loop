# Comment history: `internal/panetrust`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d5

### `go/internal/panetrust/apicover_named_test.go:10` — above `func TestRedactSecrets_NamesExportedWrapper(t *testing.T) {`

```text
// TestRedactSecrets_NamesExportedWrapper exercises the WS3-S1 exported redaction
// entry point (ADR-0052): a secret-shaped token is replaced with the marker and
// clean text is returned unchanged. It names panetrust.RedactSecrets for the
// public-API coverage gate; the redaction policy is proven by the Digest suite,
// so this pins that the wrapper delegates to the same core.
```

### `go/internal/panetrust/extract.go:3` — above `import (`

```text
// extract.go — I5 full (ADR-0045): the typed-extraction half of the trust
// boundary. The privileged path (orchestrator, bridge decision code) never
// branches on raw pane text — only on an Extraction produced here by
// ALLOWLISTED patterns. Anything unextractable is an error: the caller routes
// to the quarantined LLM tail (with Frame), never to ad-hoc string handling.
```

### `go/internal/panetrust/extract_test.go:3` — above `import (`

```text
// ADR-0045 I5 full (§8 RED): typed extraction, secret redaction, untrusted
// framing.
```

### `go/internal/panetrust/panetrust.go:1` — above `package panetrust`

```text
// Package panetrust is the single trust boundary for pane-derived text
// (ADR-0045 I5). Anything captured from a tmux pane is attacker-influenceable
// agent output (OWASP LLM Top-10: segregate untrusted content): a manipulated
// or compromised agent can print text crafted to steer the supervisor — fake
// verdict sentinels, fake channel breadcrumbs, ANSI tricks. Every path that
// carries pane text toward an LLM prompt, a privileged decision, or a
// persisted ledger MUST traverse this package.
//
// Slice 1 (shipped with I1 telemetry) provides Digest: the neutralized,
// length-capped data block used for quarantined LLM consumption and for the
// interaction ledger (threat S10 — a stored-injection vector one hop removed).
// The full typed-extraction surface (Extract, untrusted framing) ships in the
// I5-full slice.
//
// Leaf constraints: imports stdlib only, so bridge, core, and interaction can
// all depend on it without cycles.
```

### `go/internal/panetrust/panetrust.go:44` — above `{'"evolve_channel"', '"evolve_channel' + defangTag + '"'},`

```text
// ADR-0037 channel breadcrumb: `{"evolve_channel":...,"corr_id":...}`.
```

### `go/internal/panetrust/panetrust.go:83` — above `func RedactSecrets(s string) string { return redactSecrets(s) }`

```text
// RedactSecrets is the exported entry point to the secret-redaction pass for
// callers that persist agent-adjacent text WITHOUT the full Digest treatment.
// The advisor (ADR-0052 WS3-S1) captures the RAW routing prompt and response
// for forensics + deterministic replay (WS3-S5): defanging house markers or
// truncating — what Digest does — would corrupt the replay, so capture redacts
// secrets ONLY. It delegates to the same unexported core as Digest, so the
// redaction policy stays single-source (the seam guard in core/seams_test.go
// still pins the private redactSecrets by name).
//
// Consequence of skipping the ANSI strip Digest does: a secret an adversarial
// agent splits with an ANSI escape (sk-<esc>...) could evade the patterns. Held
// acceptable — the prompt is pure-Go-synthesized (no ANSI), and the captured
// response is consumed only as forensics/replay, never re-fed to a privileged
// decision (the floor clamp re-derives the plan from the parsed JSON).
```

### `go/internal/panetrust/panetrust_test.go:3` — above `import (`

```text
// ADR-0045 I5 Digest core (slice 1). Black-box: the assertions use the REAL
// downstream parsers (phasecontract sentinel) where possible, so "neutralized"
// means "the production parser cannot parse it", not "looks different".
```
