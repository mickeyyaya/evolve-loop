# Comment history: `internal/ledgerverify`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d6

### `go/internal/ledgerverify/verify.go:1` — above `package ledgerverify`

```text
// Package ledgerverify ports verify_cycle from legacy/scripts/dispatch/
// evolve-loop-dispatch.sh:489-546. It walks the ledger.jsonl chain once
// per cycle and asserts the pipeline ran end-to-end: scout + builder +
// auditor each have at least one exit_code-zero entry, plus intent if the
// cycle was init'd with intent_required=true, plus memo if the cycle's
// audit verdict was PASS.
//
// Ledger vocabulary (cycle-137 fix): two writers record per-phase entries
// with DIFFERENT shapes, and the verifier must accept both or it
// false-negatives an entire class of cycles:
//
//   - bash subagent-run.sh   → kind="agent_subprocess", role=AGENT name
//     ("scout", "builder", "auditor")
//   - Go native orchestrator → kind="phase",            role=PHASE name
//     ("scout", "build", "audit")
//
// The Go `evolve loop` path emits ONLY kind="phase" entries (no
// agent_subprocess), so a verifier that matched the bash shape alone
// reported "missing [scout builder auditor]" for every native cycle —
// the cycle-137 incident. canonicalRole + the kind allow-list below
// fold both vocabularies onto the same {scout,builder,auditor,intent,
// memo} buckets so bash and Go cycles verify identically.
//
// Pure: the package depends only on core.Ledger + small data types.
// Callers (cmd_loop) are responsible for sourcing IntentRequired from
// cycle-state.json (priority) or state.json (fallback) and CycleVerdict
// from .cycle-verdict — see LoadVerifyContext.
```

### `go/internal/ledgerverify/verify.go:95` — above `CycleVerdict string`

```text
// CycleVerdict is the audit verdict for this cycle (PASS / FAIL /
// WARN / ""). When PASS, the memo agent_subprocess entry MUST exist
// (Layer P / Layer E3 contract from v8.58.0). Other verdicts skip
// the memo check.
```

### `go/internal/ledgerverify/verify_test.go:170` — above `entry(2, "scout", "agent_subprocess", 0),`

```text
// Wrong cycle — must not satisfy cycle 1's requirement.
```

### `go/internal/ledgerverify/verify_test.go:205` — above `func TestVerifyCycle_GoNativePhaseVocabulary(t *testing.T) {`

```text
// cycle-137 regression: the Go-native orchestrator records kind="phase"
// with PHASE-name roles (scout, build, audit) — never agent_subprocess.
// VerifyCycle must accept this vocabulary or it false-negatives every
// native cycle as "missing [scout builder auditor]".
```
