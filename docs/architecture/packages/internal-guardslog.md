# internal/guardslog

## Purpose

`internal/guardslog` is the best-effort guard-log append shared by the commit-prefix and post-edit validation guards. Each line is `[timestamp] [label] message`.

## Design

- **One home for open, write and close.** The guards used to open, write and close inline and discard the close error. Centralizing the three steps makes the close error a returned, testable value.
- **First error wins.** `Append` returns the write error when there is one, else the close error. A buffered or networked sink can surface its failure only at close.
- **Parent directories are created.** A missing parent is made with `MkdirAll` before the file is opened.
- **Empty path is a no-op.** An empty path returns nil without touching the disk.
- **Best-effort callers discard the result explicitly.** The guards decide what a failed log line means. This package only reports it.
- **Seam for tests.** `openAppend` is a package variable. Tests replace it with a `WriteCloser` whose close or write fails, because those paths are hard to trigger on a real file.

## Invariants

- **The close error is surfaced.** `TestAppend_SurfacesCloseError` pins this.
- **A write error takes precedence over a close error.** `TestAppend_WriteErrorTakesPrecedence` pins this.
- **Mkdir and open failures are surfaced.** `TestAppend_MkdirErrorSurfaced` and `TestAppend_OpenErrorSurfaced` pin this.
- **The line format is fixed.** `TestAppend_WritesLine` pins the timestamp in UTC, RFC 3339.

## Findings

- None.
