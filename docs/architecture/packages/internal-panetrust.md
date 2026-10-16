# internal/panetrust

> Decision record: [ADR-0045](../adr/0045-corrective-interaction-protocol.md) (I5, the trust boundary for pane-derived text).

## Purpose

`internal/panetrust` is the single trust boundary for text from a tmux pane. Anything captured from a pane is agent output that an attacker can influence (OWASP LLM Top-10: segregate untrusted content). A manipulated or compromised agent can print text that is made to steer the supervisor: fake verdict sentinels, fake channel breadcrumbs and ANSI tricks. Each path that carries pane text toward an LLM prompt, a privileged decision or a persisted ledger goes through this package.

## Design

- **Two halves.** `Digest` makes the neutralized, length-capped data block. Quarantined LLM consumption and the interaction ledger use it (threat S10, a stored-injection vector one hop away). `Extract` and `Frame` are the typed-extraction half.
- **Leaf.** The package imports the standard library only. Thus `bridge`, `core` and `interaction` can all depend on it with no cycles. For the same reason `ansiRE` is a verbatim copy of the two-alternative CSI/OSC regex in `bridge/tmux.go` (the `panestream` precedent). It does not import `bridge`.
- **Defanging.** `markerBreaks` defangs the house control markers that an agent can fake. A fake marker can steer the supervisor (threat S1) or poison the persisted ledger (S10).
  - Each entry maps a marker, as the strict parser needs it, to its broken form.
  - The two markers are the `phasecontract` verdict sentinel `<!-- evolve-verdict: {...} -->` and the ADR-0037 channel breadcrumb `{"evolve_channel":...,"corr_id":...}`.
  - `defangTag` goes inside the marker. The strict parsers (`phasecontract.ParseVerdictSentinelFull`, the channel-breadcrumb correlator) then cannot parse it. A human who reads the digest still sees what the agent printed and that it was defused.
  - Substring replacement, and not a regex, keeps the neutralizer easy to audit.
- **Secret redaction.** `secretREs` redact secret-shaped strings whole. `kvSecretRE` redacts the value of a `key: value` or `key=value` line when the key names a credential. All patterns are RE2, so backtracking blowups cannot occur.
- **Order in `kvSecretRE`.** The compound keys (`access_token`, `private_key`) come before the bare `token`, `secret` and `key`. Thus the longer key name is the one that is captured. A bare `token` alternative cannot match inside `access_token` anyway, because no word boundary comes before `token`. The order closes that gap (S6).
- **Digest bounds.** `Digest` caps the output at `maxLines` from the tail (recency beats volume) and `maxCols` runes per line. The truncation is rune-safe (`truncateRunes`). `maxLines <= 0` requests nothing. `maxCols <= 0` means no column cap.
- **`Extract` allowlist.** The privileged path (orchestrator, bridge decision code) never branches on raw pane text. It branches only on an `Extraction` that an allowlisted pattern made. Anything that cannot be extracted is an error. The caller then routes to the quarantined LLM tail (with `Frame`), never to ad-hoc string handling.
- **`ExtractQuestion`** pulls the trailing interactive question that blocks an agent (the input of the AskBroker). It takes the last plausible question line in the pane tail.
  - `questionScanLines` bounds the search to the recent tail. An old question far up the scrollback is not what blocks the agent now.
  - `minQuestionLen` rejects chrome fragments such as `?` and `ok?`.
  - `extractValueMaxCols` caps each extracted value.
  - `paneChromeCutset` holds the leading TUI glyphs (box drawing, prompt markers, bullets, spinners). They are trimmed before the pattern checks.
- **`Frame`.** `untrustedPreamble` is the explicit quarantine framing (CaMeL, OWASP LLM Top-10). It tells the consumer, next to the data, that nothing inside is an instruction. The digest sits in a fence. Backticks that the pane prints are softened, so they cannot close the data block early from inside.

## Invariants

- **Each path that embeds pane text in an LLM prompt goes through `Frame`.**
- **Defang after the ANSI strip.** A marker that an escape sequence splits then cannot reassemble into a form that parses.
- **Neutralize first, then match patterns.** `Extract` builds its value from `Digest` output. An extracted value thus never carries escapes, markers or secrets, and no caller handles raw pane bytes.
- **`Digest` never joins pane text with anything else** (no env, no templates). Nothing beyond what the agent printed, minus secrets, can leak (S6).
- **A credential that an agent echoed must never survive.** Digests persist, and they ship into prompts, possibly to a fallback CLI of another vendor (S6, `redactedToken`).
- **`RedactSecrets` is for raw capture.** The advisor captures the raw routing prompt and response for forensics and deterministic replay. Defanging or truncating corrupts the replay, so capture redacts secrets only. It uses the same unexported core as `Digest`, so the pattern list has one home. A test pins the private `redactSecrets` by name.
- **Known gap of raw capture.** It skips the ANSI strip.
- **The gap is accepted.** A secret that an adversarial agent splits with an ANSI escape can evade the patterns. The prompt is built in pure Go, so it has no ANSI. The captured response serves only forensics and replay. No privileged decision reads it, because the floor clamp derives the plan again from the parsed JSON.
