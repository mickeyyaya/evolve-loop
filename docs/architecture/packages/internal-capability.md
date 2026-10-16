# internal/capability

## Purpose

`internal/capability` parses the adapter capability manifests (`adapters/<cli>.capabilities.json`). It produces two things. The first is the WARN lines and the dispatch-plan JSON envelope that `cmd_validate_profile` of the legacy `subagent-run.sh` used to print. The second is the aggregate quality tier of an adapter, a port of `_capability-check.sh`. A capability that the adapter lacks natively triggers a WARN with a substitute (for example a wall-clock timeout in place of a native budget cap). The runner shows these to operators before the agent starts.

## Design

- **`Manifest` types only what is inspected.** It holds the `supports` booleans. Other keys are tolerated and ignored. Both booleans default to true when the field is absent or null, as the bash `jq` expression `if . == null then "true" else tostring` did.
- **A missing manifest is not an error.** `Inspect` returns both supports as true and no warns, because the manifest is optional for an adapter that has not declared one. An I/O error other than not-exist is returned.
- **The scanners are string based.** `extractObject` and `extractBool` find the `supports` block first, so a stray match outside it cannot change a result. A field that is absent, null or not a literal bool gives the default. The package stays free of `encoding/json` on this path by decision.
- **`PlanJSON` is byte-compatible with the bash `printf`** at `subagent-run.sh:555`: the same field order and the same boolean text. The caller writes the line to `EVOLVE_DISPATCH_PLAN_LOG`. `jsonEscape` escapes only `"`, as the bash `sed` did for the warn entries. The top-level fields are not escaped, so a value that holds `"` breaks the line in bash too. The Go code keeps this on purpose, for parity. Each warn entry loses the `[adapter-cap] WARN ` prefix and keeps `cli=... missing=... substitute=...`.
- **`Probe` is the seam for the host checks.** It replaces `command -v` and `uname` of `_capability-check.sh`. `DefaultProbe` checks binary presence for `claude` and `agy`, and OS-gated presence for the sandbox probes. An unknown probe name is false.
- **`QualityTier` is the lowest mode over all capabilities,** ranked `none < degraded < hybrid < full`. A capability is a fixed mode string, or an object with a default and probes. For an object, the probes that apply are tried in manifest order. An empty `applies_to` matches all. The first probe that holds upgrades the mode to `if_true_mode`, and the scan stops.
- **The edge cases of the tier.** An empty capabilities map gives `full`, as the bash start value `LOW_RANK=3` does. An unknown mode label ranks as `none`.
- **A missing or malformed manifest gives `("unknown", err)`.** The bash caller (`consensus-dispatch`) treats a non-zero result as tier `unknown`. A `require_min_tier` filter of `hybrid` or higher then excludes that voter.

## Invariants

- **A false probe never applies `if_false_mode`.** This is the parity quirk. The live `_capability-check.sh` reads the probe with jq `.[k] // "unknown"`, and the jq `//` operator treats false like null. The result matches neither branch, so `if_false_mode` is dead code in the shell and the capability keeps its default. The Go code does the same, and it parses `ifFalseMode` so that the divergence stays visible (`TestResolveMode_FalseProbeLeavesDefault`). A test that reports `degraded` here means the port "fixed" the quirk, and that is a behaviour change, not a migration.
- The golden tiers over the real manifests match the bash output (`TestQualityTier_GoldenParityWithBashManifests`).

## Findings

- The shell quirk makes `gemini`, `codex` and `agy` resolve to `none` instead of `degraded` when the binary is off PATH. The intended meaning must be fixed once, here, and not in the shell that is being deleted. This is an open follow-up and it is not fixed.
