# internal/auditledger

## Purpose

`internal/auditledger` owns the schema of an auditor row, the row identity (`kind=agent_subprocess`, `role=auditor`) and the run-scope rule. Each consumer maps the errors onto its own vocabulary. It is a leaf: it imports no consumer and not `internal/core`.

## Design

- `AuditorRows` returns every auditor row, newest first. A read failure wraps the OS error. `BindRun` returns the first row stamped with the run id, or the first row when the id is empty. `LatestAuditorEntry` combines both.
- `IsAuditorRow` accepts the audit agent's own subprocess row, never a phase marker.

## Invariants

- **An alien line is skipped.** An unparseable line is not an auditor row, so foreign lines in a shared ledger cannot break a reader.
- **An unstamped row never binds a run.** The ledger is shared by every fleet lane, so a row without a run id can belong to another lane's audit.
- **A miss wraps `ErrNoAuditorForRun`** and names the newest refused row, so the caller can say why.
