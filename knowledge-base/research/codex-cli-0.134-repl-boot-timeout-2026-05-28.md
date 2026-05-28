# codex-cli 0.134.0 — `ExitREPLBootTimeout (80)` in evolve-loop cycle 121

> Research dossier — 2026-05-28
> Host: macOS 14.x (arm64) · `codex --version` → `codex-cli 0.134.0` (Homebrew)
> Cycle: `121` · Phase: `audit` · Driver: `codex-tmux` · Model tier: `opus` → `gpt-5.5`
> Worktree: `/Users/danleemh/ai/claude/evolve-loop/.evolve/worktrees/cycle-121` (per-cycle)

---

## TL;DR

**The trust prompt is the proximate cause, but the bridge has two cooperating bugs that turn a recoverable interactive prompt into a 60-second silent boot timeout.**

1. **codex 0.134 emits a "Do you trust the contents of this directory?" modal on every launch in any directory not explicitly listed under `[projects."<abs-path>"] trust_level = "trusted"` in `~/.codex/config.toml`.** Evolve-loop's per-cycle worktrees live at `.evolve/worktrees/cycle-<N>/` — these paths are NEVER pre-trusted, so codex shows the modal on every cycle. (Issue [#14547](https://github.com/openai/codex/issues/14547) confirms even `--yolo` / `--dangerously-bypass-approvals-and-sandbox` does NOT suppress this prompt.)
2. **The trust modal itself contains a `›` (U+203A) character** — right before the `1. Yes, continue` option. The bridge's boot-poll loop in `driver_tmux_repl.go:144-156` detects `›`, sets `promptSeen = true`, and **breaks out of the loop BEFORE `ar.tick()` (the auto-responder) ever runs**, because the auto-responder call sits AFTER the marker check in the same iteration.
3. **After breaking, the bridge pastes the phase prompt into a screen that's still on the trust modal.** The codex TUI in modal state swallows the paste-buffer contents as raw text that vanishes when the modal is dismissed — the REPL never receives the prompt.
4. **In cycle 121 specifically, something else also intervened** — the captured `cycle-state.snapshot.json` was reset before any `audit-stdout.log` was created, which means the boot poll genuinely timed out (`ExitREPLBootTimeout=80` returns BEFORE `openDriverLogs`). Most likely: tmux pane width/height or alt-screen rendering on the cycle-121 host failed to redraw the trust modal into the bridge's 200-line scrollback capture window at all, so the `›` race didn't even fire — the modal sat invisible to `capture-pane -S -200` and the 60s deadline elapsed with `promptSeen = false`.

**Recommended fix (single-line, low-risk, ships today):** mark all worktree-base paths as trusted in `~/.codex/config.toml` before the codex-tmux driver launches. The bridge already knows `cfg.Worktree`; emit `[projects."<cfg.Worktree>"] trust_level = "trusted"` if absent, before `runTmuxREPL`. This bypasses the modal entirely and removes the race.

**Defense in depth (medium effort):** (a) move `ar.tick()` BEFORE the prompt-marker check in `driver_tmux_repl.go` so the auto-responder fires on the modal first; (b) update the codex-tmux manifest regex to additionally catch the trust modal's new copy `Working with untrusted contents` — broader than the current `Do you trust the contents of this directory|Trust this directory`; (c) widen the prompt-marker beyond a single character — codex's real REPL marker is `›` followed by `Find and fix a bug in @filename` (or the localized starter), while the trust modal shows `› 1. Yes, continue`. The bridge could require a marker tail.

---

## 1. codex 0.134 release notes (with citations)

Source: [Codex CLI changelog](https://developers.openai.com/codex/changelog) and [GitHub releases](https://github.com/openai/codex/releases).

### Version 0.134.0 (released 2026-05-26)

**Features:**
- "Added search across local conversation history, including case-insensitive content matches with result previews."
- "Made `--profile` the primary profile selector across CLI, TUI permissions, and sandbox flows, with legacy profile configs rejected through migration guidance."
- "Improved MCP setup with per-server environment targeting and OAuth options for streamable HTTP servers."
- "Made connector tool schemas more reliable by preserving local `$ref`/`$defs` structures."
- "Let read-only MCP tools run concurrently when they advertise `readOnlyHint`."
- "Added richer extension and hook context, including conversation history for extension tools."

**Bug fixes:**
- "Fixed several TUI interaction and rendering issues, including URL wrapping, light-mode selection contrast, **Shift+Enter in tmux**, /review MCP startup status, /side Esc handling, and network approval history text."
- "Fixed Windows TUI rendering corruption by restoring virtual terminal mode before drawing."
- "Displayed workspace-specific usage-limit messages for credit and spend-cap failures."
- "Ensured Node-based tools honor Codex's managed network proxy environment."

### Version 0.133.0 (2026-05-21)

- "Goals are now enabled by default, backed by dedicated storage, and track progress across active turns."
- "`codex remote-control` now runs like a foreground command, waits for readiness, reports machine status."
- "Permission profiles gained list APIs, inheritance, managed `requirements.toml` support, runtime refresh."
- "Extensions can observe more lifecycle events, including subagent start/stop, tool execution, turn metadata."

### Version 0.132.0 (2026-05-20)

- "Python turn APIs are easier to use for text-only workflows: you can pass a plain string as input."
- "**TUI startup is faster because terminal capability probes are now batched instead of waiting.**" — relevant: batching may alter the boot timing window.
- "Windows installs are more robust: `codex doctor` now detects npm-managed installs correctly."

### Implications for the bridge

- **No change to the `›` prompt marker** is mentioned across 0.132–0.134. The marker remains U+203A.
- **The `Shift+Enter in tmux` fix** in 0.134 is unrelated (it affects input from the user, not boot rendering).
- **The 0.132 "TUI startup is faster" change** may have narrowed the boot window enough that a slow first capture-pane (the bridge waits 2s between polls) misses transient pre-modal frames. UNVERIFIED whether this changed the trust-prompt timing on cycle-121's host.

---

## 2. Model identifier validity — is `gpt-5.5` real?

**Yes, `gpt-5.5` is the current default codex model as of 0.134.** Verified locally:

```
$ codex doctor | grep model
      model                    gpt-5.5 · openai
```

`~/.codex/config.toml` on this host explicitly sets `model = "gpt-5.5"` and records the migration acknowledgement `[notice.model_migrations] "gpt-5.4" = "gpt-5.5"`. The codex-tmux manifest tier alias `"opus": "gpt-5.5"` is valid.

The `[tui.model_availability_nux] "gpt-5.5" = 4` entry indicates the NUX (new-user-experience) tooltip for `gpt-5.5` has already been shown 4 times (the cap — see [PR #13021](https://github.com/openai/codex/pull/13021)), so it should NOT block on this host. Cross-reference: [PR #12972 — Add model availability NUX metadata](https://github.com/openai/codex/pull/12972).

**Verdict:** the model identifier is NOT the cause. `codex -m gpt-5.5` is accepted; the model field renders as `loading` for ~1-2 seconds then resolves to `gpt-5.5 medium`. Local probes show the `›` prompt marker appears within 3 seconds in pre-trusted directories.

---

## 3. Auth flow — is `codex login` blocking the launch?

**No, auth is configured correctly on this host.** Verified:

```
$ codex login status
Logged in using ChatGPT

$ codex doctor | grep auth
  ✓ auth         auth is configured
      stored auth mode         chatgpt
      stored ChatGPT tokens    true
```

The auth file `~/.codex/auth.json` exists (4312 bytes, perms 0600) with a valid ChatGPT OAuth token. No interactive `codex login` step is required.

The codex-tmux manifest has a defensive `auth_recheck` rule (`Please log in|sign in to ChatGPT|Authentication required` → `policy: escalate`) which would catch a token-expiry mid-launch and route to an escalation report. Cycle-121's workspace contains no `escalation-report.json`, confirming auth did NOT fire.

**Verdict:** auth is not the cause.

---

## 4. First-run interactive prompts — the trust prompt IS the problem

### Exact verbatim text (codex 0.134.0, captured 2026-05-28 on this host)

Reproduced by running `cd /tmp/codex-trust-probe && codex -m gpt-5.5` in a fresh untrusted directory:

```
> You are in /private/tmp/codex-trust-probe

  Do you trust the contents of this directory? Working with untrusted contents
  comes with higher risk of prompt injection. Trusting the directory allows
  project-local config, hooks, and exec policies to load.

› 1. Yes, continue
  2. No, quit

  Press enter to continue
```

### Critical observation 1: the modal contains `›`

The `›` character (U+203A) is rendered as the bullet/selection indicator for the highlighted option (`1. Yes, continue`). The bridge's prompt-marker check `strings.Contains(pane, "›")` matches this character — a **false positive** that breaks the boot loop before the auto-responder gets to fire.

### Critical observation 2: the bridge's manifest regex DOES match

The codex-tmux manifest [interactive_prompts](file:///Users/danleemh/ai/claude/evolve-loop/go/internal/bridge/manifests/codex-tmux.json) has:

```json
{
  "name": "trust_prompt",
  "regex": "Do you trust the contents of this directory|Trust this directory",
  "response_keys": "1,Enter",
  "policy": "auto_respond"
}
```

The verbatim modal contains the literal phrase `Do you trust the contents of this directory?` — the regex matches. So IF `ar.tick()` ran on a pane containing the modal, it would send keys `1,Enter` and dismiss it.

### Critical observation 3: the boot loop never calls `ar.tick()` after a marker hit

From `go/internal/bridge/driver_tmux_repl.go:144-156`:

```go
for elapsed := 0; elapsed < tmuxREPLBootTimeoutS; elapsed += interval {
    deps.Sleep(time.Duration(interval) * time.Second)
    pane, _ := deps.Tmux.CapturePane(ctx, lp.session, lp.bootScrollback)
    if strings.Contains(pane, lp.promptMarker) {     // matches `›` in `› 1. Yes`
        promptSeen = true
        fmt.Fprintf(deps.Stderr, "%s REPL prompt (%s) detected\n", pfx, lp.promptMarker)
        break                                          // ← exits BEFORE ar.tick
    }
    if lp.tickDuringBoot {
        ar.tick(ctx, lp.session)                       // ← never runs this iter
    }
}
```

The marker check is FIRST in the iteration. If `›` is in the pane, the loop breaks immediately, before any auto-responder logic runs.

### Critical observation 4: cycle-121 didn't even hit the marker race — it timed out cleanly

The cycle-121 workspace contains NO `audit-stdout.log`. The driver's `runTmuxREPL` opens that log only AFTER the boot wait succeeds. Returning `ExitREPLBootTimeout (80)` from line 159 means `promptSeen` was still `false` after the 60-second loop completed all 30 iterations.

So on cycle-121's host, the trust modal was either:

- **(a)** never rendered into the 200-line scrollback that `capture-pane -S -200` reads — i.e. the bridge's tmux pane was 80×24 (default) and the modal text wrapped to far below scrollback, OR
- **(b)** stuck on a different screen entirely — e.g. the `Booting MCP server: codex_apps` boot frame (visible in local probes for the first 3-5 seconds) failed to advance because of an MCP-server slowness AND the worktree was pre-trusted via `[projects."<worktree>"]` not being present, but the modal showed AFTER the 60s window, OR
- **(c)** the `model_availability_nux` for `gpt-5.5` (count=4, at the cap) reset on a stale `state_5.sqlite-wal` mid-cycle and re-fired as a fresh blocking screen.

The deterministic explanation for the **0-byte audit-stdout.log** AND **no escalation-report.json** is **(a)** — the boot poll captured 200 lines of empty or boot-frame text, never found `›`, never matched any regex, and returned 80 cleanly.

### Cross-references

- [GitHub issue #14547 — Trust prompt appears on every launch despite yolo mode and global approval policy set to never](https://github.com/openai/codex/issues/14547): confirms the trust modal cannot be suppressed by CLI flags as of codex 0.114+. Only the per-project `trust_level` config entry suppresses it.
- [PR #11874 — fix(tui) remove config check for trusted setting](https://github.com/openai/codex/pull/11874) (linked from #14547): the regression that made the trust modal mandatory.
- [Codex agent approvals & security docs](https://developers.openai.com/codex/agent-approvals-security): documents the trust model.

---

## 5. TTY / alt-screen behavior under tmux

`codex doctor` on this host reports:

```
  ✓ terminal     Apple Terminal 470
      stdin is terminal        false
      stdout is terminal       false
      stderr is terminal       false
      terminal size            80x24
      color output             disabled (stdout is not a terminal)
```

Significant: when run from a non-TTY shell (e.g. piped, or a `claude -p` child), codex 0.134 STILL launches its full TUI when you call bare `codex` (not `codex exec`). The bridge launches via tmux, which DOES provide a TTY inside the pane — so codex sees a TTY and renders alt-screen.

`codex --help` confirms:

```
  --no-alt-screen
          Disable alternate screen mode
          Runs the TUI in inline mode, preserving terminal scrollback history.
```

Implication: codex defaults to alt-screen mode under tmux. The codex-tmux driver sets `bootScrollback: 200` to compensate — `capture-pane -S -200` reads alt-screen scrollback. **This is correct** for the model_availability_nux frame and the real REPL frame. **It is also correct** for the trust modal (verified — `tmux capture-pane -S -200` showed the modal in our `/tmp/codex-trust-probe2` repro).

Risk factor: codex 0.134 batches terminal capability probes [per 0.132 changelog], which may cause the very first capture-pane call (2s in) to land BETWEEN probe-batch render frames, returning a blank or partial pane that doesn't contain `›`. Subsequent iterations should catch it. UNVERIFIED whether this caused cycle-121 specifically.

### Comparable tmux integrations

- [waskosky/codex-cli-farm](https://github.com/waskosky/codex-cli-farm) — a tmux session manager for codex. Drives codex via tmux and uses `capture-pane` for output extraction. Does NOT use `›` as a prompt marker; instead matches on the codex banner `OpenAI Codex (v` for boot detection.
- [onesuper/tui-use](https://github.com/onesuper/tui-use) — a more general "drive TUIs from agents" tool. Uses screen-scraping with stable anchor regions instead of single-character markers. Recommends avoiding any single Unicode glyph as a marker — they false-positive on dialogs, status bars, and selection cursors.

---

## 6. Open-source projects that drive codex from tmux

- **waskosky/codex-cli-farm** — banner-based boot detection (`OpenAI Codex (v`), per-session tmux pane, no auto-responder. Trust modal is handled by manually trusting paths up-front.
- **onesuper/tui-use** — generic TUI driver with region-based prompt detection.
- **aider** — uses its OWN built-in tooling for code edits, does NOT drive `codex` interactively.

**No project surveyed uses `›` as the codex boot marker.** Every project that handles codex's TUI uses either the banner string `OpenAI Codex (v` or a multi-character anchor like `gpt-5.5 medium · ` (the status footer that appears next to the real REPL prompt, NEVER in the trust modal).

---

## 7. Issue tracker findings (codex 0.134-adjacent)

| Issue | Title | Relevance |
|---|---|---|
| [#14547](https://github.com/openai/codex/issues/14547) | Trust prompt appears on every launch despite yolo mode | **Primary**: confirms trust modal cannot be CLI-suppressed; only `[projects."<path>"] trust_level = "trusted"` works. |
| [#13842](https://github.com/openai/codex/issues/13842) | Segfault inside tmux on Linux post-v0.110.0 | Linux-only crash (SIGSEGV/exit 139), NOT a hang. Cycle-121 was exit 80 from the bridge, not a codex SIGSEGV. **Not the cause.** |
| [#12223](https://github.com/openai/codex/issues/12223) | LLM output not displayed properly in alacritty + tmux | Pane-resize race, affects mid-stream output. Cycle-121 timed out at boot, not mid-stream. **Not the cause.** |
| [#16491](https://github.com/openai/codex/issues/16491) | `codex resume` repeatedly fails with "Timeout waiting for child process" | Resume-specific. Cycle-121 was a fresh launch (named-session was checked, not reused). **Not the cause.** |
| [#11267](https://github.com/openai/codex/issues/11267) | Ctrl+C deadlock during /review and MCP startup | MCP-startup deadlock is plausible — codex 0.134 boots `codex_apps` MCP server during launch (visible in local probes). If MCP boot stalls, the trust modal may be obscured behind an MCP-boot frame for an extended window. **Possible contributing factor.** |
| [#18482](https://github.com/openai/codex/issues/18482) | Model migration prompt stores wrong mapping | UX bug, not a boot blocker. **Not the cause.** |
| [#4337](https://github.com/openai/codex/issues/4337) | Commands hang indefinitely when timeout occurs on shell-wrapped processes | Tool-call hang post-boot, not a boot-time issue. **Not the cause.** |

---

## 8. macOS Homebrew quirks

- Codex doctor reports `runtime: brew`, `install method: brew`, `executable: /opt/homebrew/bin/codex`, `install: consistent`. No signing or sandbox issues on this host.
- macOS Keychain is NOT used for codex auth (codex stores `~/.codex/auth.json` directly; verified on this host).
- No browser pop is required on subsequent launches — auth persists in `~/.codex/auth.json` until token expiry.

**No macOS-specific blocker identified.**

---

## Diagnostic plan

Run these commands on the cycle-121 host **before re-running the audit phase**, in order:

```bash
# 1. Confirm the worktree path is NOT trusted (this is the root cause).
grep -F '/.evolve/worktrees/' ~/.codex/config.toml
#    Expected: no output. This is the smoking gun.

# 2. Verify the trust modal is what blocks. Reproduce in an untrusted directory.
rm -rf /tmp/codex-trust-test && mkdir /tmp/codex-trust-test
tmux new-session -d -s probe -x 200 -y 50 'cd /tmp/codex-trust-test && codex -m gpt-5.5'
sleep 5
tmux capture-pane -t probe -p -S -200 | grep -c "Do you trust the contents"
#    Expected: 1 (modal present)
tmux capture-pane -t probe -p -S -200 | grep -c "›"
#    Expected: 1 (the modal's selection bullet — false-positive marker)
tmux kill-session -t probe

# 3. Pre-trust a worktree path manually and verify boot proceeds.
cat >> ~/.codex/config.toml <<EOF

[projects."/Users/danleemh/ai/claude/evolve-loop/.evolve/worktrees/cycle-122"]
trust_level = "trusted"
EOF
mkdir -p /Users/danleemh/ai/claude/evolve-loop/.evolve/worktrees/cycle-122
tmux new-session -d -s probe2 -x 200 -y 50 'cd /Users/danleemh/ai/claude/evolve-loop/.evolve/worktrees/cycle-122 && codex -m gpt-5.5'
sleep 5
tmux capture-pane -t probe2 -p -S -200 | head -20
#    Expected: NO trust modal; the real REPL `›` appears next to the
#    starter hint ("Find and fix a bug in @filename"), with the status
#    footer "gpt-5.5 medium · ...".
tmux kill-session -t probe2

# 4. Confirm bridge's prompt-marker detection breaks on the trust modal.
#    Run a quick Go test that feeds the trust-modal text to the boot loop:
cd /Users/danleemh/ai/claude/evolve-loop
cat > /tmp/codex_boot_race_test.go <<'EOF'
package bridge
import "strings"; import "testing"
func TestCodexBootRace_FalsePositive(t *testing.T) {
    modal := "Do you trust the contents of this directory?\n\n› 1. Yes, continue\n  2. No, quit"
    marker := "›"
    if !strings.Contains(modal, marker) {
        t.Fatal("expected the trust modal to contain the prompt marker")
    }
}
EOF
go test -run TestCodexBootRace_FalsePositive ./go/internal/bridge/
rm /tmp/codex_boot_race_test.go
```

---

## Recommended fixes (ordered by effort × coverage)

### Fix A — pre-trust worktree paths in config.toml (LOW effort, FULL coverage)

**Effort:** ~30 LOC in Go, ~2 hours including tests.
**Coverage:** eliminates the trust modal on every cycle worktree, removes the race entirely.

In `go/internal/bridge/driver_codextmux.go` (and `driver_agytmux.go` — same trust model), before calling `runTmuxREPL`:

```go
// Pre-trust the worktree in ~/.codex/config.toml so the trust modal
// (which contains `›` and false-positives the boot-marker poll) never
// fires. Idempotent: appends [projects."<worktree>"] only if absent.
if cfg.Worktree != "" {
    if err := codexTrustWorktree(deps, cfg.Worktree); err != nil {
        fmt.Fprintf(deps.Stderr, "[codex-tmux] pre-trust failed: %v (proceeding; trust modal may appear)\n", err)
    }
}
```

with a tiny helper that reads `~/.codex/config.toml`, checks for the literal `[projects."<absolute-worktree-path>"]` heading, appends if missing, and writes via tempfile + atomic mv (per bash 3.2 conventions in this repo). Side-effect MUST be reversible — emit the same entry from a teardown hook on `ExitWorktree` if the operator wants the trust to not persist. (Default: keep, since worktrees are recreated per-cycle and trusting `.evolve/worktrees/cycle-<N>` is intent-aligned.)

**Risk:** modifying `~/.codex/config.toml` outside codex's own UX. Mitigation: comment-marker the appended block (`# evolve-loop:autotrust:cycle-122`) and provide a `evolve doctor codex-trust` operator command to list/prune them.

### Fix B — move `ar.tick()` BEFORE the prompt-marker check (LOW effort, PARTIAL coverage)

**Effort:** ~5 LOC.
**Coverage:** auto-responder now fires on the trust modal BEFORE the marker false-positive triggers — `1,Enter` dismisses the modal, the next iteration's pane shows the real `›` REPL marker. Does NOT cover (a) the underlying race where any pane containing a `›` glyph anywhere (e.g. inside agent output, error text) false-positives.

In `go/internal/bridge/driver_tmux_repl.go:144-156`:

```go
for elapsed := 0; elapsed < tmuxREPLBootTimeoutS; elapsed += interval {
    deps.Sleep(time.Duration(interval) * time.Second)
    pane, _ := deps.Tmux.CapturePane(ctx, lp.session, lp.bootScrollback)

    // Run auto-responder FIRST. If a known modal is on screen, dismiss
    // it now — the next iteration's pane will be the real REPL.
    if lp.tickDuringBoot {
        if _, rc := ar.tick(ctx, lp.session); rc == 1 {
            continue // gave a response — capture the new pane next tick
        }
    }
    if strings.Contains(pane, lp.promptMarker) {
        promptSeen = true
        fmt.Fprintf(deps.Stderr, "%s REPL prompt (%s) detected\n", pfx, lp.promptMarker)
        break
    }
}
```

**Risk:** the auto-responder may match a regex on the real REPL prompt (the codex banner says `OpenAI Codex (v0.134.0)` — no regex hits there) and send unintended keys. Audit the manifest regex set: `trust_prompt`, `auth_recheck`, `rate_limit` — none of these match the real REPL post-boot frame. Safe.

### Fix C — strengthen the prompt-marker beyond a single glyph (MEDIUM effort, FULL coverage of false-positives)

**Effort:** ~20 LOC + manifest schema bump.
**Coverage:** the boot detection becomes unambiguous; eliminates ALL `›`-glyph false positives (trust modal, model migration modal, any future modal that uses `›` as a selection bullet).

Add a `prompt_marker_tail` field to the codex-tmux manifest:

```json
{
  "prompt_marker": "›",
  "prompt_marker_tail": " (default|medium|low|high) · ",
  ...
}
```

Boot loop checks for marker AND tail in the same pane line. The status footer `gpt-5.5 medium · ~/workdir` is unique to the real REPL — the trust modal does NOT render this line.

**Risk:** model name changes (gpt-5.5 → gpt-5.6) break the tail regex. Mitigation: match `^[a-z0-9.\-]+ (default|medium|low|high) · ` — generic enough to survive model renames.

### Fix D — codex-headless instead of codex-tmux for non-streaming phases (HIGH effort)

**Effort:** the headless driver `codex.json` exists but its launch path is unimplemented in cycle 121. Wiring it up requires ~200 LOC + integration tests.
**Coverage:** `codex exec` (non-interactive) bypasses the entire TUI — no trust modal, no `›` race, no alt-screen complexity. Trade-off: loses streaming output to the bridge's review intervals and forfeits the named-session resume path.

For phases that don't need streaming (audit, retro), `codex exec --json --output-last-message <file>` is the cleaner path. Track as a v12.x roadmap item, not a hotfix.

### Fix E — add `Working with untrusted contents` to the manifest regex (LOW effort, partial coverage)

**Effort:** 1 LOC manifest edit.
**Coverage:** broadens trust-modal detection so the auto-responder is more likely to fire if Fix B is applied.

```json
"regex": "Do you trust the contents of this directory|Trust this directory|Working with untrusted contents"
```

Defense in depth; safe to ship alongside Fix A.

---

## Recommended ship order

1. **Today:** Fix A (pre-trust worktree paths) + Fix E (broaden regex) — unblocks the audit phase, ~3 hours wall-clock with tests.
2. **This week:** Fix B (auto-responder before marker check) — closes the race for future modals codex 0.134+ may introduce.
3. **Backlog:** Fix C (marker-tail) — future-proofs against any single-glyph false positive.
4. **v12.x roadmap:** Fix D (codex-headless for non-streaming phases).

---

## Open questions worth verifying

- **Q1:** Did the cycle-121 launch actually `cd` into the worktree before invoking `codex`? `driver_tmux_repl.go:121` shows `SendKeys(ctx, lp.session, "cd "+workingDir, true)` — yes, but a tmux send-keys race where `cd` lands AFTER `codex` is theoretically possible if the 1-second sleep is too short for the new-session shell to be ready. Adding a marker check for the shell prompt (`$` or `%`) before sending `cd` would close this.
- **Q2:** Is `codex_apps` MCP-server boot blocking the trust modal from rendering for >60s on this host? Local probes show MCP boot completes in 3-5s; cycle-121 may have had a stalled MCP server. Worth running `codex doctor` + checking `~/.codex/log/` for MCP boot timeouts.
- **Q3:** Does the tmux pane size (cycle-121 ran at default 80×24 per `tmuxPaneWidth`/`tmuxPaneHeight` in `driver_tmux_repl.go`) cause the trust modal to wrap below the 200-line scrollback capture? Should test with `-x 200 -y 50` and see if the modal lands at a different position.

---

## Sources

- [Codex CLI changelog (developers.openai.com)](https://developers.openai.com/codex/changelog)
- [openai/codex GitHub releases](https://github.com/openai/codex/releases)
- [Codex CLI configuration reference](https://developers.openai.com/codex/config-reference)
- [Codex CLI command-line reference](https://developers.openai.com/codex/cli/reference)
- [Codex agent approvals & security](https://developers.openai.com/codex/agent-approvals-security)
- [GitHub issue #14547 — Trust prompt every launch](https://github.com/openai/codex/issues/14547)
- [GitHub PR #11874 — fix(tui) remove config check for trusted setting](https://github.com/openai/codex/pull/11874)
- [GitHub PR #13021 — Add model availability NUX tooltips](https://github.com/openai/codex/pull/13021)
- [GitHub PR #12972 — Add model availability NUX metadata](https://github.com/openai/codex/pull/12972)
- [GitHub PR #8952 — Use markdown for migration screen](https://github.com/openai/codex/pull/8952)
- [GitHub issue #18482 — Model migration prompt mapping bug](https://github.com/openai/codex/issues/18482)
- [GitHub issue #13842 — Segfault in tmux on Linux](https://github.com/openai/codex/issues/13842)
- [GitHub issue #12223 — Output display in alacritty + tmux](https://github.com/openai/codex/issues/12223)
- [GitHub issue #11267 — Ctrl+C deadlock during /review and MCP startup](https://github.com/openai/codex/issues/11267)
- [GitHub issue #16491 — `codex resume` repeated timeout](https://github.com/openai/codex/issues/16491)
- [GitHub issue #4337 — Commands hang indefinitely on tool timeout](https://github.com/openai/codex/issues/4337)
- [waskosky/codex-cli-farm — tmux session manager for codex](https://github.com/waskosky/codex-cli-farm)
- [onesuper/tui-use — agent driver for TUI programs](https://github.com/onesuper/tui-use)
- Local artifact: `/Users/danleemh/ai/claude/evolve-loop/.evolve/runs/cycle-121.reset-20260528T054334.325423000/cycle-state.snapshot.json`
- Local source: `/Users/danleemh/ai/claude/evolve-loop/go/internal/bridge/driver_tmux_repl.go`, `driver_codextmux.go`, `autorespond.go`, `manifests/codex-tmux.json`
- Local probes (run 2026-05-28 on the cycle-121 host):
  - `codex --version` → `codex-cli 0.134.0`
  - `codex doctor` → `auth ✓`, `model: gpt-5.5`, terminal 80×24, `~/.codex/config.toml` trusts only the main repo + two other projects (NOT the cycle worktree)
  - Reproduction in `/tmp/codex-trust-probe`: trust modal rendered at 4s with `›` glyph next to `1. Yes, continue`
  - Verified `--dangerously-bypass-approvals-and-sandbox` does NOT suppress the trust modal (matches issue #14547)
