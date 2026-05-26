> **Ground-truth capture of real LLM-CLI behavior inside a tmux REPL.** Empirically
> recorded 2026-05-26 on darwin (macOS, Apple Silicon) to anchor the bridge's
> `*-tmux` drivers and the real-tmux integration suite. Re-capture with the harness
> in §Reproduce when CLI versions change. Supersedes assumptions in driver comments
> where they conflict (see §Corrections).

# Real tmux REPL behavior — claude / codex / agy

## TOC
- [Why this exists](#why-this-exists)
- [Captured facts (per CLI)](#captured-facts-per-cli)
- [Corrections to driver assumptions](#corrections-to-driver-assumptions)
- [Bridge contract this validates](#bridge-contract-this-validates)
- [Reproduce](#reproduce)

## Why this exists

The bridge's core value is driving an interactive CLI REPL through tmux:
`new-session → cd → launch → poll capture-pane for the boot marker → load-buffer +
paste-buffer the prompt → poll for the artifact → capture scrollback → exit`. The
boot-marker poll is the load-bearing assumption — if the marker string the driver
greps for never matches the real pane, the launch dies with `EC 80` (REPL-boot
timeout). Markers, alt-screen rendering, and boot timing are **upstream-CLI
behavior that drifts across versions**, so they must be observed, not assumed.

## Captured facts (per CLI)

Capture host: darwin, tmux pane 220×80 (the bridge's `tmuxPaneWidth`×`tmuxPaneHeight`).

| CLI | Version | Launch command | Boot marker (driver greps) | Marker first seen | Visible-pane content | Alt-screen |
|---|---|---|---|---|---|---|
| claude | Claude Code v2.1.150 (Haiku 4.5 · Claude Max) | `claude --model <m> --dangerously-skip-permissions` | `❯` (U+2771) | **t≈1s** | 9 non-blank lines (banner + `❯` prompt + bypass footer) | No |
| codex | OpenAI Codex v0.133.0 (gpt-5.5) | `codex` | `›` (U+203A) | **t≈1s** | 11 non-blank lines (box banner + `› Implement {feature}` + footer) | No (this version) |
| agy | Antigravity CLI 1.0.2 (Gemini 3.5 Flash) | `agy --dangerously-skip-permissions` | `? for shortcuts` | **t≈2s** | 11 non-blank lines (banner + `>` prompt + `? for shortcuts` footer) | No (this version) |

For all three, the marker appeared in **both** the visible capture (`capture-pane -p`)
and the scrollback capture (`capture-pane -p -S -200`) at the same tick — i.e. a
`bootScrollback=0` capture would have sufficed in these versions.

### claude — representative boot pane
```
 ▐▛███▜▌   Claude Code v2.1.150
▝▜█████▛▘  Haiku 4.5 · Claude Max
  ▘▘ ▝▝    ~/ai/claude/evolve-loop-bridge-port
──────────────────────────────────────────────────────────────────
❯
──────────────────────────────────────────────────────────────────
  ⏵⏵ bypass permissions on (shift+tab to cycle) · ← for agents      auto mode unavailable for this model
```
- No trust dialog: `--dangerously-skip-permissions` + an already-trusted dir boots
  straight to `❯`. Footer confirms `bypass permissions on`.

### codex — representative boot pane
```
╭────────────────────────────────────────────────╮
│ >_ OpenAI Codex (v0.133.0)                     │
│ model:     gpt-5.5   /model to change          │
│ directory: ~/ai/claude/evolve-loop-bridge-port │
╰────────────────────────────────────────────────╯
  Tip: … Codex is included in your plan for free …
› Implement {feature}
  gpt-5.5 default · ~/ai/claude/evolve-loop-bridge-port
```
- Launched with NO flags (subscription/ChatGPT auth). `›` is the input prompt.
- No trust dialog observed in this run (dir already trusted under the account).

### agy — representative boot pane
```
      ▄▀▀▄        Antigravity CLI 1.0.2
     ▀▀▀▀▀▀       mickeyyaya@gmail.com (Google AI Pro)
    ▀▀▀▀▀▀▀▀      Gemini 3.5 Flash (Medium)
   ▄▀▀    ▀▀▄     ~/ai/claude/evolve-loop-bridge-port
────────────────────────────────────────────────────────────────
>
────────────────────────────────────────────────────────────────
? for shortcuts                                          Gemini 3.5 Flash (Medium)
```
- A trust/confirmation prompt CAN appear during boot; the bridge's `agy-tmux`
  driver sets `tickDuringBoot=true` so the auto-responder sends `Enter`. Observed
  live in a bridge launch (`[auto-respond] sent keys: Enter`) though not in this
  bare-tmux capture — treat it as **intermittent**, so keep `tickDuringBoot`.

## Corrections to driver assumptions

| Location | Stale claim | Observed reality (2026-05-26) | Action |
|---|---|---|---|
| `driver_codextmux.go` comment | "codex uses alt-screen rendering (boot wait must read scrollback, not the visible pane)" | codex v0.133.0 renders to the NORMAL pane; visible capture is non-blank with `›` present | `bootScrollback=200` kept (defensive superset); comment is version-stale — do NOT rely on "visible is blank" |
| `driver_agytmux.go` comment | "it renders alt-screen (boot wait reads scrollback)" | agy 1.0.2 renders to the normal pane; `? for shortcuts` visible | same — keep scrollback capture defensively, treat alt-screen claim as version-stale |
| `driver_tmux_repl.go` const | `tmuxPromptMarkerDefault = "❯"` (codex ›, agy "? for shortcuts") | All three markers CONFIRMED present | none — markers correct |

The `bootScrollback=200` setting is safe because `capture-pane -S -200` is a superset
of the visible pane. The risk is the inverse direction: if a future CLI version goes
true-alt-screen and `capture-pane -p` returns blank, only the scrollback path saves
the boot. Keeping it is the right defensive default.

## Bridge contract this validates

- Boot markers (`❯` / `›` / `? for shortcuts`) match the real panes → `runTmuxREPL`'s
  boot poll succeeds against all three real CLIs.
- Boot completes well under the 60s `tmuxREPLBootTimeoutS` (1–2s observed); the
  per-CLI `bootIntervalS` (claude 1, codex/agy 2) is comfortably safe.
- `load-buffer -b <session>` / `paste-buffer -b <session> -t <session> -d` (the
  concurrency fix) deliver prompts without cross-session contamination — see
  `TestRealTmux_ConcurrentSessionsIsolated`.

## Reproduce

Harness (drives each CLI exactly as the bridge does; captures visible + scrollback
across the boot window, then kills the session):

```bash
WT=<a trusted working dir>; sess=learn-$$
tmux new-session -d -s "$sess" -x 220 -y 80
tmux send-keys -t "$sess" "cd $WT" Enter; sleep 1
tmux send-keys -t "$sess" "claude --model haiku --dangerously-skip-permissions" Enter
for s in 1 2 3 4 6 8 10 12; do sleep 1
  tmux capture-pane -p -t "$sess"        # visible
  tmux capture-pane -p -S -200 -t "$sess" # scrollback
done
tmux kill-session -t "$sess"
```

Repeat with `codex` and `agy --dangerously-skip-permissions`. Re-run on any CLI
upgrade and update the version + marker columns above.
