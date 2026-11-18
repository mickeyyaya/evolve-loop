# internal/incidentbundle

> Plan: [incident knowledge base](../../plans/incident-knowledge-base-2026-11.md), component I3 and decisions D3, D13 and D16.

## Purpose

`internal/incidentbundle` builds the evidence bundle at `knowledge-base/incidents/<id>/` and checks it. The bundle stays in git, so an incident record traces to its evidence after the run directories are deleted.

## Design

- **Add.** `Add` redacts a source with `redact.Secrets`, writes the copy and appends one manifest entry. The entry embeds `inputindex.Entry` and adds `captured_at`, `source_address`, `source_sha256`, `redactions`, `state` and `templates`. `source_sha256` is the digest of the raw source, and `sha256` is the digest of the copy. `Add` skips a source with a known `source_sha256`.
- **Budget.** The limit is the policy key `incident.evidence.budget_bytes`, first value 256 KB. When the copies exceed it, the oldest repeat source becomes a `pointer` entry. A repeat source is a source whose every error template another full entry also holds. The copy is deleted, and the digest, the address and the size stay. When no source repeats, `Add` keeps all the copies.
- **Error template.** A line with `ERROR`, `FATAL` or `panic:` loses its leading timestamp, and its digit runs become `#`.
- **Check.** `Check` fails in four cases:
  - A copy differs from its manifest digest.
  - A file is in the bundle and not in the manifest.
  - A copy still holds a secret that `redact.Secrets` replaces.
  - `redact.EntropyRuns` finds a run in a copy.
- **CheckAll.** It checks each bundle of the root.
- **Id.** The id is `yyyy-mm-dd-slug`, so it cannot name a path outside the bundle root.

## Limits

- The size ratchet on the whole `knowledge-base/incidents/` tree and the `extract` pipeline of D16 are later components (I12).
- The CLI is `evolve issue evidence add` and `evolve issue check` in `cmd/evolve/cmd_issue.go`.
