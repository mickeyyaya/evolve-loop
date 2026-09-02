// Package dashboard is the read-only observability surface behind
// `evolve dashboard` (ADR-0095): a stdlib-only local web UI that renders the
// loop's on-disk state — `.evolve/cycle-state.json`, the per-cycle run
// workspaces under `.evolve/runs/`, the committed dossiers under
// `knowledge-base/cycles/`, and the inbox — as a live page a human can read
// to answer: is the loop alive, what is queued, what did each cycle do, what
// went wrong, and how is the ship rate trending.
//
// Design rules the package holds itself to (see the design spec in
// docs/superpowers/specs/2026-09-02-ship-rate-harness-and-pipeline-dashboard-design.md):
//
//   - Every read is best-effort. An absent or half-written artifact zero-values
//     its field and lands in Snapshot.Warnings; it never fails the page.
//   - The package never takes the loop's flock sidecars and never writes. It
//     relies on the writers' atomic-rename discipline: a reader sees the old
//     file or the new one, never a torn one.
//   - No fsnotify (the module is vendored stdlib-only): change detection is a
//     mtime fingerprint poll that feeds one Server-Sent-Events stream.
//   - Everything rendered is LLM-authored text. The server ships a static
//     shell and JSON; the client places content into the DOM with textContent
//     only. Artifact reads are allowlisted by name, symlink-safe
//     (reportdoc.OpenRegularNoFollow) and size-capped.
//   - Beliefs owned elsewhere are imported, not re-declared: cycle workspace
//     paths (core.RunWorkspacePath), cycle-state path
//     (core.ResolveCycleStatePath), liveness (runlease.Fresh), inbox items
//     (inboxbatch.LoadDir), dossiers (dossier.ParseJSON), phase timing
//     (phasetiming.Read). Phase ORDER is taken from the observed timing log,
//     not from a fourth copy of the canonical list.
package dashboard
