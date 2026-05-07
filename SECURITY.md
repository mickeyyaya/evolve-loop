# Security Policy

## Supported Versions

evolve-loop's security guarantees apply to the most recent minor version. The pipeline's anti-gaming layer (Tier-1 kernel hooks, ledger SHA verification, sandbox isolation, hash-chained ledger) evolves continuously; security fixes land in the latest minor only.

| Version | Supported |
|---------|-----------|
| 8.x (current) | ✅ Yes |
| 7.x and earlier | ❌ Please upgrade |

## What's In Scope

Reports about the following are welcome:

- **Kernel hook bypasses**: `phase-gate-precondition.sh`, `role-gate.sh`, `ship-gate.sh`, `phase-gate.sh` — any technique that lets an agent or operator skip an enforced check.
- **Ledger integrity**: tampering, forgery, or chain-break techniques that the v8.37+ `verify-ledger-chain.sh` doesn't detect.
- **Sandbox escapes**: techniques that let a profile-restricted subagent write outside its allowlist (Edit/Write or via Bash redirect, interpreter shells, etc.).
- **Cycle binding bypasses**: shipping audit `Verdict: PASS` for a tree state that doesn't match the audited state (HEAD or tree_state_sha).
- **Secret leakage**: scenarios where `state.json`, `ledger.jsonl`, or audit/build artifacts could surface user-environment secrets (API keys, OAuth tokens, etc.) into git, logs, or shipped commits.
- **Forgery of agent identity**: techniques to impersonate another agent role's challenge-token or artifact_sha256.
- **TOFU bypass**: techniques to defeat ship.sh's version-aware self-SHA pin.

## What's Out of Scope

These are NOT vulnerabilities in evolve-loop:

- Issues in third-party dependencies (`claude` CLI, `gemini` CLI, `codex` CLI, `jq`, `git`, `gh`). File those upstream.
- LLM model behavior (sycophancy, hallucination, jailbreaks of the underlying model). evolve-loop's job is to constrain LLM behavior structurally; model behavior itself is Anthropic's / OpenAI's / Google's responsibility.
- Operator misconfigurations (e.g., setting `EVOLVE_BYPASS_PHASE_GATE=1` and then complaining it bypasses).
- Performance / cost issues (please open a regular GitHub issue).

## How to Report

**Preferred**: [Open a Private Vulnerability Report on GitHub](https://github.com/mickeyyaya/evolve-loop/security/advisories/new). This routes the report directly to maintainers via GitHub's built-in security flow.

**Alternative**: email the maintainer at the contact listed in `.claude-plugin/plugin.json:author`. Include:
1. A description of the vulnerability and its impact
2. Reproduction steps (minimal command sequence)
3. The version (`jq -r .version .claude-plugin/plugin.json`) where you observed it
4. Whether you'd like credit in the changelog

**Please do NOT** open a public issue for a security report — that exposes the vulnerability before a fix is available.

## Response SLA

- **Acknowledgment**: within 48 hours
- **Triage + scope confirmation**: within 7 days
- **Fix release**: depends on severity. Critical (anti-gaming bypass, secret leakage) → within 14 days. High → within 30 days. Medium/Low → next minor release.
- **Coordinated disclosure**: 90 days from acknowledgment by default. Earlier if the fix is shipped sooner; later by mutual agreement.

After the fix ships, the report is added to `CHANGELOG.md` under "Security" with credit to the reporter (unless they opt out).

## Hardening Recommendations for Operators

These are best practices for users running evolve-loop in production-adjacent environments:

1. **Never commit `.evolve/state.json` or `.evolve/ledger.jsonl`** — they're gitignored by default. They contain operator metadata and may include sensitive task context.
2. **Run `bash scripts/verify-ledger-chain.sh`** periodically to detect tampering. Add to a scheduled job for long-running deployments.
3. **Set `EVOLVE_STRICT_AUDIT=1`** if you want WARN audits to block ship (default is fluent — WARN ships).
4. **Monitor `.evolve/release-journal/`** for unexpected releases. It's an append-only audit trail of every release-pipeline invocation.
5. **Don't disable kernel hooks.** `EVOLVE_BYPASS_*` env vars exist for emergency operator-driven recovery only. Each bypass logs a WARN.
6. **Keep `claude` (or `gemini` / `codex`) CLI updated.** evolve-loop's sandbox layer assumes current CLI security defaults.

## Cryptographic Trust

evolve-loop v8.37+ uses SHA-256 for the ledger hash chain. The chain is local-only (not Sigstore-signed). For cross-org attestation needs, see the v8.39+ roadmap entry on Sigstore Rekor integration.
