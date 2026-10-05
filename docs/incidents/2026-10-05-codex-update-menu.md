# 2026-10-05 — codex's startup update menu took every codex dispatch of wave 59

## Impact

Every codex-tmux phase launch in wave 59b (cycles 1791–1793) failed and fell
back to claude. The runtime plane's loop log
(`.evolve/loop-20261005-wave59b.log`) records 19 codex launches and none that
reached a working session: 16 ended `exit 81` (artifact timeout, cause
`submit_wedged`) and 3 ended `exit 80` (`ExitREPLBootTimeout`: cycle 1792's
test-amplification and one build, cycle 1793's retrospective). Each cost the
phase its codex attempt (300 s for the light phases, up to 1500 s for a build)
before the fallback ran. The cli-health canary saw `rc=81` and, correctly for
its rules, called it "not a wall", so nothing benched codex and every phase kept
trying it first. The loop readiness gate's codex boot passed: it judged
readiness on codex's composer before the menu mounted (Root cause, step 2).

## Root cause

Three defects lined up.

1. **The update menu came back.** codex 0.153.4 (Homebrew cask, pinned) opens
   with a menu when its cached `latest_version` is newer than itself (the
   capture below carries a `│` gutter and drops the blank lines, so a pane
   that shows this page can never take the live menu's shape):

   ```text
   │   ✨ Update available! 0.153.4 -> 0.160.0
   │   Release notes: https://github.com/openai/codex/releases/latest
   │ › 1. Update now (runs `brew upgrade --cask codex`)
   │   2. Skip
   │   3. Skip until next version
   │   Press enter to continue
   ```

   The cursor starts on `Update now`. The codex preflight
   (`dismissCodexUpdateNag`) writes `dismissed_version: "999.999.999"` to
   `~/.codex/version.json` to keep this menu away; on 0.153.4 it does not: the
   menu showed with exactly that file on disk (`latest_version` 0.160.0).
2. **Readiness was judged before the menu mounted.** codex draws its composer
   (`› Ask Codex to do anything`, whose `›` is the codex-tmux prompt marker)
   about 0.2 s after launch, and replaces it with the update menu about 2.4 s
   after launch. The codex boot polls every 2 s, so its first capture saw the
   composer, found no dialog and declared the REPL ready
   (`[codex-tmux] REPL prompt (›) detected`). The prompt was pasted into the
   menu, which ignores pasted text, and the submit Enter confirmed the menu's
   default: `Update now`. codex ran `brew upgrade --cask codex` inside the
   sandbox, which failed (`/opt/homebrew/Cellar is not writable`, `Not
   upgrading 1 pinned package`), and codex exited to zsh. Submit-verify found
   no echo of the prompt after the last `›` (the menu's cursor line) and
   recorded the prompt as submitted; the stop review's nudge then went to the
   shell and stayed
   "parked at the `›` input line" through three re-sends (`submit_wedged`,
   exit 81). This is the path of all 16 exit-81 launches: none of their
   launch logs carries the bespoke path's line (step 3).
3. **The bespoke boot path answered a menu it could not see.** When a boot
   pass did capture the menu, `bootTmuxREPL` sent the codex driver's
   `bootMenuSkip` key, `2`, with Enter, on any pane that contained
   `Update available!`, `Update now` and `Skip` anywhere in its 200 captured
   lines (`tmuxPaneLooksLikeUpdateMenu`). On the live menu the digit `2`
   selects Skip at once, so the first send worked. But the answered menu stays
   in the scrollback above the composer, the check had no bottom anchor, and
   it ran on every boot pass: each pass typed `2` and Enter into the composer
   until the 60 s boot deadline (exit 80). In cycle 1793's retrospective the
   typed `2` reached the model as a prompt (`› 2`, answered by the account's
   400 for the deep-tier model). The two exit-80 launches whose logs survive
   (cycle 1792's test-amplification, cycle 1793's retrospective) record the
   dismissal line 30 times each. Had a `2` been dropped (sent before
   the menu read input), the Enter that followed it would have confirmed
   `Update now`, the burst defect [ADR-0118](../architecture/adr/0118-confirm-a-menu-only-on-the-observed-choice.md)
   removed from claude's trust dialog the same day.

## Evidence

- `runtime/.evolve/runs/cycle-1791/scout-escalation-report.json`
  `final_pane`, and `tmux-final-scrollback.txt` of the same cycle's build: the
  launch line, the menu on its default, `Updating Codex via brew upgrade --cask
  codex...`, the brew errors, the zsh prompt, and the build's nudge text typed
  into zsh as a `quote>` continuation.
- `runtime/.evolve/runs/cycle-179{1,2,3}/*-launch-error.txt`: every exit-81
  launch logs `REPL prompt (›) detected` with no dismissal line;
  `cycle-1792/test-amplification-launch-error.txt` and
  `cycle-1793/retrospective-launch-error.txt` log `boot interstitial dismissed
  before prompt delivery` 30 times each, then `REPL prompt never appeared
  after 60s`. `cycle-1793/escalation-report.json` (pattern
  `model_unsupported`) shows `› 2` submitted to the model.
- `~/.codex/version.json` at the time: `dismissed_version` `999.999.999`,
  `latest_version` `0.160.0`.
- Live probes on codex 0.153.4, 2026-10-05, in a tmux pane of the bridge's
  size in a fresh temp directory, capturing every 0.2 s and never sending
  Enter on `Update now`:
  - plain `codex`: composer frames from 0.2 s to 2.2 s, the update menu from
    2.4 s;
  - `Down` moves the cursor 1 → 2 → 3 and wraps 3 → 1;
  - `Enter` with the cursor on Skip draws the composer below the menu within a
    second, and the menu text stays above it in the scrollback;
  - the digit `2` alone selects Skip;
  - `codex -c check_for_update_on_startup=false`, with the same
    `version.json`: composer, then codex's trust dialog for the untrusted temp
    directory, and no update menu in any frame; `version.json` unchanged.
  - `strings` on the binary lists `check_for_update_on_startup` among the
    top-level `ConfigToml` keys.

## Fix (fix/codex-update-menu-never-confirms-update)

Prevent, then defend, as data
([ADR-0118 addendum](../architecture/adr/0118-confirm-a-menu-only-on-the-observed-choice.md#addendum-2026-10-05-codexs-update-menu)):

- **Prevent.** The codex-tmux manifest's `default_args` now carry
  `-c check_for_update_on_startup=false` after `--yolo`, so every realized
  codex launch (the engine's phase launch, the clicontrol recipe) starts with
  codex's update check off. It is a per-launch override: no host-global file
  is written.
- **Every smoke launches what a phase launches** (architecture review, same
  day). The boot smoke (the readiness gate, `evolve doctor boot`) and the live
  smoke (the CLI-health canary, `evolve doctor live`) were launched with no
  realization, so the canary still ran bare `codex` into the menu and misread
  the family. `BootSmokeTest` and `LiveSmokeTest` now share
  `smokeLaunchConfig`, which realizes the driver's manifest with the bypass
  intent whenever the caller supplies none; a caller's own realization is
  launched unchanged.
- **Defend.** Two manifest rules answer the menu if it shows anyway, in
  ADR-0118's shape: `update_menu_cursor_off_skip` sends `Down` when the cursor
  is on `Update now` or on option 3 (repeatable, bounded by the loop guard;
  permissive, so a torn redraw with two cursors gets Down), and
  `update_menu_cursor_on_skip` sends `Enter` only when the cursor is on Skip
  with uncursored options 1 and 3 around it (strict). Both are bottom-anchored
  on the footer (`\z`, `tail_lines` 8), so the answered menu above the
  composer matches neither, and navigation precedes confirmation in the
  manifest. A cursor that never moves ends the boot on the loop guard as a
  boot timeout (exit 80) with a FAIL line naming the rule, and no Enter.
- **Fail closed on a menu no rule parses** (architecture review). The menu's
  cursor `›` is codex's prompt marker, so a variant the shaped rules miss (the
  review's probe: a footer reading "Press enter to confirm or esc to skip")
  was declared ready on its own cursor, and the prompt's Enter confirmed
  `Update now`. A third rule, `update_menu_unparsed`, with a new policy,
  `hold`, sends nothing and makes the boot re-poll instead of judging
  readiness; six holds end the boot as a boot timeout naming the rule. It
  holds any frame whose last 24 lines start a line with the menu's header and
  carry at most one `›` after it, so it lets go once codex draws its composer
  below the answered menu, and it never holds a busy pane. The re-review found
  the hold also matched in run phases (an idle pane whose agent quotes the
  header, with the composer's `›` below it: five holds, then the loop guard
  abandoned the run before its idle-artifact nudge), so the hold is boot-only:
  the responder drops it once readiness is declared (`autoResponder.endBoot`,
  at the tmux and the recipe boot). It is also the manifest's last rule, so it
  never shadows a rule that can answer the same frame.
- **Self-match guards slide the pane bottom over every line** (architecture
  review). The update-menu guard had run the rules on whole files, which a
  bottom-anchored rule only tests at the end. `requireNoRuleFiresAtAnyPaneBottom`
  checks the 64 lines ending at every line of each file. It caught this
  record's own capture of the menu (now behind a `│` gutter) and, applied to
  ADR-0118's claude trust guard, two claude test fixtures that quote the
  numbered trust dialog flattened with `\n` escapes; claude's `trust_prompt`
  now needs its cursor line and footer to begin a line.
- **Delete the bespoke path.** `tmuxLaunch.bootMenuSkip`, its send in
  `bootTmuxREPL` and `tmuxPaneLooksLikeUpdateMenu` are gone; no boot path sends
  keys except through the manifest rules. The other drivers have no bespoke
  boot menu path: claude's and agy's boot dialogs are manifest rules, and the
  recipe boot (`EnsureSession`) uses the same `bootTick`.

Pinned by `TestCodexLaunch_ArgvCarriesTheUpdateCheckOffOverride`,
`TestCodexUpdateMenu_ASwallowedDownNeverLetsEnterConfirmUpdateNow`,
`TestCodexUpdateMenu_ADownThatLandsBootsWithOneDownAndOneEnterOnSkip`,
`TestCodexUpdateMenu_ACursorThatNeverMovesEndsTheBootAsABootTimeoutWithoutConfirming`,
`TestCodexUpdateMenu_AnAnsweredMenuLeftInScrollbackAboveTheComposerIsNeverAnsweredAgain`,
`TestAutoRespond_CodexUpdateMenuRulesAreDisjointOnEveryMenuFrame`,
`TestAutoRespond_CodexUpdateMenuNavigationWinsAFrameBothRulesMatch`,
`TestAutoRespond_CodexUpdateMenuRulesDoNotMatchThisRepositorysOwnFiles`,
`TestCodexUpdateMenu_AMenuTheRulesCannotParseEndsTheBootAsABootTimeoutWithoutAnyKey`,
`TestAutoRespond_CodexUpdateMenuTailLinesRejectsAFooterFarBelowTheOptions`,
`TestAutoRespond_CodexUpdateMenuHoldSkipsABusyPane`, the smoke tests
`TestBootSmokeTest_ACallerWithNoRealizationLaunchesCodexWithTheManifestsArgs`,
`TestLiveSmokeTest_ACallerWithNoRealizationLaunchesCodexWithTheManifestsArgs`,
`TestBootSmokeTest_ACallerWithNoRealizationBootsClaudeAndAgyAsAPhaseLaunchDoes`,
`TestBootSmokeTest_ACallerWithNoRealizationGetsOllamasDefaultArgs` and
`TestBootSmokeTest_ACallerRealizationIsLaunchedUnchanged`, the callers'
`TestCLIHealthCanaryProbe_LaunchesCodexWithItsStartupUpdateCheckOff` and
`TestDoctorLive_LaunchesCodexWithItsStartupUpdateCheckOff`, and
`TestRealizeFor_RealManifests_NoCrossCLILeak`; `TestCodexUpdateMenuDismiss`
and `TestAdversarialFaultMatrix/codex_update-menu` were rewritten from the
`2` keypress to the observed Down and Enter. The boot tests' fake pane models
the live menu: Down wraps, `2` selects Skip, Enter on `Update now` runs the
failing upgrade and exits to the shell, and the answered menu stays above the
composer. Each test failed on the previous code for the reason it names: the
swallowed `2` let Enter confirm `Update now`, the stale menu kept the boot from
ever declaring readiness, and the launch line lacked the override.

## Residual risk

- **The composer-first race is still open.** The menu no longer shows, but
  codex draws its composer before any startup dialog, and the trust dialog for
  an untrusted directory also mounts after the composer (about 3.4 s after
  launch in the probe). The bridge pre-trusts each worktree, so that dialog
  does not show today; any new startup dialog would hit the same race, because
  the defence rules only see a menu that is on screen at a boot pass. Filed as
  inbox `codex-readiness-judged-on-the-composer-drawn-before-a-startup-dialog`.
  The hold rule only sees a menu that a boot pass captures, so the filed item's
  acceptance now also asks that an unparsed dialog at the delivery's
  pre-submit capture gets no submit Enter.
- **`dismissCodexUpdateNag` keeps a false name and a dead write.** It still
  writes the `999.999.999` sentinel; its one live role is to make the
  `version.json` the readiness gate's freeze check reads exist. Filed as inbox
  `codex-version-state-file-without-the-dead-dismissal-sentinel`.
- **The plan-mode self-match guard still reads whole files.** A sliding scan
  shows codex's `plan_question` fires on a pane ending at its own fixture
  (`autorespond_planmode_test.go:36`). Filed as inbox
  `plan-mode-self-match-guard-checks-only-the-end-of-each-file`.
- **The bridge tests write the real codex config.** Tests that drive codex
  through `Engine.LaunchArgs` run the codex preflight against `~/.codex`: on
  this host `config.toml` had grown to 20 MB with 141,664 `[projects]` trust
  entries, 140,371 of them test temp directories. Filed as inbox
  `bridge-tests-write-the-real-codex-config`.

## Lessons

- Prevent before you defend: a CLI's own startup interstitial is best switched
  off by a per-launch flag, and the auto-responder rule is the backstop.
- A check that scans the whole scrollback for a dialog's words matches the
  dialog after it is answered. Every dialog rule needs the bottom anchor, and
  no boot path may answer a dialog outside the manifest rules that carry it.
- A readiness marker the CLI draws before its startup dialogs is not
  readiness: the first frame with the marker can precede the dialog that will
  swallow the prompt.
- A dismissal written into a CLI's private state file is version-bound: the
  `999.999.999` sentinel stopped working without any error.
- A prevention that lives in the launch realization only covers the launches
  that realize; a probe that builds its own launch must share the same
  realization, or it tests a different CLI from the one the phases run.
- A dialog the rules cannot parse must fail closed. When the dialog's own
  cursor is the readiness marker, "no rule matched" reads as "ready".
- A self-match guard for a bottom-anchored rule has to move the bottom: a
  whole-file check only ever tests the file's last lines.
