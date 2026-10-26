# internal/inputindex

> Decision: [ADR-0154](../adr/0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md) I-4 and I-5. Spec of record: [llm-isolation.md](../llm-isolation.md) §8 and §9. Plan: [llm-isolation-2026-10](../../plans/llm-isolation-2026-10.md) component X5. Callers: `go/internal/core/input_index.go` writes the index, and `go/internal/bridge/read_isolation.go` grants its paths.

## Purpose

`internal/inputindex` builds the artifact index `<ws>/inputs/index.json` of one phase cell. The index lists the files that the cell can open on demand. The sandbox grants reads of exactly these paths and the own workspace. Nothing from the index goes into the prompt.

The operator set the rule on 2026-10-25: earlier deliverables and the retry and recovery lessons must be "accessible when needed (not necessary to be loaded at the beginning)".

## Design

- **`Build(Request)`** reads only pipeline records. It returns an `Index` with the schema `evolve.inputs.index/1`, the cycle, the phase, the item ids, the `attempt` number, the lineage, the entries and the notes. The root and the workspace must be absolute.
- **Deliverables.** For each phase contract of `phasecontract.Contracts` that writes to the workspace, the artifact of each other phase is an entry when the file exists. `triage-decision.json`, `acs-verdict.json` and each `handoff-*.json` are entries too. A `handoff-*.json` has the kind `handoff-asks`.
- **Records.** `WriteRecord(root, Record)` writes `.evolve/lineage/cycle-N.json` with the cycle, the run id and the item ids. `ReadRecord` reads it back. The caller gives the ids from the fleet scope of the orchestrator (`Request.ItemIDs`). No cell can write the directory.
- **Lineage.** `Lineage(root, cycle, itemIDs)` gives each cycle whose record holds an item id of this cycle. A malformed record adds a note. It adds the cycle of the continuation registry entry of each item id. `Build` keeps the last `Request.LineageMax` cycles (default `DefaultLineageMax`, 5).
- **Lineage entries.** For each lineage cycle the entries are `retrospective-report.md`, `failure-digest.json`, `carryover-todos.json`, `audit-report.md`, `retrospectives/*.md` (the ADR-0151 documents) and `failure-lesson*.yaml`. A lesson under `.evolve/instincts/lessons/` is an entry when its text names an item id or `cycle-N` of a lineage cycle as a whole token.
- **The `attempt` field.** The `attempt` of the cycle is the number of lineage cycles before the cap, plus 1. Each lineage entry carries the `attempt` of its cycle.
- **Entries.** An entry has `id`, `class`, `kind`, `path` (relative to the project root), `cycle`, `phase`, `attempt`, `summary`, `bytes` and `sha256`. The summary is one line of at most 160 characters. It comes from the first line of a Markdown file that is not a comment, or the `summary` key of a JSON object. For a YAML file it comes from the `summary:` or `title:` line. No LLM writes it.
- **Raw output.** `IsRawOutput` refuses each path under a `.bridge/` directory and each name that ends in one of `RawOutputSuffixes`: `-stdout.log`, `-stderr.log`, `-stdout.clean.txt`, `-final-pane.txt`, `-final-scrollback.txt`, `-pane-watch.json`. No entry is raw output, because no source names it.
- **Grants.** `ReadablePaths(root, ws)` reads the index and returns the index path and the absolute path of each entry. It drops an absolute path, an empty path and a path that leaves the root.

## Invariants

- Lineage comes only from the records and the continuation registry. A `lane-scope.json` in a workspace and the text of an artifact never add a cycle (`TestLineageComesOnlyFromPipelineRecords`).
- No item id gives no lineage, and a note in `notes` says so. An unreadable continuation registry adds a note and no cycle.
- A source that is a symlink, that is under a symlinked directory, or that has a second hard link is not an entry. So a cell of an earlier phase cannot plant a link that puts the text of another file into a summary.
- A deliverable that exists but cannot be read fails `Build`. The caller warns and the cell gets no index.

## Findings

- Real records: the run directory of cycle 2028 holds `audit-stdout.log`, `audit-stdout.clean.txt`, `audit-stderr.log` and three `-final-pane.txt` captures beside the reports (listing of `.evolve/runs/cycle-2028/`, 2026-10-26). The raw suffixes come from that listing.
