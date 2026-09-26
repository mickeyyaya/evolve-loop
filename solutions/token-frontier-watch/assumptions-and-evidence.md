# Assumptions and Evidence — token-frontier-watch

Every number in `options/*.md` and `recommendation.md` cites an entry below. `E<n>` entries are sourced
facts: repo telemetry, repo code and docs, upstream docs and PRs, and papers. Each names its source.
`A<n>` entries are **assumptions**. Each one gives the reasoning behind it and the probe that would
replace it with a measurement. The full narrative and arithmetic are in the dossier,
`docs/research/token-optimization-2026/frontier-watch-2026-09-26.md`.

## Assumptions

- **A1** — While a model stays resident, ollama's runner reuses a matching prompt prefix across separate
  `ollama run` REPL sessions, so `keep_alive: -1` already captures most of the in-memory KV-reuse win.
  *Basis:* upstream llama.cpp has `cache_prompt` on by default (E6). *Not measured on ollama.* Probe:
  two identical triage prompts through the REPL within the keep-alive window, comparing
  `prompt_eval_duration` from `ollama run --verbose`.
- **A2** — Cold-prefill time scales roughly linearly with parameter count, so the 3B prefill time
  interpolated from E5 (A6) scales ×8/3 to the driver's default 8B model (E3). *Basis:* prefill is compute-bound, at about 2 × params
  FLOPs per token. Probe: time one ≈7.5K-token prompt on `llama3.1:8b` on the operator host.
- **A3** — A prompt is ≈4 bytes per token for these English-and-markdown phase prompts. *Basis:* the
  usual tokenizer ratio for English prose. Probe: `ollama run --verbose` prompt-token count on one triage
  prompt.
- **A4** — A quarterly re-check costs about one small document cycle, the size of this one. *Basis:* this
  cycle's triage sized it `small`. Probe: this cycle's `token-usage.json`, once the cycle closes.
- **A5** — Missing an applicability flip by one quarter costs little. Neither family has a
  production-ready path, so the first win after a flip is still a spike and not a deployment. *Basis:*
  E4 and E11–E14. Probe: none needed until a trigger fires.
- **A6** — Between two measured rows of E5, cold prefill and the TTFT cut vary linearly with prompt
  length, so the ≈7,464-token triage prompt (E7) takes the value 48% of the way from the 5,181-token row
  to the 9,932-token row. *Basis:* the two rows bracket the prompt, and E5's per-token cost rises only
  from ≈0.117 to ≈0.136 ms per token across that span, so the curvature inside it is small. Probe: the
  same one-prompt timing as A2, run on `llama3.2:3b`.

## Evidence

- **E1** — Fleet dispatch share, cycles 1600–1705 (1,239 `llm-calls.ndjson` rows, 103 cycle directories,
  measured 2026-09-26): agy-tmux 441, claude-tmux 434, codex-tmux 363, ollama-tmux 1. The ollama share is
  **0.08%**.
- **E2** — The single ollama dispatch was the cycle-1687 triage at 2026-09-15T01:44:23Z. It exited with
  code 10 (`cause_code` bad_flags, `dispatch_source` not_started) and 0 tokens. The lane had **0
  successful dispatches** in the window. No `.evolve/profiles/*.json` routes to ollama.
- **E3** — `go/internal/bridge/driver_ollamatmux.go` drives `ollama run <model>` as a tmux REPL (default
  `llama3.1:8b`) and rejects source-writing phases, so the lane is limited to reasoning and review.
- **E4** — The ollama API exposes no KV save/restore/export and no prompt-cache endpoint. `context` is
  deprecated, and `keep_alive` (default 5m) is the only residency lever
  (https://github.com/ollama/ollama/blob/main/docs/api.md; `docs/research/ollama-control-surface-2026.md`).
- **E5** — Ollama PR #17953 (`OLLAMA_PREFILL_CACHE=1`) was opened 2026-08-23 and is still open. A
  maintainer says it likely won't be reviewed "in the near future". Its measurement on llama3.2:3b and an
  RTX 4070, as cold prefill and TTFT cut per prompt size: 3,806 tokens 431 ms (77%), 5,181 tokens 604 ms
  (78%), 9,932 tokens 1,348 ms (81%), 19,805 tokens 3,472 ms (86%), 30,643 tokens 6,891 ms (89%). The
  TTFT cut includes verify, restore and warm prefill (https://github.com/ollama/ollama/pull/17953).
- **E6** — llama-server already has `cache_prompt` (default on) and the slot save/restore endpoints behind
  `--slot-save-path`. Ollama does not pass that flag
  (https://github.com/ggml-org/llama.cpp/blob/master/tools/server/README.md).
- **E7** — The median `triage-prompt.txt` is 29,856 bytes (n = 103 cycles). At A3's 4 bytes per token
  that is ≈7,464 tokens.
- **E8** — Fleet scale: 404 calls report tokens. The median input plus cache-read is 2,646,268 tokens per
  call and the mean is 3,011,496, which is ≈1.22B tokens in total. The median dispatch wall-clock is
  129.8 s (n = 1,207).
- **E9** — 500xCompressor (arXiv 2408.03094) retains 62–73% of QA capability at peak compression. It
  needs a compressor trained per target model plus KV or soft-token injection
  (`docs/research/token-optimization-2026/part1-context-compression.md`, finding #2).
- **E10** — The latent lineage is hidden-state exchange (arXiv 2511.09149), HyLaT (arXiv 2605.25421),
  EcoLANG (arXiv 2505.06904) and action-state communication (arXiv 2606.05304). All of it is white-box
  (`docs/research/token-optimization-2026/part2-multiagent-economics.md`, finding #19).
- **E11** — A causal audit found that latent-channel gains partly survive an unrelated message (−6.17
  points against +5.17 example-specific on GSM8K with Qwen3-4B), and that they reverse at 8B
  (arXiv 2607.26773).
- **E12** — Relayed KV pays only when the receiver needs the sender's private information (100% against
  23–25%). Without that need, results are equivalent within 2.8 points (arXiv 2608.04893).
- **E13** — Relayed KV is an integrity-critical, uninspectable object (arXiv 2606.28958; LCGuard,
  arXiv 2605.22786).
- **E14** — Vendor features are same-vendor. Anthropic compaction is a server-written text summary (beta
  `compact-2026-09-04`, https://platform.claude.com/docs/en/build-with-claude/compaction). OpenAI
  `/responses/compact` returns an opaque, encrypted item for OpenAI models
  (https://developers.openai.com/api/docs/guides/compaction). Gemini context caching is same-model prefix
  reuse (https://ai.google.dev/gemini-api/docs/caching). None of them accepts another vendor's latent or
  compressed state.

## Derived figures (arithmetic from the entries above)

- The ≈7,464-token triage prompt (E7) lies between E5's 5,181-token row (604 ms, 78%) and its
  9,932-token row (1,348 ms, 81%), at (7,464 − 5,181) ÷ (9,932 − 5,181) ≈ 0.48 of the span (A6).
- Cold prefill at 3B: 604 + 0.48 × 744 ≈ 0.96 s. At 8B: 0.96 × 8/3 ≈ 2.6 s (A2).
- TTFT cut: 78 + 0.48 × 3 ≈ 79%. The restore saving is 79% of cold prefill: ≈0.76 s at 3B and ≈2.0 s at
  8B, which is 0.6%–1.6% of the 129.8 s median dispatch (E5, E8, A2, A6).
- The largest row's rate (6,891 ms ÷ 30,643 tokens ≈ 0.225 ms per token) is not used: E5's per-token cost
  grows with prompt length, so that rate would overstate a 7,464-token prompt's cold prefill about 1.75×,
  and its 89% cut would overstate the saving about 2×.
- Realized saving at the measured share: 0 successful dispatches means 0 tokens and 0 ms (E2).
- Counterfactual prefill avoided over 103 cycles: 103 × 7,464 ≈ 768,792 local tokens ≈ 0.06% of the
  ≈1.22B fleet tokens (E7, E8). None of these tokens is billed.
