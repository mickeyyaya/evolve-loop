# 2026-11-14: an agy CLI update changed its footer, and the loop halted

Related documents: the research note [claude-login-split-2026-11.md](../research/claude-login-split-2026-11.md) (the update of 2026-11-02 that broke Claude seats) and the package note [internal-bridge.md](../architecture/packages/internal-bridge.md).

## Summary

- **Impact:** the loop halted on the smoke failure after the CLI update. The fact sheet gives no start time and no end time, so the duration is not recorded.
- **Root cause:** agy 1.5.9 puts a mode word before the model in its footer, for example `auto · Gemini 3.8 Flash · high`. The pattern `model_label_regex` did not allow the mode word.
- **Effect:** the launch model verification read a correct seat as an unknown model. The boot was refused with exit 87.
- **Fix:** PR #1456 adds an optional mode prefix to `model_label_regex`. A test boots over a real 1.5.9 footer.
- **Open item:** a smoke failure after a CLI update halts the whole loop. It must bench only the CLI family. The inbox item `cli-update-smoke-failure-benches-the-family-not-the-loop` tracks this.

## Timeline

| Time (UTC) | Event | Evidence |
|---|---|---|
| 2026-11-14 | agy updates to 1.5.9. The footer now has a mode word before the model. | footer sample in `agyFooterModePrefix`, `go/internal/bridge/agy_every_launch_model_test.go:172` |
| 2026-11-14 | The launch model verification cannot read the model. The boot is refused with exit 87. The CLI-update smoke fails and the loop halts. | `ExitModelMismatch = 87` in `go/internal/bridge/exitcodes.go:18` |
| 2026-11-14 | PR #1456 is integrated. | commit `22e63a4a6` |

## Root cause

**F1.1** The launch model verification reads the booted model from the footer line with `model_label_regex` (`go/internal/bridge/manifests/agy-tmux.json:184`).
*Implication:* the pattern decides which model the seat runs.

**F1.2** The old pattern did not allow a mode word (`auto`, `plan`, `ask` or `default`) before the model.
*Implication:* the footer `auto · Gemini 3.8 Flash · high` did not match.

**F1.3** A footer with no readable model counts as unreadable, and unreadable means exit 87 (`ExitModelMismatch`). The signal text in `go/internal/bridge/launch_model_verification.go:26` says: "no prefix line means unreadable, which means 87".
*Implication:* a correct seat looked like a wrong seat, and the boot was refused.

**F1.4** Exit 87 is a walk trigger: the try moves to the next CLI. The smoke check that runs after a CLI update treated the failure as a failure of the whole loop, and the loop halted.
*Implication:* one family with a changed footer stopped the families that were fine.

## What changed

| Change | Regression test (red without the change) |
|---|---|
| PR #1456: `model_label_regex` gains the optional group `(?:(?:auto\|plan\|ask\|default)\s*·\s*)?` before the model, in `go/internal/bridge/manifests/agy-tmux.json` | `TestAgyBoot_ReadsTheModelBehindTheModePrefixOfAgy159` in `go/internal/bridge/agy_every_launch_model_test.go`. It boots through the helper `bootAgyTmuxOver` over a real 1.5.9 footer. |

The same PR registers the test in `go/internal/bridge/parallel_serial_tests_test.go`, because the test swaps the seam `modelCatalogDirFn`.

The inbox item `cli-update-smoke-failure-benches-the-family-not-the-loop` is named by the fact sheet. This worktree holds no file for it, so the record does not cite a path.

## What we learned

- A CLI update changes the footer that a pattern reads. A pattern on text that a CLI owns must allow the optional parts of that text. A test must hold a real sample of each footer.
- The same class of event happened on 2026-11-02: the update of the Claude CLI to 2.1.316 broke the project-scoped seats (see [claude-login-split-2026-11.md](../research/claude-login-split-2026-11.md), F5.3). A CLI update at a boundary can break every seat of one family at once.
- A smoke failure after a CLI update must bench that CLI family and not halt the loop. This is not yet a guard; the inbox item tracks it.
