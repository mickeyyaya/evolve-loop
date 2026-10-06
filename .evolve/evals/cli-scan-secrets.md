---
score_cap:
  - criterion: "evolve scan secrets (staged diff by default, or --staged) over a staged AWS-key-shaped string exits 1 and prints <file>:<line>: aws-access-key-id: AKIA**************** with the raw key absent from stdout and stderr; a repo with no commit yet is scanned against the empty tree"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1805_001' ./acs/cycle1805"
  - criterion: "a clean staged change and an empty index exit 0 with only a scan secrets: PASS summary; an unstaged key is not part of the staged diff"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1805_002' ./acs/cycle1805"
  - criterion: "--diff main...HEAD (and --diff=main...HEAD) scans exactly that committed range, names it in the summary, and never reads the index"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1805_003' ./acs/cycle1805"
  - criterion: "git I/O failures (not a repo, missing --project-root, bad revision) exit 2 with an empty stdout and an evolve scan secrets: git diff: stderr line, so no false PASS is ever printed"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1805_004' ./acs/cycle1805"
  - criterion: "usage errors exit 10 before any git runs, a --diff value starting with - is refused (no option injection reaches git), and --help/-h exit 0"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1805_005' ./acs/cycle1805"
  - criterion: "every detector rule's match is masked rune-preserving (first 4 runes kept) and the printed report fed back through ScanDiff yields no finding"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1805_006' ./acs/cycle1805"
  - criterion: "secretleakscan.ScanDiff attaches the post-image File and Line to each finding for every diff-header shape (count left out, removed and no-newline lines, trailing-TAB and quoted paths, missing or garbage hunk headers) without changing the Rule/Match sequence"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1805_007' ./acs/cycle1805"
  - criterion: "the scan is read-only (HEAD, index, refs, stash, working tree and .evolve untouched) and byte-deterministic"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1805_008' ./acs/cycle1805"
  - criterion: "the scanned repo is the cwd or --project-root, never EVOLVE_PROJECT_ROOT"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1805_009' ./acs/cycle1805"
  - criterion: "evolve's top-level usage lists scan secrets and the registry dispatches it"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1805_010' ./acs/cycle1805"
  - criterion: "runtime-reference.md documents evolve scan secrets under Operator commands with its flags, exit codes and masking, matching the binary's --help"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1805_011' ./acs/cycle1805"
  - criterion: "the /commit skill's step 4 runs evolve scan secrets before commit-gate run, keeps steps 1..6, and its invocation works on a dirty and a clean repo"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1805_012' ./acs/cycle1805"
  - criterion: "the lane adds no comments to go/cmd/evolve or secretleakscan, grows no function past the size ratchet, and keeps secretleakscan standard-library only"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1805_013' ./acs/cycle1805"
---

# Eval: evolve scan secrets

> Pins `evolve scan secrets [--staged | --diff <ref>] [--project-root P]`
> (inbox item `cli-scan-secrets`, cycle 1805). `secretleakscan.ScanDiff` had no
> production caller: the secret-leak-scan phase runs only inside a cycle, so a
> console change reached `evolve ship --class manual` with no deterministic
> secret check. The verb scans the staged diff (or a ref range), prints each
> finding as `<file>:<line>: <rule>: <masked match>` (for example
> `config/aws.go:2: aws-access-key-id: AKIA****************`), exits 0 clean,
> 1 on a finding, 2 on git I/O and 10 on usage, and the /commit skill runs it
> first in step 4. The worst failure for a secret check is a false PASS, so the
> eval caps hardest on the exit-1 path, the exit-2 path that prints no verdict,
> and the rule that EVOLVE_PROJECT_ROOT never redirects the scan.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| staged-finding | staged key exits 1, masked file:line:rule, raw key never printed | 3/10 | `TestC1805_001` |
| clean-pass | clean/empty staged diff exits 0 PASS | 4/10 | `TestC1805_002` |
| range | `--diff main...HEAD` scans that range only, not the index | 4/10 | `TestC1805_003` |
| no-false-clean | git failures exit 2 with no verdict on stdout | 4/10 | `TestC1805_004` |
| usage-and-injection | usage exits 10 before git; `--diff -…` refused | 6/10 | `TestC1805_005` |
| masking | every rule masked rune-preserving; report never re-trips the scanner | 4/10 | `TestC1805_006` |
| location | ScanDiff fills File/Line for every header shape, detection unchanged | 5/10 | `TestC1805_007` |
| read-only | no write to git state or .evolve; deterministic stdout | 6/10 | `TestC1805_008` |
| root-selection | cwd or `--project-root`, never EVOLVE_PROJECT_ROOT | 5/10 | `TestC1805_009` |
| wiring | usage lists `scan`; registry dispatches it | 7/10 | `TestC1805_010` |
| docs | runtime-reference bullet matches `--help` | 7/10 | `TestC1805_011` |
| commit-skill | step 4 runs the scan first; its invocation works | 6/10 | `TestC1805_012` |
| conventions | no comments, size ratchet, scanner stays a stdlib leaf | 7/10 | `TestC1805_013` |
