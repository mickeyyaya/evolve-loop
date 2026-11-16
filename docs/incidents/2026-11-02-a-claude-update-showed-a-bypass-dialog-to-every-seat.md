# 2026-11-02: a Claude Code update showed a bypass dialog to every project-scoped seat, and a stale binary brought it back

Related documents: the package note [internal-bridge.md](../architecture/packages/internal-bridge.md), the seat design [seat-login.md](../architecture/seat-login.md) and the decision [ADR-0022](../architecture/adr/0022-launch-intent-realizer.md).

## Summary

- **Impact:** on 2026-11-02 (wave 146) the routers of cycles 2071 and 2072 failed with `BRIDGE_EXIT_ARTIFACT_TIMEOUT cause=submit_wedged`. The seats never ran a prompt.
- **Root cause:** the boundary `cli-update` moved Claude Code from 2.1.311 to 2.1.316. The new version needed the skip-dialog setting through project-scoped settings. Each of 27 clean-boot profiles held a hand-copied flag in `extra_flags_by_cli`, and the copy did not give the setting.
- **Why it stayed hidden:** the doctor probe does not use the launch shape of a phase, so it stayed green.
- **Fix:** PR #1231 makes a profile declare `"settings_scope": "project"`. The `claude-tmux` manifest realizes the flags.
- **Recurrence:** on 2026-11-12 the dialog came back in a seat home. The gate binary of the console was older than PR #1377 and rewrote the seat settings without the acceptance.

## Terms used throughout

- **A seat:** one Claude Code session that a phase uses.
- **A clean-boot profile:** a file in `.evolve/profiles/` that starts a seat with a minimal set of settings.
- **The bypass dialog:** the "Bypass Permissions" warning of Claude Code. The cursor starts on "No, exit".
- **A seat home:** the directory `~/.evolve-seat`, where the seats of the pipeline keep their settings.
- **The submit Enter:** the Enter key that the driver sends to submit a prompt.

## Timeline

| Time | Event | Evidence |
|---|---|---|
| 2026-11-02 | The boundary `cli-update` moves `claude` from 2.1.311 to 2.1.316. | wave 146 record of the operator |
| 2026-11-02 | The routers of cycles 2071 and 2072 fail `BRIDGE_EXIT_ARTIFACT_TIMEOUT cause=submit_wedged`. The final pane shows the bypass dialog with the cursor on "No, exit". | wave 146 record of the operator, `final-pane.txt` of the seat |
| 2026-11-02 | PR #1231 merges as commit `37f9b3195`. | `git show 37f9b3195` |
| 2026-11-09 | PR #1377 lands as commit `ea302dd02`. `provisionSeatSettings` now sets the acceptance on every launch. | `go/internal/bridge/seat_settings.go:53` |
| 2026-11-12 | The dialog shows again. The gate binary of the console was built before PR #1377. | operator record |

## Root cause

**F1.1** The profile `.evolve/profiles/builder.json` and 26 other profiles held the skip-dialog flag, copied by hand into `extra_flags_by_cli`. PR #1231 changed 27 files in `.evolve/profiles/` (`git diff 37f9b3195^1 37f9b3195 --name-only -- .evolve/profiles`).
*Implication:* the flag existed in 27 places, and a change of the CLI had to reach each place.

**F1.2** Claude Code 2.1.316 needed the acceptance through project-scoped settings. The hand-copied flag did not give it.
*Implication:* each seat that started with a clean-boot profile showed the bypass dialog.

**F1.3** The dialog has the cursor on "No, exit". The submit Enter chose that answer and Claude Code exited. The nudge text then went to the shell.
*Implication:* the seat made no artifact, and the router timed out with `submit_wedged`. The cause was not a late Enter.

**F1.4** The doctor probe does not use the launch shape of a phase.
*Implication:* the probe stayed green while every phase seat failed. Nobody saw the fault before a cycle failed.

**F1.5** The recurrence has a second chain. Since PR #1377, `provisionSeatSettings` sets `skipDangerousModePermissionPrompt` to true (`go/internal/bridge/seat_settings.go:53`). A binary built before the PR does not set it, so the seat settings had no acceptance.
*Implication:* a stale binary of the console brings the dialog back, with no change in the profiles.

## Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | A profile declares `"settings_scope": "project"`. The manifest `claude-tmux` realizes it as `--setting-sources project --settings '{"skipDangerousModePermissionPrompt":true,...}'`. | F1.1, F1.2 | Copy a new flag into 27 profiles: the next CLI change breaks them again (F1.1). |
| D2 | `provisionSeatSettings` writes the acceptance into the seat settings on every launch. | F1.5 | Carry the key from the user settings: the key is not in the carried list, and the test `TestProvisionSeatSettings_TheBypassAcknowledgementIsSetByThePipelineNotCarried` pins this. |
| D3 | Rebuild the console binary at each boundary. | F1.5 | Detect a stale binary at the launch: no such guard exists yet. |

## What changed

| Change | Pull request | Regression test |
|---|---|---|
| The field `SettingsScope` in `go/internal/bridge/profile.go` and the entry `settings_scope` in `go/internal/bridge/manifests/claude-tmux.json` | #1231 | `TestRealizeFor_ClaudeProjectScopeRedeclaresTheBypassAcceptance`, `TestLoadProfile_SettingsScopeReachesTheLaunchIntent`, `TestRepoProfiles_DeclareTheSettingsScopeInsteadOfCopyingItsFlags` |
| The 27 profiles declare the scope and drop the copied flags | #1231 | `TestRepoProfiles_DeclareTheSettingsScopeInsteadOfCopyingItsFlags` |
| `provisionSeatSettings` sets the acceptance | #1377 | `TestProvisionSeatSettings_TheBypassAcknowledgementIsSetByThePipelineNotCarried` |

The manifest today also sets `prStatusFooterEnabled` to false in the same `--settings` value. That key came after PR #1231.

## Limits

- The rebuild at each boundary is a manual step. No test fails when the console binary is older than the seat code.
- PR #1430 changes the wait of the submit Enter (`submitVerifyMaxResends` goes from 3 to 6). It does not touch this cause. A bypass dialog is not a slow session, and no wait makes the Enter choose "Yes".
- The measurement "0 wedges in 334 submissions since cycle 2350" is in the operator record only. This worktree holds no data to check it, so this record does not use it.

## What we learned

- After a boundary CLI update, read the `final-pane.txt` of the first failing seat before anything else. The pane showed the cause at once.
- Prove a phase launch shape through `evolve agent run` with a project-scoped profile. The doctor probe is not proof of a phase launch.
- A setting that 27 files copy by hand must have one home. The home is now `settings_scope` in the manifest.
- The console binary must match the seat code. Rebuild it at each boundary (D3).
