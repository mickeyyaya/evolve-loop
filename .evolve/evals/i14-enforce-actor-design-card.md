---
score_cap:
  - criterion: "The solution contract holds for the I14 PR (c) document deliverable"
    max_if_missing: 4
    evidence: "./go/evolve solution check solutions/wave-integration-i14-driver-pr-c"
  - criterion: "The EnforceActor design card exists, is non-empty and passes strict ste-lint"
    max_if_missing: 5
    evidence: "test -s docs/architecture/integration-driver-enforce.md && ./go/evolve docs ste-lint --strict docs/architecture/integration-driver-enforce.md"
  - criterion: "The card decides each section 9 open input: F2, F3, F11, F12 and Findings 3, 4, 5"
    max_if_missing: 5
    evidence: "for k in F2 F3 F11 F12 'Finding 3' 'Finding 4' 'Finding 5'; do grep -q \"^#\\+ .*$k\" docs/architecture/integration-driver-enforce.md || exit 1; done"
  - criterion: "AC1: the journey names a fake CI and a bare origin, one fast-forward landing and one conflict requeued with its brief"
    max_if_missing: 5
    evidence: "f=docs/architecture/integration-driver-enforce.md; grep -qi 'fake CI' $f && grep -qi 'bare origin' $f && grep -qi 'fast-forward' $f && grep -qi 'requeue' $f && grep -qi 'brief' $f"
  - criterion: "AC2: the card holds a delivered item off the menu so no second lane starts on it"
    max_if_missing: 5
    evidence: "f=docs/architecture/integration-driver-enforce.md; grep -qi 'second lane' $f && grep -qi 'delivered' $f && grep -qi 'stale-claim' $f"
  - criterion: "AC3: the card makes evolve loop --resume of a lane checkpoint refuse or re-identify at enforce"
    max_if_missing: 5
    evidence: "f=docs/architecture/integration-driver-enforce.md; grep -q 'evolve loop --resume' $f && grep -qi 'checkpoint' $f && grep -qi 'lane identity' $f"
  - criterion: "The card defines EnforceActor with its Actor methods Resume, Flush and WaveEnd"
    max_if_missing: 5
    evidence: "f=docs/architecture/integration-driver-enforce.md; grep -q 'EnforceActor' $f && grep -q 'Resume' $f && grep -q 'Flush' $f && grep -q 'WaveEnd' $f"
  - criterion: "Section 9 of integration-driver.md links to the card"
    max_if_missing: 4
    evidence: "sed -n '/^## 9\\./,/^## 10\\./p' docs/architecture/integration-driver.md | grep -q 'integration-driver-enforce.md'"
  - criterion: "Negative: the card has no placeholder text"
    max_if_missing: 4
    evidence: "test -s docs/architecture/integration-driver-enforce.md && ! grep -q -i -E 'TBD|TODO|FIXME|lorem ipsum' docs/architecture/integration-driver-enforce.md"
  - criterion: "Negative: this card cycle changes no driver code (code follows in a later cycle)"
    evidence: "git diff --quiet HEAD -- go/internal/integration go/internal/phases/ship go/cmd/evolve/cmd_loop_wavesync.go && test -z \"$(git ls-files --others --exclude-standard -- go/internal/integration go/internal/phases/ship go/cmd/evolve/cmd_loop_wavesync.go)\""
    max_if_missing: 6
---

# Eval: I14 PR (c) enforce actor design card (cycle 2361)

> Grades the document deliverable of inbox item `wave-integration-i14-driver-pr-c`. The item asks for a design card that passes architecture review before any code. The card must decide each open input of section 9 in `docs/architecture/integration-driver.md` and meet the three acceptance criteria of the item. The solution directory is the deterministic document floor of ADR-0099. Source incident: PR (b) landed shadow only, and nothing lands a lane at enforce.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| solution-floor | `evolve solution check` exits 0 | 4/10 | solution contract engine |
| card-lint | Card exists and passes strict ste-lint | 5/10 | `evolve docs ste-lint --strict` |
| section-9-coverage | One decision heading per open input | 5/10 | heading grep |
| ac1-journey | Fake CI, bare origin, fast-forward, requeue with brief | 5/10 | term grep on card |
| ac2-delivered | Delivered item held off the menu and the stale-claim sweep | 5/10 | term grep on card |
| ac3-resume | Resume of a lane checkpoint cannot land on main at enforce | 5/10 | term grep on card |
| actor-surface | EnforceActor defines Resume, Flush and WaveEnd | 5/10 | term grep on card |
| link | Section 9 links to the card | 4/10 | sed range and grep |
| no-placeholders | No TBD, TODO or FIXME | 4/10 | negative grep |
| code-fence | No driver code changed this cycle | 6/10 | `git diff` deny-list |

## Code Graders (bash commands that must exit 0)
- `[code]` `./go/evolve solution check solutions/wave-integration-i14-driver-pr-c`
- `[code]` `test -s docs/architecture/integration-driver-enforce.md && ./go/evolve docs ste-lint --strict docs/architecture/integration-driver-enforce.md`
- `[code]` `for k in F2 F3 F11 F12 'Finding 3' 'Finding 4' 'Finding 5'; do grep -q "^#\+ .*$k" docs/architecture/integration-driver-enforce.md || exit 1; done`
- `[code]` `f=docs/architecture/integration-driver-enforce.md; grep -qi 'fake CI' $f && grep -qi 'bare origin' $f && grep -qi 'fast-forward' $f && grep -qi 'requeue' $f && grep -qi 'brief' $f`
- `[code]` `f=docs/architecture/integration-driver-enforce.md; grep -qi 'second lane' $f && grep -qi 'delivered' $f && grep -qi 'stale-claim' $f`
- `[code]` `f=docs/architecture/integration-driver-enforce.md; grep -q 'evolve loop --resume' $f && grep -qi 'checkpoint' $f && grep -qi 'lane identity' $f`
- `[code]` `f=docs/architecture/integration-driver-enforce.md; grep -q 'EnforceActor' $f && grep -q 'Resume' $f && grep -q 'Flush' $f && grep -q 'WaveEnd' $f`
- `[code]` `sed -n '/^## 9\./,/^## 10\./p' docs/architecture/integration-driver.md | grep -q 'integration-driver-enforce.md'`
- `[code]` `test -s docs/architecture/integration-driver-enforce.md && ! grep -q -i -E 'TBD|TODO|FIXME|lorem ipsum' docs/architecture/integration-driver-enforce.md`
- `[code]` `git diff --quiet HEAD -- go/internal/integration go/internal/phases/ship go/cmd/evolve/cmd_loop_wavesync.go && test -z "$(git ls-files --others --exclude-standard -- go/internal/integration go/internal/phases/ship go/cmd/evolve/cmd_loop_wavesync.go)"`

## Acceptance Criteria (code-graded)

### AC1: journey with a fake CI and a bare origin [code]
Lands one member by fast-forward and requeues a conflict with its brief. Graders 4.

### AC2: no second lane on a delivered item [code]
Graders 5.

### AC3: resume of a lane checkpoint cannot land on main at enforce [code]
Graders 6.
