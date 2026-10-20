---
score_cap:
  - criterion: "The solution directory satisfies the document solution contract (two or more options, recommendation, assumptions and evidence, no placeholders)"
    max_if_missing: 6
    evidence: "go/bin/evolve solution check practice-agy-cli-changelog-command-be72426b-202610200639 --project-root ."
  - criterion: "The proposal names the source command agy changelog, reads version 1.4.3, and explains the /diff search, agy plugin validate, subagent compaction summary, PostToolUse hook and Argon lines as untrusted data"
    max_if_missing: 6
    evidence: "P=solutions/practice-agy-cli-changelog-command-be72426b-202610200639/proposal.md; test -f $P && grep -q -F 'agy changelog' $P && grep -q -F '1.4.3' $P && grep -q -F '/diff' $P && grep -q -F 'agy plugin validate' $P && grep -q -F 'PostToolUse' $P && grep -q -F 'Argon' $P && grep -q -i 'compact' $P && grep -q -i 'untrusted' $P"
  - criterion: "The proposal names each persona it changes under ADR-0133, or states that no persona changes and ties the verdict to the baseline rule"
    max_if_missing: 7
    evidence: "P=solutions/practice-agy-cli-changelog-command-be72426b-202610200639/proposal.md; test -f $P && grep -q -F 'ADR-0133' $P && { for n in evolve-scout evolve-triage evolve-tdd-engineer evolve-builder evolve-auditor evolve-retrospective; do grep -q -F \"$n\" $P || exit 1; done; } && grep -q -i -E 'no persona (changes|change)|none changes|no change' $P && grep -q -i -E 'baseline|10-cycle' $P"
  - criterion: "No persona file under agents/ changes in this cycle"
    max_if_missing: 7
    evidence: "git diff --quiet HEAD -- agents && test -z \"$(git ls-files --others --exclude-standard agents)\""
  - criterion: "The proposal holds no placeholder text"
    max_if_missing: 5
    evidence: "P=solutions/practice-agy-cli-changelog-command-be72426b-202610200639/proposal.md; test -f $P && ! grep -q -i -E 'TBD|TODO|lorem ipsum' $P"
  - criterion: "The solution files pass the docs-write sentence lint with zero findings"
    max_if_missing: 6
    evidence: "go/bin/evolve docs ste-lint --strict --project-root . solutions/practice-agy-cli-changelog-command-be72426b-202610200639"
  - criterion: "The solution does not claim the loop uses no agy plugin command: the repo runs agy plugin validate and agy plugin install (go/cmd/evolve/cmd_skills_publish.go), and the evidence cites that file"
    max_if_missing: 7
    evidence: "D=solutions/practice-agy-cli-changelog-command-be72426b-202610200639; grep -q -F 'agy plugin validate' go/cmd/evolve/cmd_skills_publish.go && grep -q -F cmd_skills_publish.go $D/assumptions-and-evidence.md && ! grep -q -i -E 'installs no agy plugin|uses no .?agy plugin|no agy plugin command|no agy plugin' $D/assumptions-and-evidence.md $D/proposal.md $D/recommendation.md $D/options/*.md"
  - criterion: "The A3 assumption names the variables the agy manifest really guards (GEMINI_API_KEY and GOOGLE_API_KEY) and does not say the manifest guards AGY_ADC_AUTH"
    max_if_missing: 6
    evidence: "D=solutions/practice-agy-cli-changelog-command-be72426b-202610200639; grep -q -F GOOGLE_API_KEY go/internal/bridge/manifests/agy.json && ! grep -q -F AGY_ADC_AUTH go/internal/bridge/manifests/agy.json && grep -E '^- \*\*A3\*\*' $D/assumptions-and-evidence.md | grep -q -F GOOGLE_API_KEY && ! grep -E 'AGY_ADC_AUTH.*guard|guard.*AGY_ADC_AUTH' $D/assumptions-and-evidence.md"
  - criterion: "Option 3 and the flip conditions name the live skills-publish agy plugin validate target that E8 proves (audit round 2, M1)"
    max_if_missing: 6
    evidence: "D=solutions/practice-agy-cli-changelog-command-be72426b-202610200639; grep -q -F 'agy plugin validate' $D/options/3-driver-config-check-and-follow-up-item.md && grep -q -F 'agy plugin validate' $D/recommendation.md && grep -q -F 'cmd_skills_publish.go' $D/options/3-driver-config-check-and-follow-up-item.md"
  - criterion: "The proposal file list and the explanation document cite the evidence range E1 to E8 and none of them says E1 to E7 (audit round 2, L1 and L2)"
    max_if_missing: 5
    evidence: "D=solutions/practice-agy-cli-changelog-command-be72426b-202610200639; grep -q -F 'E1 to E8' $D/proposal.md && ! grep -q -F 'E1 to E7' $D/proposal.md && grep -q -E 'E1.E8' docs/explain/builds/cycle-2011-01m5c0nq5e21q1xkgqa5jzbf49.md && ! grep -q -E 'E1.E7' docs/explain/builds/cycle-2011-01m5c0nq5e21q1xkgqa5jzbf49.md"
  - criterion: "The explanation document does not claim the eval graders pass without a count: its Verification section states the score_cap graders that were run and that all 11 exit 0 (audit round 3, M1)"
    max_if_missing: 5
    evidence: "E=docs/explain/builds/cycle-2011-01m5c0nq5e21q1xkgqa5jzbf49.md; ! grep -q -F 'The eval grep graders pass' $E && grep -q -E '11 of 11|11/11' $E"
---

# Eval: agy CLI changelog command practice proposal (cycle 2011)

> Pins the document deliverable for the practice-watch item on the `agy changelog` command, release 1.4.3 (inbox item practice-agy-cli-changelog-command-be72426b-202610200639). The source is a command, not a link, so the proposal must say it ran `agy changelog` and read the release text as untrusted data. It explains the /diff search, `agy plugin validate`, subagent compaction summary, PostToolUse hook failure and Argon model lines, then gives a verdict for each core persona under ADR-0133 (one persona at a time, measured baselines before any change). Earlier items of the same watch: docs/research/practice-agy-cli-changelog-2026-10.md and the cycle 1996 eval for the page tab.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| solution-contract | `evolve solution check` exits 0 | 6/10 | `go/bin/evolve solution check practice-agy-cli-changelog-command-be72426b-202610200639 --project-root .` |
| data-coverage | command name, 1.4.3, five line groups, untrusted-data framing | 6/10 | `grep` over proposal.md |
| persona-verdicts | six personas named, no-change verdict tied to the baseline rule | 7/10 | `grep` over proposal.md |
| personas-untouched | no change under agents/ | 7/10 | `git diff --quiet HEAD -- agents` |
| no-placeholders | no TBD, TODO or lorem ipsum | 5/10 | `grep` over proposal.md |
| ste-lint | docs-write lint reports 0 findings | 6/10 | `evolve docs ste-lint --strict` over the solution directory |
| plugin-claim-true | the solution does not say the loop runs no agy plugin command; the evidence cites cmd_skills_publish.go (audit round 1, H1) | 7/10 | `grep` over the solution files and `cmd_skills_publish.go` |
| a3-guard-vars | A3 names GEMINI_API_KEY and GOOGLE_API_KEY, the guards in agy.json, and not AGY_ADC_AUTH (audit round 1, L1) | 6/10 | `grep` over assumptions-and-evidence.md and agy.json |
| ste-lint-strict-pass | every sentence of the solution has 25 words or fewer; G6 stays the grader for this and the A2 basis sentence of 26 words keeps it RED (audit round 2, H1) | 6/10 | `evolve docs ste-lint --strict` |
| skills-publish-target | Option 3 and the recommendation name the skills-publish `agy plugin validate` target of E8 (audit round 2, M1) | 6/10 | `grep` over options/3 and recommendation.md |
| evidence-range | proposal.md and the explanation document say E1 to E8, not E1 to E7 (audit round 2, L1 and L2) | 5/10 | `grep` over proposal.md and the explanation document |
| explanation-verification-true | the explanation document states a count of graders run (11 of 11) in place of the unqualified "graders pass" claim (audit round 3, M1) | 5/10 | `grep` over the explanation document |
