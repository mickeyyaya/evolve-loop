# 2026-09-01 — claude 2.1.252 flips the folder-trust default to "No, exit"

## Impact
Wave-20260901b: all three fleet lanes (cycles 1598/1599/1600) teardown-FAILed
in triage with artifact-timeout (exit 81), zero extends, `submit_wedged`
resends=3. 0 ships. Codex-driven scout phases completed normally; every
claude phase launched in a fresh cycle worktree died identically. Loop halted
by operator per ADR-0072 (three parallel same-fingerprint lane fails =
SYSTEM) before wave 2 dispatched; the three inbox items were auto-released by
the inbox-mover (failures were pre-build — no work lost).

## Root cause
claude 2.1.252 changed the boot-time folder-trust dialog in two ways at once:
the options lost their numbering, and the pre-highlighted default flipped
from "Yes, I trust this folder" to **"❯ No, exit"**. The v2.1.193-era
auto-respond rule (`trust_prompt`) therefore missed twice: its regex anchors
on the numbered `1. Yes, I trust this folder` line (no longer rendered), and
its response — a bare Enter for the then-Yes-default — now confirms
"No, exit", killing the REPL. The dialog's ❯ cursor also satisfies the REPL
prompt marker, so the boot loop declared ready and pasted the phase prompt
into the modal (the known v2.1.193 collision, unhandled for the new pane).
Preflight had WARNed the version drift (2.1.251 → 2.1.252) at batch boot.

## Evidence
- Escalation report final_pane (cycle-1599 triage): the dialog verbatim with
  `❯ No, exit` selected, then the shell prompt back — claude exited.
- Live reproduction same day: fresh tmp dir, `claude --model haiku
  --dangerously-skip-permissions` → identical dialog.
- Remedy verified live before coding: `Down` moves the cursor to
  "Yes, I trust this folder", `Enter` boots a trusted REPL (status bar
  confirms workdir + model).

## Fix (fix/claude-2252-trust-dialog)
New manifest rule `trust_prompt_no_default` (claude-tmux): matches the
selected-No line plus the Yes option, bottom-anchored on the
`Enter to confirm · Esc to cancel` footer (`\z`, `tail_lines` 12, distance
bounds — the plan_approval discipline from
2026-08-27-plan-mode-dialog-blind-spot.md), responds `Down,Enter`,
fire-once. The v2.1.193 rule stays for older builds and was bottom-anchored
symmetrically in the same change — the architecture review's probe showed
the unanchored form self-matching this repo's own tracked files when an
agent renders them, and showed the fix's own test fixture becoming a new
trigger with a masking flip (the old rule's once-budget, previously spent at
boot, would survive to fire on the first rendered file and then suppress a
genuine later dialog).

## Lessons
- A CLI's interactive dialogs are part of its interface contract: version
  drift WARNs at preflight deserve a dialog-smoke before a batch, not after.
- Every auto-respond rule carries the bottom-anchor discipline from birth;
  "prose cannot false-match" is not a property — `\z` + `tail_lines` is.
- Fixture text of dialogs inside tracked files is itself a false-match
  vector; the self-match guard test must cover every rule family that quotes
  its own dialog.
