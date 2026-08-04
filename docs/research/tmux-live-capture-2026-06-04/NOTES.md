# Live tmux-LLM capture — protocol learned + architecture conclusion (2026-06-04)

Captured a real `claude --model claude-haiku-4-5 --dangerously-skip-permissions` session inside
tmux 3.6a, exactly as the bridge's tmux driver drives it, with `tmux pipe-pane -o` recording the
raw stream and `tmux capture-pane -p` snapshotting the rendered pane each 2s. Goal: build tests
from the REAL live signal instead of fabricated data.

## What the tmux-LLM session actually sends/receives

1. **Launch echo:** the shell echoes the launch command, then claude initializes the terminal
   (mode-set escapes: `\e[?2004h` bracketed-paste, `\e[?25l/h` cursor hide/show, `\e7/\e8` save/
   restore, scroll-region setup).
2. **Trust modal (boot gate):** in an untrusted cwd claude prompts
   `Quick safety check: Is this a project you trust? 1. Yes / 2. No · Enter to confirm`.
   Boot must dismiss it (`send-keys 1 Enter`). `--dangerously-skip-permissions` does NOT auto-trust
   the folder. (The driver's auto-responder handles this class of prompt.)
3. **REPL ready:** the prompt marker `❯` (U+2771) appears at the bottom in a boxed input region.
4. **Prompt submission:** our text is echoed on the `❯` line.
5. **Answer:** the assistant response is prefixed `⏺` and rendered as readable lines (bullets
   render as `  - ...`). On completion a summary line `✻ Worked for Ns` appears, then the REPL
   returns to an empty `❯` prompt.
6. **Volatile bottom region (always changing):** the input box (`❯ ...`), the footer
   (`⏵⏵ bypass permissions on (shift+tab to cycle) · esc to interrupt`), and the thinking spinner.
   `esc to interrupt` is present while the turn runs; it disappears at idle. This region is
   re-painted every tick and is NOT content.

## The decisive finding: raw `pipe-pane` ≠ rendered `capture-pane`

The same answer line, two ways:

- **Rendered (`capture-pane -p`):** `⏺ - Terminal multiplexer: tmux lets you create and manage …`
  — clean, linear, readable.
- **Raw (`pipe-pane`):** painted with **2D cursor motion** — `\e[2C \e[3A \e[2D \e[3B` (cursor
  forward/up/back/down), **105** absolute column moves (`\e[<n>G`) in the answer region alone,
  CRs, and even non-UTF8 bytes. Claude positions text by moving the cursor, not by emitting
  spaces+newlines.

**Conclusion:** the raw `pipe-pane` byte stream can only be linearized by a full terminal
emulator (relative+absolute cursor motion, scroll regions, CR overwrite). A `stripANSI` +
CR-collapse pre-pass (the originally-planned filter) would DELETE the positioning and jam words
together (`Accessing\e[12Gworkspace:` → `Accessingworkspace:`) — garbage. **tmux `capture-pane`
already does the emulation/rendering for us.** Therefore the correct live content source is
**polling `capture-pane` and emitting newly-stabilized rendered lines**, not `pipe-pane`.

This overturns the `pipe-pane`-based primitive in the approved fix plan. Net effect: SIMPLER —
no terminal emulator, no CR/ANSI raw filter; reuse tmux's rendering + the existing plaintext
noise classifier (`phasestream` `isBorderRune`/`isSpinnerRune`/dedup) for residual box/spinner
chrome.

## Revised architecture (capture-pane-poll-delta)

- The driver poll loop ALREADY `capture-pane`s each ~2s tick (completion/idle/auto-respond).
  Add: each tick, emit NEW stable content to `<agent>-pane.live` — the lines ABOVE the last `❯`
  prompt-marker line that weren't emitted before (the at/below-prompt region is volatile →
  excluded, which drops the spinner/footer/input for free). `stripANSI` the capture (tmux.go has
  it). Track a cursor over the stable above-prompt history (large `-S` keeps history; the
  at-exit 10k dump backstops).
- Breadcrumbs → a live `<agent>-breadcrumbs.live` file (unchanged from the fix plan; the driver
  generates these exactly).
- Producer/Normalizer read `<agent>-pane.live` (plaintext; existing classifier handles residual
  noise) + `<agent>-breadcrumbs.live` (correlation). No raw-pane CR/ANSI mode needed.
- `pipe-pane`/`PipePane` (fix-task FT1) is NOT on this path — revert or leave as unused.

## Fixtures saved here (PII-redacted)

- `rendered-final.txt` — full rendered pane incl. the `⏺` answer + `✻ Worked for 3s` + footer.
- `rendered-snap-thinking.txt` — mid-turn snapshot with the volatile `esc to interrupt` footer.
- `raw-pipe-pane-answer.catv.txt` — `cat -v` of the raw stream's answer region (the 2D-motion
  evidence). Not a parse target — kept to justify the pivot and to test that raw is rejected.

These drive the re-planned filter + e2e: feed `rendered-*` snapshots through the capture-pane
delta extractor → assert the `⏺` answer lines are emitted and the spinner/footer/`❯` region is
not.
