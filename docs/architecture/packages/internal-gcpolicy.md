# internal/gcpolicy

## Purpose

`internal/gcpolicy` is the zero-dependency leaf that holds the `gc` block of `.evolve/policy.json`: the vocabulary of the retention engine's configuration, split from the engine (`internal/gc`).

## Design

- **Why a leaf.** `internal/policy`, the configuration SSOT, held `GC *gc.Policy`, so configuration depended on an engine. That edge closed an import cycle the moment `gc` needed the `phasecontract` registry (`policy -> gc -> phasecontract -> phasespec -> policy`). Configuration now points at a leaf that points at nothing. The engine re-exports these types as aliases, so every `gc.Policy` call site is unchanged. `cyclestate` and `shiperr` have the same shape.
- **Zero means defaults.** The zero `Policy` resolves through `WithDefaults`. `Mode` is `off`, `shadow` or `enforce`. An absent mode resolves to `shadow` (discover, plan and log a manifest, no mutation). `enforce` adds the apply step and is always an explicit operator decision.
- **Retention ladder.** `RunsPolicy.KeepFull` keeps the newest N run directories in full (by mtime, live or not; live runs are protected independently of the count). Beyond it, a dead run strictly older than `ArchiveAfterDays` moves under `<evolve>/archive/runs/`, and one strictly older than `DeleteAfterDays` is deleted. Delete wins over archive.
- **Worktree grace.** `WorktreesPolicy.KeepRecent` keeps the newest N fully eligible candidates (merged, clean, dead). `MinAgeMinutes` leaves a younger candidate alone, which covers the create-to-lease-write race.
- **Other TTLs.** `SalvageTTLDays` prunes `<evolve>/operator-salvage` (default 30), `LogsTTLDays` prunes `<evolve>/dispatch-logs/*.log` (default 30), `TrackerTTLDays` prunes the `.ephemeral` subtrees of kept runs (default 7).

## Invariants

- **No invented deletion horizon.** `WithDefaults` never defaults `ArchiveAfterDays` or `DeleteAfterDays`. Zero means never, and a retention engine must not invent a horizon the operator did not ask for. Pinned by `TestWithDefaults`.
- **Defaults never overwrite an operator value**, and `WithDefaults` is a value copy. Pinned by `TestWithDefaultsPreservesExplicitValues`.
- **The JSON tags are a wire contract.** Operators write them under `gc`, and `internal/policy` embeds the struct, so a tag change is a silent config break. Pinned by `TestPolicyJSONRoundTrip`.
