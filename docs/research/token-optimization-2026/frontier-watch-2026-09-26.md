# Frontier Watch — 2026-09-26 re-check of the two "track, don't build" families

> **Status:** research re-check, cycle 1704 (inbox `token-frontier-watch`, created 2026-07-06).
> **Scope:** the two technique families the 2026-07-05 survey parked as "track, don't build"
> ([README.md](README.md), the "Track, don't build" line): (a) soft-prompt / KV compression on the
> self-hosted ollama lane, and (b) latent inter-agent channels and compressed agent languages across the
> closed CLI fleet.
> **Method:** fleet telemetry measured from `.evolve/runs/cycle-*/llm-calls.ndjson` (cycles 1600–1705),
> a read of the ollama driver and the control-surface dossier, then web sweeps of the ollama API docs,
> ollama / llama.cpp issues and PRs, arXiv, and vendor API docs. Paper claims were read from abstracts
> and vendor docs. Nothing was reproduced locally. The solution package that weighs the watch strategies
> is `solutions/token-frontier-watch/recommendation.md`.
> **Placement note:** the inbox item names `knowledge-base/research/token-optimization-2026/`. That
> package moved here on 2026-08-05 (#410), and `knowledge-base/` is now a runtime write surface, so the
> dossier lives beside parts 1–5.

## Summary

| Family | Verdict (2026-07-05) | Verdict (2026-09-26) | What changed | What would flip it |
|---|---|---|---|---|
| (a) Soft-prompt / KV compression, self-hosted lane | track | DEFER | Ollama gained an *open* prefill-cache-persistence PR (not merged). The lane carries 0 successful dispatches. | A released ollama KV save/restore **and** at least 5% successful ollama-lane dispatch share |
| (b) Latent inter-agent channels | track | DEFER | Five newer papers, two of them causal audits and two on integrity. Anthropic and OpenAI shipped server-side *compaction*. It is text or opaque, and it stays inside one vendor. | Any fleet CLI accepts compressed or latent context produced by another session or model |

No verdict flipped to BUILD, so this re-check emits no new inbox record (AC2 is satisfied vacuously).
Each section names the observable trigger the next re-check should test.

## Measured baseline — what the self-hosted lane actually carries

Everything the ollama estimates rest on, measured on 2026-09-26 from 1,239 `llm-calls.ndjson` rows in the
103 cycle directories between cycle 1600 and cycle 1705:

- **Dispatch share by CLI:** agy-tmux 441, claude-tmux 434, codex-tmux 363, **ollama-tmux 1**. The ollama
  lane's share is 1 / 1,239 = **0.08%**.
- **The one ollama dispatch failed before launch.** It was the cycle-1687 triage phase at
  2026-09-15T01:44:23Z: `exit_code` 10, `cause_code` bad_flags, `dispatch_source` not_started,
  0 input and 0 output tokens. The lane has carried **0 successful dispatches** in this window.
- **No production routing to ollama.** No `.evolve/profiles/*.json` names ollama. The only repo mentions
  are the driver-agnosticism tests (`go/internal/profiles/driver_agnostic_test.go`).
- **The driver is a REPL, not the HTTP API.** `go/internal/bridge/driver_ollamatmux.go` drives
  `ollama run <model>` through tmux (default tag `llama3.1:8b`). It rejects source-writing phases, so the
  lane is limited to reasoning and review phases. Any API-level cache feature therefore needs a driver
  change before the bridge can use it.
- **Fleet scale for comparison:** 404 of the calls report token usage. Their median input plus cache-read
  is 2,646,268 tokens per call, and the mean is 3,011,496 tokens, which is ≈1.22B tokens in total. The
  median dispatch wall-clock is 129.8 s (n = 1,207).
- **Prompt size of the phase ollama was tried on:** the median `triage-prompt.txt` is 29,856 bytes
  (n = 103). At ≈4 bytes per token that is ≈7,464 tokens.

The lane-share query, run from the project root (the glob also picks up the one `cycle-1623.reset-*`
directory, which is how the window has 103 directories):

```bash
calls() { cat .evolve/runs/cycle-16[0-9][0-9]*/llm-calls.ndjson .evolve/runs/cycle-170[0-5]*/llm-calls.ndjson; }
calls | jq -r '.cli' | sort | uniq -c | sort -rn           # rows per CLI lane
calls | jq -s 'length as $n | [.[] | select(.cli == "ollama-tmux")] | {rows: $n, ollama: length, ollama_ok: [.[] | select(.exit_code == 0 and (.tokens.input + .tokens.output) > 0)] | length}'
calls | jq -c 'select(.cli == "ollama-tmux") | {ts, phase, exit_code, cause_code, dispatch_source}'
```

Cycles 1704 and 1705 were still open when it ran, so a re-run adds their new rows. A second run later on
2026-09-26 returned 1,253 rows: agy-tmux 446, claude-tmux 443, codex-tmux 363, and ollama-tmux still 1
row with 0 successes.

## Family A — soft-prompt / KV compression on the self-hosted ollama lane

**Verdict:** DEFER — the lane carries no traffic, and ollama exposes neither KV import/export nor a
soft-prompt loader. Re-check when the trigger below fires.

### What ollama exposes today (2026-09-26)

- **No KV save/restore/export and no prompt-cache endpoint in the public API.** The `context` array on
  `/api/generate` is documented as *deprecated*: "the context parameter returned from a previous request
  … can be used to keep a short conversational memory"
  (https://github.com/ollama/ollama/blob/main/docs/api.md). It is a token-id list, not KV state. Our
  control-surface dossier lists it as deprecated too (`docs/research/ollama-control-surface-2026.md`,
  §`/api/generate` fields).
- **`keep_alive` / `OLLAMA_KEEP_ALIVE` is the only residency lever.** The default is 5m, `-1` means
  indefinite, and `OLLAMA_KV_CACHE_TYPE` sets `f16`/`q8_0`/`q4_0` KV quantization
  ([ollama-control-surface-2026.md](../ollama-control-surface-2026.md), env-var table). While the model
  stays resident, the bundled llama.cpp runner reuses a matching prompt prefix in memory. `cache_prompt`
  is on by default upstream (https://github.com/ggml-org/llama.cpp/blob/master/tools/server/README.md).
  Whether ollama's runner reuses the prefix across *separate* `ollama run` REPL sessions is assumption A1
  in the solution evidence file, and it is not measured.
- **KV persistence is in flight, not shipped.** Ollama PR #17953 (https://github.com/ollama/ollama/pull/17953),
  opened 2026-08-23 and still open, adds `OLLAMA_PREFILL_CACHE=1`. It saves the prefill cache before a
  runner unload and restores it on reload with the same model and settings. It exposes no public API and
  uses an 8 GiB LRU in daemon-local temp storage. A maintainer wrote that the team "likely won't be able
  to look at this in the near future", because MLX-runner work comes first. Its measurement: with a
  30,643-token prompt on llama3.2:3b and an RTX 4070, cold prefill takes 6,891 ms and verify/restore takes
  765 ms, a 77–89% TTFT cut. A community fork exposes `/api/experimental/kv/save` and `/restore`
  (https://github.com/ai-systems-notes/ollama-prefill-kv-restore). Upstream llama-server already has
  `POST /slots/{id}?action=save|restore` behind `--slot-save-path`, but ollama never passes the flag.
- **Soft-prompt compression needs more than KV persistence.** 500xCompressor (arXiv 2408.03094) and gist
  tokens (Mu et al., NeurIPS 2023) need two things: a compressor trained against the *specific* target
  model, and a server that accepts injected KV values or soft tokens as input. Ollama's API has no
  embedding-input or KV-import path, so the technique cannot run on ollama as shipped. It would need a
  different server (llama-server slots, vLLM, or raw transformers). That is a driver swap, not a spike.
  Its quality ceiling is also low for this pipeline: it retains 62–73% of QA capability at peak
  compression ([part1-context-compression.md](part1-context-compression.md), finding #2). A review or
  audit phase cannot absorb that loss.

### Quantified estimate for the ollama lane (arithmetic shown)

The estimates are derived from the measured baseline above, not asserted. Each unmeasured input is
tagged with its assumption id from `solutions/token-frontier-watch/assumptions-and-evidence.md`: A1
(prefix reuse across REPL sessions), A2 (parameter scaling), A3 (bytes per token) and A6 (interpolation
in prompt length).

1. **Realized win at the measured share: 0 tokens and 0 ms.** The lane completed 0 dispatches in 103
   cycles, so there is no prefill to skip. Any KV technique multiplies against zero.
2. **Counterfactual: route triage to ollama once per cycle.** Triage is the only phase ever attempted on
   the lane.
   - PR #17953 measured five prompt sizes on llama3.2:3b and an RTX 4070 (cold prefill, then the TTFT
     cut after verify, restore and warm prefill): 3,806 tokens 431 ms (77%), 5,181 tokens 604 ms (78%),
     9,932 tokens 1,348 ms (81%), 19,805 tokens 3,472 ms (86%), 30,643 tokens 6,891 ms (89%). The
     per-token cost grows with prompt length, from ≈0.117 ms per token at 5,181 tokens to ≈0.225 ms at
     30,643, so the rate of the largest row does not transfer to a shorter prompt.
   - The ≈7,464-token triage prompt (A3) falls between the 5,181-token and 9,932-token rows, 48% of the
     way from the first to the second. Interpolating linearly between those two rows (A6): cold prefill
     ≈ 604 + 0.48 × (1,348 − 604) ≈ **0.96 s** at 3B, and the TTFT cut ≈ 78 + 0.48 × 3 ≈ **79%**.
   - Scaling linearly with parameters to the driver's default 8B model (×8/3, A2), cold prefill is
     ≈**2.6 s**.
   - KV restore removes ≈79% of that, so it saves ≈**0.76 s** (3B) to ≈**2.0 s** (8B) per dispatch.
     Against the 129.8 s median dispatch wall-clock, that is **0.6%–1.6% of phase latency**.
   - The saving applies only when the model was *unloaded* between dispatches. With `keep_alive: -1` the
     model stays resident, in-memory prefix reuse already covers the case (if A1 holds), and the marginal
     latency win of disk persistence falls to ≈0 s.
3. **Token-denominated effect.** KV reuse cuts no billed tokens: the lane is local, so it costs $0 per
   token. It avoids re-prefilling ≈**7,464 tokens** per dispatch. Over the 103-cycle window that is
   103 × 7,464 ≈ **768,792 prefill tokens** of local compute. The fleet's measured total is ≈1.22B
   tokens, so that is ≈**0.06% of fleet tokens**, and none of those tokens is billed.

**Conclusion:** even the counterfactual win is about two seconds per dispatch on a lane nobody routes
to. A spike now would buy no tokens and would require source changes (driver and server swap) outside
this research task's scope.

### Trigger that flips this to BUILD (the next re-check's test)

Both of these must hold:

- **(T-A1) Serving surface:** an ollama *release* (not only a PR) ships persistent KV/prefill cache or a
  public KV save/restore endpoint. Check: PR #17953 merged, or a release note mentioning
  `OLLAMA_PREFILL_CACHE` or `/api/.../kv`.
- **(T-A2) Traffic:** over any 20 consecutive cycles, ≥5% of `llm-calls.ndjson` rows are *successful*
  `ollama-tmux` dispatches (`exit_code` 0, tokens > 0). The bad_flags launch failure must be fixed first.

If only T-A2 fires, the right first move is the free lever, not a KV build: set `keep_alive: -1` per the
control-surface dossier and measure again.

## Family B — latent inter-agent channels and compressed agent language

**Verdict:** DEFER — the literature moved toward *auditing* latent channels. Every productized vendor
feature is same-vendor, and each one produces a text summary or an opaque blob. No fleet CLI accepts
latent or compressed context from another model.

### Literature re-survey (lineage plus what is newer)

- **Named lineage, still white-box and same-weights:** hidden-state exchange (arXiv 2511.09149), HyLaT
  hybrid latent-text (arXiv 2605.25421), EcoLANG induced compressed languages (arXiv 2505.06904), and
  action-state communication (arXiv 2606.05304). See
  [part2-multiagent-economics.md](part2-multiagent-economics.md), finding #19. Each needs access to the
  hidden states or KV of models the operator runs. None applies to claude, codex or agy.
- **Newer since the 2026-07-05 survey:**
  - *Beyond Tokens*, a unified framework for latent communication (arXiv 2606.05711, 2026-07). It
    classifies the exchanged objects as embeddings, hidden states or KV caches. All of them are white-box.
  - *Do Latent Channels Actually Communicate?* (arXiv 2607.26773, 2026-07-29). A causal audit: on
    GSM8K with Qwen3-4B, a −6.17-point effect survives when the message is swapped for an unrelated one,
    against +5.17 points from example-specific content. The effects *reverse* at 8B. Headline accuracy
    overstates what the channel carries.
  - *When Does Latent Communication Pay?* (arXiv 2608.04893, 2026-08). Relaying KV caches pays only when
    the receiver needs the sender's private information: 100% against 23–25% for answer-irrelevant
    relays. Without that need, results are equivalent within 2.8 points (GSM8K, ARC-Challenge, MedQA).
  - *When Latent Agents Lie* (arXiv 2606.28958) and LCGuard (arXiv 2605.22786). These treat the relayed
    KV as an integrity-critical, uninspectable object that is open to reconstruction and to deception
    attacks.
- **Implication for evolve-loop:** our phase handoffs are *designed* to be inspectable. The auditor
  grades the builder's report and diff (ADR-0099 and the contract gates). A channel the auditor cannot
  read conflicts with the pipeline's integrity floor. The two causal audits also say the gain is
  conditional, and it is often an artifact of cache size rather than content.

### Vendor API check across the fleet's CLIs

- **Anthropic (claude):** server-side compaction replaces older turns with "a summary that Claude writes
  on the server". It is on-demand (beta header `compact-2026-09-04`) or triggered at a token threshold,
  and it works inside one Claude conversation (https://platform.claude.com/docs/en/build-with-claude/compaction).
  It is text, it stays with one vendor, and it is not a latent handoff.
- **OpenAI (codex):** `/responses/compact` returns a compaction item that "is opaque and not intended to
  be human-interpretable". It is documented for OpenAI models only, and the docs say nothing about Codex
  CLI support (https://developers.openai.com/api/docs/guides/compaction). This is the closest thing to a
  productized compressed handoff. It cannot cross vendors, and its opacity fails the auditor-readability
  requirement above.
- **Google (agy / Gemini):** explicit context caching is server-side prefix reuse for the same model
  (https://ai.google.dev/gemini-api/docs/caching). It saves cost on repeated prefixes, and it is not an
  inter-agent channel.
- **Fleet reachability:** all three features are *API* features. The fleet drives the CLIs as tmux REPLs
  (`go/internal/bridge/driver_ollamatmux.go` and its claude, codex and agy peers), so none of them can be
  reached without a driver-model change. Same-vendor compaction is already covered by the CLIs' own
  compaction and by the part-1 context-editing lever. It is not new work for this family.
- **Corpus note:** this check read vendor docs and paper abstracts. It did not exercise the APIs, and it
  did not reproduce any result.

### Trigger that flips this to BUILD (the next re-check's test)

Any one of these:

- **(T-B1)** a fleet CLI (claude, codex or agy) exposes a flag or API that *imports* a compressed or
  latent context produced by another session, model or vendor;
- **(T-B2)** a cross-vendor latent-handoff protocol is adopted by two or more of the fleet's vendors;
- **(T-B3)** two fleet phases run on the *same* self-hosted weights with ≥5% successful dispatch share,
  which is the homogeneous case the literature supports, *and* the latent payload can be logged in an
  auditor-readable form.

## Re-check protocol

- **Cadence:** the next re-check is due around 2026-12 (quarterly), or earlier if T-A1, T-B1 or T-B2 is
  observed in release notes.
- **Cheapest probes:** (1) the state of `gh pr view 17953 -R ollama/ollama`; (2) the successful
  ollama-tmux share over the last 20 cycles (the telemetry query above); (3) a scan of the claude, codex
  and agy CLI changelogs for "import context", "compaction item" or "latent".
- **Solution package:** `solutions/token-frontier-watch/recommendation.md` compares this DEFER-on-triggers
  strategy with a BUILD-now spike and with retiring the watch.
