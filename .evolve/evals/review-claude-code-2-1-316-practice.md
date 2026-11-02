# Eval: review-claude-code-2-1-316-practice

## Graders

```bash
test -f docs/research/claude-code-changelog-2.1.316-review.md
```

```bash
grep -q 'https://raw.githubusercontent.com/anthropics/claude-code/main/CHANGELOG.md' docs/research/claude-code-changelog-2.1.316-review.md
```

```bash
grep -qi 'untrusted' docs/research/claude-code-changelog-2.1.316-review.md
```

```bash
grep -q 'ADR-0133' docs/research/claude-code-changelog-2.1.316-review.md
```

```bash
grep -q 'CLAUDE_CODE_SUBAGENT_EFFORT' docs/research/claude-code-changelog-2.1.316-review.md
```

```bash
grep -qiE 'no persona (text )?changes|none changes|no persona changes' docs/research/claude-code-changelog-2.1.316-review.md
```

```bash
grep -q 'claude-code-changelog-2.1.316-review.md' docs/research-index.md
```

```bash
go/evolve docs ste-lint --strict docs/research/claude-code-changelog-2.1.316-review.md
```
