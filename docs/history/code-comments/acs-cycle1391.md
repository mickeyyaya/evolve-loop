# Comment history: `acs/cycle1391`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1391/predicates_test.go:3` — above `package cycle1391`

```text
// Package cycle1391 ports the cycle-1391 ACS predicates for the
// role-scoped-instruction-digest-generator lane (inbox item
// tokenopt-role-scoped-instruction-digests).
//
// Two tasks, one fleet-scoped item:
//
//   - digest-projector-core: a new go/internal/digest package. ProjectDigest
//     scans an SSOT skill/instruction source for
//     "<!-- digest:role=ROLE[,ROLE2,...] -->...<!-- /digest -->" marker pairs
//     and returns the concatenated content of every block whose role list
//     contains the requested role. Untagged content and blocks tagged for
//     other roles are excluded — no hand-maintained duplicate copy.
//   - digest-wire-scout-profile: go/internal/systemprompt.Resolve gains a
//     new profile field "digest_file" (resolved relative to profileDir like
//     system_prompt_file). When set AND the file exists on disk, its content
//     wins over system_prompt_file. When digest_file is unset, or set but the
//     file is absent, the existing 4-tier precedence chain
//     (env > profile.system_prompt > system_prompt_file > "") is unchanged.
```

### `go/acs/cycle1391/predicates_test.go:180` — above `func TestC1391_007_ResolveUnchangedWhenDigestFileUnset(t *testing.T) {`

```text
// TestC1391_007_ResolveUnchangedWhenDigestFileUnset is the no-digest_shape
// regression predicate: a profile that never sets digest_file at all must
// behave exactly like the pre-cycle-1391 precedence chain (inline
// system_prompt wins, else system_prompt_file, else "").
```
