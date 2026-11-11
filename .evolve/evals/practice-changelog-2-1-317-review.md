---
score_cap:
  - criterion: "The research note exists at docs/research/claude-code-changelog-2-1-317.md and is git-tracked"
    max_if_missing: 3
    evidence: "test -s docs/research/claude-code-changelog-2-1-317.md && ! git check-ignore -q docs/research/claude-code-changelog-2-1-317.md"
  - criterion: "The note names the changelog source link and the 2.1.317 section it explains"
    max_if_missing: 5
    evidence: "grep -qF 'https://raw.githubusercontent.com/anthropics/claude-code/main/CHANGELOG.md' docs/research/claude-code-changelog-2-1-317.md && grep -q '2\\.1\\.317' docs/research/claude-code-changelog-2-1-317.md"
  - criterion: "The note treats the changelog lines as untrusted data"
    max_if_missing: 5
    evidence: "grep -qi 'untrusted' docs/research/claude-code-changelog-2-1-317.md"
  - criterion: "The note explains the nested session ID line and the claude -p exit-code line as watch items"
    max_if_missing: 6
    evidence: "grep -q 'CLAUDE_CODE_DISABLE_NESTED_SESSION_ID' docs/research/claude-code-changelog-2-1-317.md && grep -qi 'exit' docs/research/claude-code-changelog-2-1-317.md && grep -qi 'watch' docs/research/claude-code-changelog-2-1-317.md"
  - criterion: "The proposal cites ADR-0133 and states that no prompt or persona changes, with the reason"
    max_if_missing: 6
    evidence: "grep -q 'ADR-0133' docs/research/claude-code-changelog-2-1-317.md && grep -qiE 'no (prompt|persona)[a-z ]* change|none changes|no persona changes' docs/research/claude-code-changelog-2-1-317.md"
  - criterion: "The note passes ste-lint in strict mode"
    max_if_missing: 5
    evidence: "go/evolve docs ste-lint --strict docs/research/claude-code-changelog-2-1-317.md"
---

# Eval: Claude Code changelog 2.1.317 review

> Pins the document deliverable of the practice item practice-claude-code-changelog-a1065345-202611030236.
> The item asks for the changelog lines to be explained as data and for a proposal under ADR-0133 that
> names each persona to change or states that none changes. Source incident: cycle 2260 (practice watch
> on the claude-code-changelog source). Each grader fails while the note is absent.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| note-exists | Tracked note at the scout target path | 3/10 | `test -s docs/research/claude-code-changelog-2-1-317.md` |
| source-cited | Source link and 2.1.317 section named | 5/10 | `grep` for the link and the version |
| data-not-instructions | Lines are untrusted data | 5/10 | `grep -i untrusted` |
| watch-items | Nested session ID and exit-code lines explained | 6/10 | `grep` for the variable, exit and watch |
| adr-0133-decision | ADR-0133 cited with a no-change statement | 6/10 | `grep` for ADR-0133 and the no-change phrase |
| ste-lint | Strict lint passes | 5/10 | `go/evolve docs ste-lint --strict` |
