# Platform compatibility — current Go runtime

The runtime entrypoint is `evolve`, built from `go/cmd/evolve`. Phase ordering, routing policy, evidence verification, and shipping are native Go. The deleted shell adapter tree is not an extension point or fallback.

## CLI support

Claude, Codex, Antigravity (`agy`), and Ollama have bridge implementations. A Gemini model through agy is distinct from a standalone Gemini CLI identity. Profiles select abstract model tiers and supported CLI transports; the bridge realizes launch arguments against each transport's capabilities.

Use `evolve bridge probe` and `evolve doctor` to inspect the installed environment. A detected executable does not prove that authentication, transport, confinement, or every model capability works. Live verification must use the intended profile and transport.

## Guarantees and limits

- Host phase and shipping checks operate independently of an agent's narrative.
- CLI tool permissions and hooks vary by transport. They are not interchangeable with OS confinement.
- OS confinement must be measured and applied. macOS sandbox-exec and Linux bubblewrap do not expose identical policies; unsupported required restrictions are explicit failures.
- Nested-session markers alone are not evidence of an effective outer sandbox. Explicit operator opt-out is reported as degraded.
- Cloud transports require network access. The filesystem sandbox does not provide per-provider network allowlisting.
- Model-family separation is a preference. Do not claim a hard pairwise family gate or restore the superseded five-layer routing proposal.

Current implementation lives in `go/internal/bridge`, `go/internal/adapters/sandbox`, `go/internal/looppreflight`, and `go/internal/setup`. Configuration is compiled defaults plus `.evolve/policy.json` and phase profiles, with documented compatibility overrides.

See [runtime contract](current-runtime-contract.md), [isolation policy](recovery-isolation-policy.md), and [the historical adapter protocol](../private/research/archived-2026-09-09/architecture-platform-compatibility.md).
