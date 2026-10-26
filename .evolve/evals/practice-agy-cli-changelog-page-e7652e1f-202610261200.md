---
score_cap:
  - criterion: "The solution directory satisfies the document solution contract (two or more options, recommendation, assumptions and evidence, no placeholders)"
    max_if_missing: 6
    evidence: "go/bin/evolve solution check practice-agy-cli-changelog-page-e7652e1f-202610261200 --project-root ."
  - criterion: "The proposal cites the changelog source link, names the versions read at the link, and explains the page text as untrusted data"
    max_if_missing: 6
    evidence: "P=solutions/practice-agy-cli-changelog-page-e7652e1f-202610261200/proposal.md; test -f $P && grep -q -F 'https://antigravity.google/docs/changelog?tab=cli' $P && grep -q -F 'v2.26.0' $P && grep -q -F 'v2.25.0' $P && grep -q -i 'untrusted' $P"
  - criterion: "The proposal names each persona it changes under ADR-0133, or states that no persona changes and ties the verdict to the baseline rule"
    max_if_missing: 7
    evidence: "P=solutions/practice-agy-cli-changelog-page-e7652e1f-202610261200/proposal.md; test -f $P && grep -q -F 'ADR-0133' $P && { for n in evolve-scout evolve-triage evolve-tdd-engineer evolve-builder evolve-auditor evolve-retrospective; do grep -q -F \"$n\" $P || exit 1; done; } && grep -q -i -E 'no persona (changes|change)|none changes|no change' $P && grep -q -i -E 'baseline|10-cycle' $P"
  - criterion: "No persona file under agents/ changes in this cycle"
    max_if_missing: 7
    evidence: "git diff --quiet HEAD -- agents && test -z \"$(git ls-files --others --exclude-standard agents)\""
  - criterion: "The proposal holds no placeholder text"
    max_if_missing: 5
    evidence: "P=solutions/practice-agy-cli-changelog-page-e7652e1f-202610261200/proposal.md; test -f $P && ! grep -q -i -E 'TBD|TODO|lorem ipsum' $P"
  - criterion: "The solution files pass the docs-write sentence lint with zero findings"
    max_if_missing: 6
    evidence: "go/bin/evolve docs ste-lint --strict --project-root . solutions/practice-agy-cli-changelog-page-e7652e1f-202610261200"
  - criterion: "The solution places v2.26.0 and v2.25.0 under the Antigravity App tab, names v1.4.0 (October 16, 2026) as the latest CLI-tab release, and claims no CLI-binary behavior from the App fixes (audit H1)"
    max_if_missing: 6
    evidence: "D=solutions/practice-agy-cli-changelog-page-e7652e1f-202610261200; P=$D/proposal.md; E=$D/assumptions-and-evidence.md; grep -q -F 'Antigravity App' $P && grep -q -F 'v1.4.0' $P && grep -q -F 'v1.4.0' $E && grep -q -F 'October 16, 2026' $E && ! grep -r -q -i -E 'CLI tab (lists|shows) v2|as internal CLI binary behavior|CLI binary (already )?handles' $D"
  - criterion: "The solution states the agy manifest passes GOOGLE_APPLICATION_CREDENTIALS through, and the manifest still does (audit M1)"
    max_if_missing: 6
    evidence: "D=solutions/practice-agy-cli-changelog-page-e7652e1f-202610261200; P=$D/proposal.md; E=$D/assumptions-and-evidence.md; grep -q -F GOOGLE_APPLICATION_CREDENTIALS $E && grep -q -F GOOGLE_APPLICATION_CREDENTIALS go/internal/bridge/manifests/agy-tmux.json && ! grep -r -q -i -E 'sets no ADC|no ADC (variables|environment)|AGY_ADC_AUTH|passes .--model. only' $D"
  - criterion: "The proposal explains the slash-command lines /agents to /voice as docs site navigation data (audit M2)"
    max_if_missing: 6
    evidence: "D=solutions/practice-agy-cli-changelog-page-e7652e1f-202610261200; P=$D/proposal.md; E=$D/assumptions-and-evidence.md; grep -q -F '/agents' $P && grep -q -F '/plan' $P && grep -q -F '/voice' $P && grep -q -i 'navigation' $P"
---

# Eval: agy CLI changelog page practice proposal (cycle 2033)

> Pins the document deliverable for the practice-watch item on the agy CLI changelog page (inbox item practice-agy-cli-changelog-page-e7652e1f-202610261200). The page now lists releases v2.26.0 and v2.25.0 that the proposal reads at the link and explains as untrusted data. The proposal gives a verdict for each core persona under ADR-0133 (one persona at a time, measured baselines before any change). Source incident: the earlier items of the same watch in docs/research/practice-agy-cli-changelog-page-2026-10.md; the eval slug is the exact inbox id.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| solution-contract | `evolve solution check` exits 0 | 6/10 | `go/bin/evolve solution check practice-agy-cli-changelog-page-e7652e1f-202610261200 --project-root .` |
| data-coverage | source link, both versions, untrusted-data framing | 6/10 | `grep` over proposal.md |
| persona-verdicts | six personas named, no-change verdict tied to the baseline rule | 7/10 | `grep` over proposal.md |
| personas-untouched | no change under agents/ | 7/10 | `git diff --quiet HEAD -- agents` |
| no-placeholders | no TBD, TODO or lorem ipsum | 5/10 | `grep` over proposal.md |
| ste-lint | docs-write lint reports 0 findings | 6/10 | `evolve docs ste-lint --strict` over the solution directory |
| app-vs-cli-tab | v2.26.0 and v2.25.0 are App releases, v1.4.0 is the latest CLI release, no false CLI-binary claim (audit H1, round 1) | 6/10 | `grep` over proposal.md, assumptions-and-evidence.md and the option files |
| adc-passthrough | the manifest passes GOOGLE_APPLICATION_CREDENTIALS and the evidence says so (audit M1, round 1) | 6/10 | `grep` over assumptions-and-evidence.md and `agy-tmux.json` |
| slash-command-lines | /agents to /voice explained as docs site navigation (audit M2, round 1) | 6/10 | `grep` over proposal.md |
