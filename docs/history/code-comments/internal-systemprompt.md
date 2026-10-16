# Comment history: `internal/systemprompt`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero k2

### `go/internal/systemprompt/systemprompt.go:1` — above `package systemprompt`

```text
// Package systemprompt resolves the launch-time system prompt / rules for an
// agent (facet B). It mirrors the resolvePolicy precedence chain so the
// per-agent and global env overrides behave identically to the interactive
// policy:
//
//	EVOLVE_<AGENT>_SYSTEM_PROMPT > EVOLVE_SYSTEM_PROMPT
//	  > profile.system_prompt > read(profile.digest_file) > read(profile.system_prompt_file) > ""
//
// digest_file (cycle-1391, tokenopt-role-scoped-instruction-digests Task 2)
// names a pre-generated role-scoped digest (go/internal/digest output). When
// set AND present on disk it wins over system_prompt_file; when unset, or
// set but the file is absent, the pre-cycle-1391 chain is unchanged.
```
