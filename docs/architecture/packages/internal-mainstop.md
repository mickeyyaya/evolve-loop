# internal/mainstop

## Purpose

`internal/mainstop` holds the record of a red `main` and the transitions of its stop. The design is in [gate-contract-g4-base-red-attribution.md](../gate-contract-g4-base-red-attribution.md), sections 6.2 and 7.4 to 7.6.

## Design

- `Stop` is a value. Each transition returns a new `Stop` and never changes the slices of its source.
- `Holds` is true when the stop has a key and no release time. A zero `Stop` does not hold.
- `Opened` starts a stop from the `MainRed` keys and the `Main` commit of a verdict. On a stop that holds, it merges the keys and moves `MainSHA`.
- `Opened` never moves `OpenedAt` of a stop that holds, so a refresh cannot reset the bound. It keeps `FixItem`, and it keeps `MainSHA` when the verdict has no `Main`.
- `WithParked` keeps one row for each cycle, in cycle order. A second park of a cycle replaces its row.
- `Admit` gives `AdmitNoStop` when the stop does not hold. It gives `RefuseRebase` when the proof base is not the `main` tip, or when the tip is empty.
- `Admit` gives `AdmitFix` when the proof has a green stop key and no lane-only red. Each other proof gives `RefuseNotAFix`.
- `Released` drops the green keys. With no key left, it sets `ReleasedAt` and `ReleasedBy`. It does not change a stop that does not hold.
- `Overdue` is true when a stop that holds is older than a positive bound. `Escalate` records only the first escalation time.
- `Cause` names the keys, `MainSHA`, `OpenedAt` and the parked cycles. `ResumeCondition` gives the condition that ends the stop.
- `ParkedCycles` gives the parked cycle numbers in order, for a signal field. It is empty when no cycle is parked.
- `FixItemID` is `fix-main-red-` and the first 8 characters of the `main` commit.

## The record

- `Path` is `main-red-stop.json` in the `.evolve` directory.
- `Read` gives no stop and no error for a missing record. A record that does not parse is an error.
- `Read` refuses a record whose `schema` is not `Schema`. `Update` sets `Schema` on each stop that it writes.
- Each key in the record has the fields `package` and `test`.
- `Update` takes a lock on the file `main-red-stop.json.lock`, reads the record, applies the function and writes the result.
- The lock is on a second file, because the write replaces the record through a rename.
- `Update` writes through `atomicwrite`. An error of the function or of the write leaves the record as it was, and `Update` returns the stop on disk.

## Invariants

- The package imports `baseattr`, `adapters/flock`, `atomicwrite` and the standard library. It imports no pipeline package.
- Ship is the one caller of `Admit` and `Released` (`phases/ship/main_red_stop.go`). `core` is the one caller of `Opened` and `WithParked`.
- The package emits no signal and reads no policy. The caller owns the stage, the bound and each signal.
- The path is on the protected surface as a gate guarantee. A lane cannot edit it.
- The record is an `authority` input in [gate-input-inventory.md](../gate-input-inventory.md). No profile grants it.
