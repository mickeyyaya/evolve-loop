# internal/inputindex

> Decision: [ADR-0154](../adr/0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md) I-4 and I-5. Spec of record: [llm-isolation.md](../llm-isolation.md) §8 and §9, and [cell-context-index.md](../cell-context-index.md) for the layers, the budgets and the access log. Plans: [llm-isolation-2026-10](../../plans/llm-isolation-2026-10.md) component X5, and [cell-context-index-2026-10](../../plans/cell-context-index-2026-10.md) components CX1 and CX3. Callers: `go/internal/core/input_index.go` writes the index, and `go/internal/bridge/read_isolation.go` grants its paths.

## Purpose

`internal/inputindex` builds the artifact index `<ws>/inputs/index.json` of one phase cell. The index lists the files that the cell can open on demand. The sandbox grants reads of exactly these paths and the own workspace. Nothing from the index goes into the prompt.

The operator set the rule on 2026-10-25: earlier deliverables and the retry and recovery lessons must be "accessible when needed (not necessary to be loaded at the beginning)".

## Design

- **`FileEntry(root, path, class, kind, why)`** makes one entry from one pipeline file, with its digest and summary, by the rules of `Build`. A missing file or a symlink is no entry. The evidence index of the self-healing loop (`selfheal.WriteEvidence`, [llm-isolation.md](../llm-isolation.md) §18.6) uses it for its own classes.
- **`Build(Request)`** reads only pipeline records. It returns an `Index` with the schema `evolve.inputs.index/2` (`Schema`). The index has the cycle, the phase, the item ids, the `attempt` number, the lineage, the entries and the notes. The root and the workspace must be absolute.
- **Deliverables.** For each phase contract of `phasecontract.Contracts` that writes to the workspace, the artifact of each other phase is an entry when the file exists. `triage-decision.json`, `acs-verdict.json` and each `handoff-*.json` are entries too. A `handoff-*.json` has the kind `handoff-asks`.
- **Records.** `WriteRecord(root, Record)` writes `.evolve/lineage/cycle-N.json` with the cycle, the run id and the item ids. `ReadRecord` reads it back. The caller gives the ids from the fleet scope of the orchestrator (`Request.ItemIDs`). No cell can write the directory.
- **Lineage.** `Lineage(root, cycle, itemIDs)` gives each cycle whose record holds an item id of this cycle. A malformed record adds a note. It adds the cycle of the continuation registry entry of each item id. `Build` keeps the last `Request.LineageMax` cycles (default `DefaultLineageMax`, 5).
- **Lineage entries.** For each lineage cycle the entries are `retrospective-report.md`, `failure-digest.json`, `carryover-todos.json`, `audit-report.md`, `retrospectives/*.md` (the ADR-0151 documents) and `failure-lesson*.yaml`. A lesson under `.evolve/instincts/lessons/` is an entry when its text names an item id or `cycle-N` of a lineage cycle as a whole token.
- **The `attempt` field.** The `attempt` of the cycle is the number of lineage cycles before the cap, plus 1. Each lineage entry carries the `attempt` of its cycle.
- **Entries.** An entry has `id`, `class`, `kind`, `path` (relative to the project root), `cycle`, `phase`, `attempt`, `summary`, `bytes` and `sha256`. The summary is one line of at most 160 characters. It comes from the first line of a Markdown file that is not a comment, or the `summary` key of a JSON object. For a YAML file it comes from the `summary:` or `title:` line. No LLM writes it.
- **Request entries.** `RequestsDir(root, cycle)` is `.evolve/lineage/requests/cycle-<N>/`, where the request channel writes one record for each request of a cell ([llm-isolation.md](../llm-isolation.md) §7.2.1). `Build` lists each record with the kind `request` (`KindRequest`). A record of the cycle is a `deliverable`, and a record of a lineage cycle is a `lineage` entry.
- **Raw output.** `IsRawOutput` refuses each path under a `.bridge/` directory and each name that ends in one of `RawOutputSuffixes`: `-stdout.log`, `-stderr.log`, `-stdout.clean.txt`, `-final-pane.txt`, `-final-scrollback.txt`, `-pane-watch.json`. No entry is raw output, because no source names it.
- **Layers (CX1).** `annotate` gives each entry its `layer` from its class: `deliverable` is `L1` (`LayerUpstream`), and `lineage` is `L2` (`LayerRecovery`). It sets `tokens` to `ceil(bytes/3)` and `why` from a fixed template of the source, for example `upstream deliverable of scout`.
- **Required (CX1).** `Request.Context` is a `ContextPolicy`. The first source that has a list gives the required kinds:
  1. `ContextPolicy.Required`, the policy list of the phase;
  2. `ContextPolicy.Inputs`, the `inputs.files` of the phase specification (the base name without the extension);
  3. the upstream edges of `phaseio.EdgeUpstreams`.
- **Required asks and lineage (CX1).** Each `handoff-asks` entry whose name ends in `-<phase>.json` is required. On a retry, the `retrospective` and the `failure-digest` of the newest lineage cycle are required.
- **Budgets (CX1).** `Budget.L1Tokens` and `Budget.L2Tokens` (defaults 8000 and 6000) bound the sum of `tokens` of the required entries of a layer, in entry order. An entry past the budget keeps its row and loses the flag. Its `why` and a note say so, and `LayerBudget.Over` counts it. The header has `stage` (default `shadow`), `estimate`, `required` (the ids) and `budgets`.
- **Read (CX1).** `Read` accepts the schema `/2` and, for one release, `evolve.inputs.index/1`.
- **Open (CX2).** `Index.Open(root, id)` returns the entry and its body. It refuses an unknown id (`ErrUnknownEntry`), a path outside the root (`ErrOutsideRoot`) and a body whose `sha256` differs from the index (`ErrChangedBody`).
- **Access log (CX3).** `RecordAccess(root, idx, dispatch, source, opened)` appends one `AccessRecord` of type `open` for each opened entry and one of type `cell` to `cycle-<N>.jsonl` in `AccessDir(root)` (`.evolve/context-access/`). The cell record lists each row (`AccessRow`), the opened ids and the unopened required ids. `source` is `sandbox` (`AccessSourceSandbox`) or `none` (`AccessSourceNone`). `ReadAccess` reads each log and skips a line that does not decode.
- **Stats (CX3).** `Stats(records, phase)` gives a `KindStat` for each phase and kind: the cells that listed it, the cells that opened it, the required count and the rate. It counts only cell records with `source: sandbox`, and it leaves out a scan: a cell with more than one row that opened each row.
- **Grants.** `ReadablePaths(root, ws)` reads the index and returns the index path and the absolute path of each entry. It drops an absolute path, an empty path and a path that leaves the root.

## Invariants

- Lineage comes only from the records and the continuation registry. A `lane-scope.json` in a workspace and the text of an artifact never add a cycle (`TestLineageComesOnlyFromPipelineRecords`).
- No item id gives no lineage, and a note in `notes` says so. An unreadable continuation registry adds a note and no cycle.
- A source that is a symlink, that is under a symlinked directory, or that has a second hard link is not an entry. So a cell of an earlier phase cannot plant a link that puts the text of another file into a summary.
- A deliverable that exists but cannot be read fails `Build`. The caller warns and the cell gets no index.
- A budget overflow never removes a row, so it never removes a grant (`TestBudget_AnOverflowClearsRequiredAndKeepsTheRow`).
- `Open` reads only a listed path inside the root, and only the body that the index hashed (`TestOpen_ReturnsTheBodyOfAListedEntryAndRefusesEachOtherRead`).

## Findings

- Real records: the run directory of cycle 2028 holds `audit-stdout.log`, `audit-stdout.clean.txt`, `audit-stderr.log` and three `-final-pane.txt` captures beside the reports (listing of `.evolve/runs/cycle-2028/`, 2026-10-26). The raw suffixes come from that listing.
