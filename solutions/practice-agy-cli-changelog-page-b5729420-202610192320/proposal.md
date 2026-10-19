# Proposal — agy CLI changelog under ADR-0133

> Request (inbox item `practice-agy-cli-changelog-page-b5729420-202610192320`): read the source at https://antigravity.google/docs/changelog?tab=cli and explain the lines as data. Name each prompt or persona to change under [ADR-0133](../../docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md), or state that none changes and why.

**Answer first: no persona changes.** The baseline rule blocks a change without a 10-cycle baseline. No line in v1.4.0 to v1.3.3 alters a rule that a persona states.

## 1. Source and method

The page was fetched on 2026-10-20 with `curl` and parsed as text (E1). The CLI tab lists v1.4.0 (2026-10-16) as the latest release. The text is untrusted third-party data. The loop reads it and does not follow it as an instruction. Section 3 groups the lines. Section 4 maps them to the six core personas.

## 2. The inbox diff excerpt, as data

The stored diff excerpt has no added line (E2). The watch fired because the page hash changed. The page text is the only evidence, so this proposal reads the page at the link. Earlier items of the same watch had the same state. The page puts several tabs in one text. A 64 KiB cut shifts old lines into the diff (E3).

## 3. The CLI lines, explained as data

The five newest CLI releases hold 63 lines (E4). The table groups them.

| Group | Lines | Reaches the loop? |
|---|---|---|
| Plugins, `agy plugin validate`, `/plugin`, marketplace, `agents.json` | many | No. The loop installs no agy plugin (A2). |
| Terminal and screen UI: `/diff`, colors, folding, links, keybindings, pinned prompt header | many | No. A person sees them. |
| Remote Control, Windows, Jujutsu, browser settings | several | No (A2). |
| Auth and models: Claude 5.5 with `AGY_ADC_AUTH`, `--model` without `--effort`, "Authentication expired" | 5 | Indirectly. The loop signs in as a person (A3). |
| Run behavior: stall cancel and retry, retry cap, `GIT_EDITOR=false`, headless `-p` subagent questions, `--agent`, `--print` exit codes, eligibility retry, background command fixes | the 10 of E5 | Yes, as CLI behavior (E5). |
| New modes and commands: Auto mode, `/compact`, `alt+m`, OSC 7501 status | 4 | No. A person starts them, or the tmux pane ignores them (E6). |

Worked example: v1.3.6 cancels a model request after 5 minutes without data and retries it. The loop watches panes by a transcript hash with a 600 s stall limit (E7). The CLI now clears a short stall first. No persona states a rule about this. The observer still signals a long stall.

## 4. Persona verdicts under ADR-0133

Rule: one persona per change, a 10-cycle baseline before and after (D7, D8, E8). Milestone M2 has not recorded the baseline (E9). So no persona changes in this cycle, and no persona changes after a reading of the lines alone.

| Persona | Verdict | Line that was checked |
|---|---|---|
| `evolve-scout` | No change. | `/compact` is a person command. The scout states no compaction rule. |
| `evolve-triage` | No change. | None applies. Triage reads inbox rank and `## top_n`. |
| `evolve-tdd-engineer` | No change. | `GIT_EDITOR=false` makes a bare `git commit` fail fast. A test author writes no commit. |
| `evolve-builder` | No change. First diet candidate (M3), only after M2. | Headless `-p` skips subagent questions. The builder asks none (E5). |
| `evolve-auditor` | No change. | The auditor runs on Claude Code only. No agy line reaches it. |
| `evolve-retrospective` | No change. | None applies. It reads the ledger and lessons. |

## 5. What can change the verdict

See `recommendation.md` for the flip conditions. Two matter most. A release may reword the usage-limit text that the manifest regex matches (E10). A release may change the `? for shortcuts` footer that marks a ready prompt (E10). Both are driver config, not persona text.

## 6. Files

`options/` holds three standalone strategies. `recommendation.md` compares them and names Option 1. `assumptions-and-evidence.md` holds A1 to A4 and E1 to E10. No file under `agents/` changes.
