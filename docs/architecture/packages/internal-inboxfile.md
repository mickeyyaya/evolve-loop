# internal/inboxfile

## Purpose

`internal/inboxfile` admits a new item to the inbox. `Publish` validates the item, takes the inbox lock, checks the id, and links the file. `Mint` enables kernel minters to file items safely: a repeated id is an idempotent skip.

## Design

- **Leaf package.** `lifecycle` imports the ledger, the ledger imports `core`, and `core` imports `faillearn`. Therefore `faillearn` cannot import `lifecycle`. The filing rules live in this leaf package. It imports `inboxbatch` and `verifylock` only.
- **Rules.** `CheckFieldRules` enforces required fields: `id`, `title`, `kind`, `summary`, and `fix`. It requires a kebab-case id, a weight in (0, 1], and a non-empty `acceptance` list. It also applies the loader length bounds.
- **Lock.** `Publish` holds `.bump.lock` in the inbox directory (`LockName`) from the scan to the link step. It waits at most `LockWait`. Two concurrent writers of one id cannot both pass the scan.
- **Identity.** `FiledIDs` scans every `*.json` file under the inbox directory. A repeated id returns `ErrAlreadyFiled`. `Mint` turns that error into `Result{Skipped: true}`.
- **Minters.** `faillearn.WriteArtifacts`, `ciwatch.fileEscalation`, and `ciwatch.FileBaseRedItems` call `Mint`. Their degraded paths remain unchanged. The floor preserves `retrospective-unqueued.md`. `ciwatch` returns an error with the `ciwatch:` prefix after it writes the verdict.

## Invariants

- A minted item passes `lifecycle.File` on a fresh inbox. Pinned by `TestC2035_004_EveryMintedItemPassesTheFilingWritersRules`.
- Concurrent mints of one id file one item. Pinned by `TestC2035_010_ConcurrentCIRedMintsFileExactlyOneItem` and `TestC2035_012_ConcurrentFloorMintsFileEachItemExactlyOnce`.
- A held `.bump.lock` blocks a mint. Pinned by `TestC2035_013_CIRedMintWaitsForTheInboxBumpLock` and `TestC2035_015_FloorMintWaitsForTheInboxBumpLock`.
