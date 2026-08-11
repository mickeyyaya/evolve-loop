# Eval: compact-prompts-enable-default
<!-- cycle: 257 -->

Change the default for `EVOLVE_COMPACT_PROMPTS` from `false` to `true` in `runner.go`, activating the cycle-256 prompt-compaction feature in production. Opt-out via `EVOLVE_COMPACT_PROMPTS=0`. Register the key constant in `envchain/keys.go` and document in `docs/architecture/control-flags.md`.

## Acceptance Criteria

### 1. Unset env → Reference Index section stripped from disk-loaded agent body [code]

```bash
cd go && go test ./internal/phases/runner/... -run TestRun_CompactPrompts_DefaultOn -v 2>&1
# Expected: test exits 0; gotComposeBody does NOT contain "## Reference Index"
#           when EVOLVE_COMPACT_PROMPTS is unset (the new default-on behavior)
```

### 2. Negative case: opt-out via =0 restores full body [code]

```bash
cd go && go test ./internal/phases/runner/... -run TestRun_CompactPrompts_OptOut -v 2>&1
# Expected: test exits 0; with EVOLVE_COMPACT_PROMPTS=0, gotComposeBody equals
#           agentDocBody byte-for-byte (identical to the full body)
```

### 3. Inline prompt bodies never stripped (R7 preserved) [code]

```bash
cd go && go test ./internal/phases/runner/... -run TestRun_CompactPrompts_InlineBodyNotStripped -v 2>&1
# Expected: PASS — inline body with ## Reference Index is returned intact
#           even with compact mode default-on
```

### 4. Full runner test suite regression-free [code]

```bash
cd go && go test ./internal/phases/runner/... 2>&1 | tail -5
# Expected: ok github.com/mickeyyaya/evolve-loop/go/internal/phases/runner
```

### 5. Key registered in envchain/keys.go [code]

```bash
grep -q "EVOLVE_COMPACT_PROMPTS" go/internal/envchain/keys.go
# Expected: exit 0 — constant exists in the keys registry
```
