# Recommendation — landing-s1-readonly-redesign-lane

Every number below cites `assumptions-and-evidence.md`.

## Options Compared

| Criterion | Option 1: keep S1 dropped | Option 2: probe plus recovery fix | Option 3: pre-screen in core |
|---|---|---|---|
| Meets acceptance 1 (read-only, no stage, no consume) | yes, no check exists | yes for the probe | yes |
| Meets acceptance 2 (test) | not applicable (E5, branch 3) | needs a new test | needs a new test |
| Meets acceptance 3 | yes (E1, E5) | not needed | not needed |
| Files changed | 1 plan document | 1 protected recovery file (E3) plus a probe | core files, protected (E2) |
| Tamper guard kept (E4) | yes | yes | yes |
| Saves a scan for HEAD == base lanes (E2) | no, the cache does (A2) | only with the recovery fix (A1) | only with the staged-content classifier, since its rebase path is the recovery that discards HEAD == base lanes (E3, A1) |
| Cost | one edit | at least one console cycle | more than one console cycle |

## Recommendation

**Adopt Option 1: S1 stays dropped, and the plan records why.**

- **Winner:** Option 1. It meets acceptance item 3 in full (E5) and removes a check that failed two review
  rounds (E6). S5 and S6 save the scan (E1).
- **Runner-up:** Option 3, because the probe lives next to the base SHA and leaves ship unchanged.
- **Evidence that would flip the choice:** the S13 simulation shows the cache (S5, S6) leaves more than
  a small share of scan time unsaved for diverged lanes (A2), and a test shows that a staged-content
  classifier separates superseded lanes from rebase-needed lanes (A1).
- **Honest limits:** A1 and A2 are not measured. The code half sits on protected ship and core files that
  stay with the console, so this lane did not edit them.
