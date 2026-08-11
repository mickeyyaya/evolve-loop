---
score_cap:
  - criterion: "profiles.Loader expands $include_policy references in disallowed_tools before returning a Profile"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run TestLoader_ExpandsIncludePolicy ./internal/profiles/"
  - criterion: "builder.json and auditor.json disallowed_tools lists are ≤40% of their current length (shared entries extracted)"
    max_if_missing: 3
    evidence: "jq '.disallowed_tools | length' .evolve/profiles/builder.json .evolve/profiles/auditor.json"
  - criterion: "Merged disallowed_tools at runtime equals the union of shared policy + profile-specific entries"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 ./internal/profiles/..."
---

# Eval: Profile tool-policy deduplication

`builder.json` and `auditor.json` share ~14 identical `disallowed_tools` entries — the inline
script execution denial list (`perl`, `ruby`, `python3 -c`, `node -e`, `osascript`, `sh -c`,
`bash -c`, `zsh -c`, `env`, `exec`, `eval`, `awk`, `unlink`, `ln`). Both also share nearly
identical `extra_flags_by_cli.claude-tmux` blocks.

The fix: introduce a `$include_policy: "no-inline-script-exec"` reference syntax in the
`disallowed_tools` array, backed by a `.evolve/profiles/tool-policy.json` that defines named
policy sets. The `profiles.Loader.Get()` expands `$include_policy:*` entries inline.

## Graders

### [code] Loader.Get expands $include_policy references before returning Profile

```bash
cd go && go test -count=1 -run TestLoader_ExpandsIncludePolicy ./internal/profiles/ 2>&1 | tail -3
# expected: PASS
```

### [code] builder.json disallowed_tools list is shorter than original (shared entries removed)

```bash
original_count=34  # number before dedup
new_count=$(jq '.disallowed_tools | length' .evolve/profiles/builder.json)
echo "builder disallowed_tools: $new_count (was $original_count)"
test "$new_count" -lt 20 && echo "PASS: reduced to $new_count" || echo "FAIL: $new_count >= 20"
```

### [code] Merged profile at runtime still contains the shared entries

```bash
cd go && go test -count=1 -run TestLoader_MergedDisallowedToolsContainsShared ./internal/profiles/ 2>&1 | tail -3
# expected: PASS
```

### [code] Negative: missing policy name in $include_policy returns an error, not silent empty

```bash
cd go && go test -count=1 -run TestLoader_UnknownPolicy_ReturnsError ./internal/profiles/ 2>&1 | tail -3
# expected: PASS
```

### [code] Full profiles package test suite green

```bash
cd go && go test -count=1 ./internal/profiles/... 2>&1 | grep -E "^(ok|FAIL)"
# expected: ok
```
