# Eval: checkpoint-clobber-fix

## Objective

`FilesystemStorage.WriteCycleState` must preserve an existing `"checkpoint"` block in
`cycle-state.json` rather than silently erasing it during the whole-struct atomic rewrite.

---

## Criterion 1 — WriteCycleState preserves an existing checkpoint block [code]

```bash
cd "$(git rev-parse --show-toplevel)/go"
go test ./internal/adapters/storage/... -run TestWriteCycleState_PreservesCheckpoint -v -count=1
```

**Expected:** `--- PASS: TestWriteCycleState_PreservesCheckpoint`

---

## Criterion 2 — WriteCycleState with no prior checkpoint leaves no checkpoint [code]

```bash
cd "$(git rev-parse --show-toplevel)/go"
go test ./internal/adapters/storage/... -run TestWriteCycleState_NoCheckpointWhenNonePrior -v -count=1
```

**Expected:** `--- PASS: TestWriteCycleState_NoCheckpointWhenNonePrior`

---

## Criterion 3 — Full storage test suite stays green [code]

```bash
cd "$(git rev-parse --show-toplevel)/go"
go test ./internal/adapters/storage/... -count=1 -timeout=30s
```

**Expected:** `ok  github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage`

---

## Negative case — checkpoint block is NOT duplicated when WriteCycleState called twice [code]

```bash
cd "$(git rev-parse --show-toplevel)/go"
go test ./internal/adapters/storage/... -run TestWriteCycleState_CheckpointNotDuplicated -v -count=1
```

**Expected:** `--- PASS: TestWriteCycleState_CheckpointNotDuplicated`
