# 2026-10-26: the claude usage capture stopped before the usage rows

Related: [internal-bridge package note](../architecture/packages/internal-bridge.md) (the spec of the usage capture), [agent bridge factory](../architecture/agent-bridge-factory.md) §7, [CLI-failure triage](../operations/cli-failure-triage.md).

## Summary

- **Impact.** `evolve clihealth usage claude` printed `no usage window read` and exited 0. The pre-wave usage probe and the failure evidence read no claude window for the same reason.
- **Root cause.** The capture of a control stopped at the first tick that showed the prompt marker `❯`. Claude Code 2.1.310 shows `❯` in its input box before `/usage` runs, and the box stays on the screen under the usage dialog.
- **Trigger.** Claude Code 2.1.310 sometimes blocks its event loop for some seconds at startup. In that time, the screen does not change and the pasted `/usage` does not show.
- **Fix.** The claude manifest sets `controls.usage.await` to `windows`. The capture then waits until the usage rows are on the screen and stay the same for two ticks. The verb exits 1 when a CLI that declares windows reads none.

## Timeline

| Time (Asia/Taipei) | Event | Evidence |
|---|---|---|
| 2026-10-21 | `evolve clihealth usage claude` read the claude windows. | The operator's report. |
| 2026-10-26 | The verb printed the note for claude, and the sweep sent SIGKILL to 10 processes. agy read its windows. | The operator's report. |
| 2026-10-26 20:05 to 20:12 | `evolve bridge control claude usage` returned a screen with no `/usage` text and no rows in 9 of 15 runs. | The captures that the bridge printed. |
| 2026-10-26 20:08 | A frame recorder on the `evolve-bridge` tmux socket showed the dialog with the three rows in each passing run. | The frames, 0.25 s apart. |
| 2026-10-26 20:11 | In the 2 failing runs with `ps`, 7 child processes of `claude` stayed `<defunct>` from the first `❯` frame to the session kill. In the 4 passing runs, `claude` collected them in 1.5 s or less. | `ps` beside each frame. |
| 2026-10-26 20:19 to 20:21 | The fixed binary read three claude windows in 8 of 8 runs, with `SIGKILL: 0` at each dispatch end. agy read four windows. | The verb output. |

## Root cause

1. `captureControl` (`go/internal/bridge/recipe_adapter.go`) sends the command. Then it reads the screen once each second, for up to 8 ticks. It stopped at the first tick that showed the prompt marker.
2. The prompt marker of the claude manifest is `❯`. Claude Code draws `❯` in the input box at REPL start. Thus the marker shows before the command runs.
3. The capture thus returned after one tick. The usage dialog takes more than one second to show, so the returned screen had no row.
4. In a failing run, `claude` did not collect its exited children, and the screen did not change. In a passing run, `claude` collected the same children in 1.5 s or less. These facts show that the event loop of `claude` was blocked in the failing runs (synthesis).
5. The capture killed the session while `claude` was blocked. `claude` did not stop on SIGTERM within the 2 s grace, so the sweep sent SIGKILL. The sweep also counted each `<defunct>` child, because a zombie stays in the process table until its parent collects it. That is why all 10 processes got SIGKILL.
6. `readUsage` gave a note, not an error, so the verb exited 0 and hid the failure.

The parse rules were correct. Today's dialog has the same layout as the 2.1.291 screen: a label line, a value line with `N% used` and a `Resets` line. The one-line form `Current session: 5% used · resets ...` is a summary of the screen, not the screen text.

## What changed

- **A control can wait for its windows.** With `ControlSpec.Await` set to `windows`, the capture waits for a window readout that is not empty. The readout must be the same on two consecutive ticks. The limit is 30 ticks (`windowsSettled`, `usageCaptureSettleTicks`). Test: `TestController_Usage2_1_310WaitsPastTheIdlePromptUntilTheRowsStopChanging`. Its fixtures are the idle, partial and full screens of 2.1.310 (`go/internal/quotastate/testdata/claude_usage_2.1.310*.txt`).
- **The manifest owns the rule.** Go has no switch on a family.
- **A windows await needs a window spec.** The manifest load refuses `await: windows` on a control with no `windows` block (`validateControlAwaits`). Test: `TestParseManifest_RefusesAWindowsAwaitWithoutAWindowSpec`.
- **The verb fails loudly.** When the manifest of a CLI declares `controls.usage.windows` and the screen yields none, `readUsage` sets an error, and the verb exits 1 (`bridge.UsageWindowsDeclared`). A CLI with no window spec still gets a note. Tests: `TestReadUsage_ACLIThatDeclaresWindowsAndReadsNoneIsAnError`, `TestClihealthUsage_AScreenThatHidesTheDeclaredWindowsExitsOne`.
- **agy is not changed.** Its manifest keeps `await: prompt_marker`, because its marker shows only after the usage screen.

## What we learned

- **A prompt marker that is on the screen before the command runs cannot end the wait.** For each control, wait for what the control reads. The manifest says what that is.
- **A read that finds nothing is a failure when the manifest says that there is something to read.** A note with exit 0 hid the failure for five days.
- **Zombie children are not survivors.** The sweep counts each `<defunct>` child of a blocked parent as a SIGKILL target. When a CLI is killed after it is ready, the count drops to 0 SIGKILL. A count of zombies in the sweep report is open work.
