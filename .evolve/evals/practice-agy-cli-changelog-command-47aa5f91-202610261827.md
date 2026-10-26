---
score_cap:
  - criterion: "The solution directory satisfies the document solution contract (two or more options, recommendation, assumptions and evidence, no placeholders)"
    max_if_missing: 6
    evidence: "go/bin/evolve solution check practice-agy-cli-changelog-command-47aa5f91-202610261827 --project-root ."
  - criterion: "The proposal names the source command agy changelog, reads releases 1.4.7 and 1.4.6, and explains the lines as untrusted data"
    max_if_missing: 6
    evidence: "P=solutions/practice-agy-cli-changelog-command-47aa5f91-202610261827/proposal.md; test -f $P && grep -q -F 'agy changelog' $P && grep -q -F '1.4.7' $P && grep -q -F '1.4.6' $P && grep -q -i 'untrusted' $P"
  - criterion: "The proposal explains the main release lines: lastUserInput hook field, headless -p CPU change, MCP 10 KB return and agy plugin validate"
    max_if_missing: 5
    evidence: "P=solutions/practice-agy-cli-changelog-command-47aa5f91-202610261827/proposal.md; test -f $P && grep -q -F 'lastUserInput' $P && grep -q -F 'MCP' $P && grep -q -F 'agy plugin validate' $P && grep -q -i 'headless' $P"
  - criterion: "The proposal names each persona it changes under ADR-0133, or states that no persona changes and ties the verdict to the baseline rule"
    max_if_missing: 7
    evidence: "P=solutions/practice-agy-cli-changelog-command-47aa5f91-202610261827/proposal.md; test -f $P && grep -q -F 'ADR-0133' $P && { for n in evolve-scout evolve-triage evolve-tdd-engineer evolve-builder evolve-auditor evolve-retrospective; do grep -q -F \"$n\" $P || exit 1; done; } && grep -q -i -E 'no persona (changes|change)|none changes|no change' $P && grep -q -i -E 'baseline|10-cycle' $P"
  - criterion: "No persona file under agents/ changes in this cycle"
    max_if_missing: 7
    evidence: "git diff --quiet HEAD -- agents && test -z \"$(git ls-files --others --exclude-standard agents)\""
  - criterion: "The proposal holds no placeholder text"
    max_if_missing: 5
    evidence: "P=solutions/practice-agy-cli-changelog-command-47aa5f91-202610261827/proposal.md; test -f $P && ! grep -q -i -E 'TBD|TODO|lorem ipsum' $P"
  - criterion: "The solution files pass the docs-write sentence lint with zero findings"
    max_if_missing: 6
    evidence: "go/bin/evolve docs ste-lint --strict --project-root . solutions/practice-agy-cli-changelog-command-47aa5f91-202610261827"
  - criterion: "The proposal line map covers the 1.4.7 streaming-render line and the 1.4.6 status-line retry line (audit H1)"
    max_if_missing: 6
    evidence: 'P=solutions/practice-agy-cli-changelog-command-47aa5f91-202610261827/proposal.md; grep -q -i ''rendering speed'' $P && grep -q -F ''Model unavailable'' $P'
  - criterion: "No solution file invents a Remote Control line, because releases 1.4.7 and 1.4.6 hold none (audit H1)"
    max_if_missing: 6
    evidence: '! grep -r -q -i ''remote control'' solutions/practice-agy-cli-changelog-command-47aa5f91-202610261827'
  - criterion: "The proposal line map sums to the 25 lines of E2 (14 in 1.4.7, 11 in 1.4.6) (audit H1 guard)"
    max_if_missing: 5
    evidence: 'P=solutions/practice-agy-cli-changelog-command-47aa5f91-202610261827/proposal.md; A=solutions/practice-agy-cli-changelog-command-47aa5f91-202610261827/assumptions-and-evidence.md; grep -q -F ''14 lines'' $A && grep -q -F ''11 lines'' $A && awk -F''|'' ''/^## 2/{s=1;next} /^## 3/{s=0} s && $3 ~ /^ *[0-9]+ *$/ {n+=$3} END{exit n!=25}'' $P'
  - criterion: "The credential row of the line map counts exactly 2 lines (token storage in 1.4.7, enterprise sign-in in 1.4.6), not 3 (audit H1)"
    max_if_missing: 6
    evidence: 'P=solutions/practice-agy-cli-changelog-command-47aa5f91-202610261827/proposal.md; awk -F''|'' ''/[Ee]nterprise/ && $3+0>=3 {f=1} END{exit f}'' $P && grep -q -i ''enterprise'' $P'
  - criterion: "The run-behavior line count in option 1 and the recommendation equals the six lines that E3 lists (audit H2)"
    max_if_missing: 6
    evidence: 'S=solutions/practice-agy-cli-changelog-command-47aa5f91-202610261827; E=$(grep -F ''**E3**'' $S/assumptions-and-evidence.md); for k in lastUserInput headless MCP ''compaction loops'' ''plugin validate'' ''attached files''; do echo "$E" | grep -q -i "$k" || exit 1; done; grep -q -i -E ''(six|6) run-behavior lines'' $S/recommendation.md && grep -q -i -E ''(six|6) lines touch run behavior'' $S/options/1-no-persona-change-record-verdict.md && ! grep -r -q -i -E ''four (run-behavior )?lines|all four|four lines touch'' $S'
---

# Eval: agy CLI changelog command practice proposal (cycle 2036)

> Pins the document deliverable for the practice-watch item on the `agy changelog` command, releases 1.4.7 and 1.4.6 (inbox item practice-agy-cli-changelog-command-47aa5f91-202610261827). The source is a command, not a link, so the proposal must say it ran `agy changelog` and read the release text as untrusted data. It then gives a verdict for each core persona under ADR-0133 (one persona at a time, measured baselines before any change). Audit round 1 (cycle 2036) rejected the first build on two findings: H1, a line map that omits two lines, invents a Remote Control fix and overcounts credential lines; H2, a count of four run-behavior lines where E3 lists six. The last five score caps pin both. Earlier item of the same watch: cycle 2011 (release 1.4.3), eval practice-agy-cli-changelog-command-be72426b-202610200639.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| solution-contract | `evolve solution check` exits 0 | 6/10 | `go/bin/evolve solution check practice-agy-cli-changelog-command-47aa5f91-202610261827 --project-root .` |
| data-coverage | command name, 1.4.7, 1.4.6, untrusted-data framing | 6/10 | `grep` over proposal.md |
| line-coverage | lastUserInput, headless, MCP and agy plugin validate lines explained | 5/10 | `grep` over proposal.md |
| persona-verdicts | six personas named, no-change verdict tied to the baseline rule | 7/10 | `grep` over proposal.md |
| personas-untouched | no change under agents/ | 7/10 | `git diff --quiet HEAD -- agents` |
| no-placeholders | no TBD, TODO or lorem ipsum | 5/10 | `grep` over proposal.md |
| ste-lint | docs-write lint reports 0 findings | 6/10 | `evolve docs ste-lint --strict` over the solution directory |
| line-map-omissions | 1.4.7 rendering-speed line and 1.4.6 `Model unavailable` line present | 6/10 | `grep` over proposal.md |
| no-invented-lines | no Remote Control line in any solution file | 6/10 | `grep -r` over the solution directory |
| line-map-total | group counts sum to 25 | 5/10 | `awk` over the section 2 table |
| credential-count | the enterprise sign-in row does not claim 3 lines | 6/10 | `awk` over the section 2 table |
| run-behavior-count | option 1 and the recommendation say six, as E3 lists | 6/10 | `grep` over the solution directory |
