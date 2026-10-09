# internal/core/defectledger

> Unit 09 of the component breakdown. Extraction record, API and test plan: [09-defectledger.md](../decomposition/09-defectledger.md), unit 09 of [ADR-0103](../adr/0103-component-breakdown-program.md). The audit phase's EGPS gate calls it: [internal-phases-audit.md](internal-phases-audit.md). The continuation ledger contract as the auditor reads it: [continuation-defect-ledger.md](../continuation-defect-ledger.md). This page keeps the package-level detail those do not.

## Purpose

`internal/core/defectledger` is the audit phase's anti-laundering record. It owns the defect and prescription ledger (`defect-ledger.json`). It also owns the continuation disposition gate, which grades an inherited ledger against the claims in `defect-dispositions.json`.

The package has four jobs:

- EMIT the OPEN rows that a rejecting audit mints.
- ARM: decide if this cycle is a continuation, and which record says so.
- GRADE: run the disposition diff, and write the result back before the verdict.
- PREFLIGHT: report the artifact-level MISSING or INCOMPLETE finding.

The package also builds the prompt block that tells a continuation which ids it inherits. The schema and vocabulary are the one home that the audit, carryover and the adoption seeder use.

## Design

- **Explicit collaborators.** A `Ledger` takes the lane-identity reader and the citation resolver as required constructor arguments. The resolver is the audit package's four-rule policy, injected as a Strategy. A nil reader or resolver panics at first use. A silent nil disarms the gate, or grades FIXED without touching disk.
- **Signal Center through an accessor.** `WithSignals` takes a function that returns the Center. The function runs at every use, because a root can install its Center after the phase is built. A nil function, or one that returns nil, is the Null Object. Every path runs, and nothing is reported.
- **One producer of signals.** `emit` is the only producer. Each fault is an `audit.warning` under module `audit`, at the code's fixed severity. INFO marks the prompt degrade. WARN marks the twelve faults. Each signal carries the cycle, the phase and the exported method that owns the diagnostic.
- **The graded diagnostics are a wire.** The gate returns diagnostics that match, byte for byte, the wire that the dossier and the bookkeeping regrade read. Every message starts with `defect ledger: `. The leaf is the one author, and the host appends the messages unchanged.
- **Arm is separate from grade.** `arm` decides if the grade runs, and which record establishes a continuation. The records are the workspace manifest and the root-owned continuation registry. The registry is keyed by this lane's scope pin. Grade is the disposition diff against an established lineage.
- **Reconcile merges and never replaces.** Inherited rows are rebuilt from the ancestor on every pass. The workspace ledger merges in. Rows that Emit appended earlier in the same cycle survive.
- **Ids are content hashes.** An entry id is `d` plus the hex form of the first sixteen bytes of the SHA-256 of the defect text. A positional id binds to different text when a list is reordered.
- **Overflow is recorded.** Past `MaxEntries`, the ledger keeps one synthetic OPEN row and reports `AUDIT_LEDGER_OVERFLOW`. A cap that erased defects is the laundering primitive.
- **Dispositions tolerate shape, not claim.** A disposition's `evidence` is a string or an array of strings. An object, a number or a bool is rejected. A silent degrade to empty evidence is the gate's cheapest bypass.
- **The prompt is context, not enforcement.** `PromptBlock` degrades to empty, with one INFO, on a read fault. The gate blocks the same fault loudly at classify time.
- **The schema example is shared.** `DispositionsSchemaExample` matches, byte for byte, the example in the auditor persona and in the continuation ledger doc. The audit package's `defect_ledger_doc_example_test.go` keeps the three in sync.
- **Rune caps.** Ledger text is cut with the suffix `…[truncated]` and no leading space. This is the fourth rune-cap rule. It differs from the three rules in `carryover` on the same input.

## Invariants

- **Absent is clean. Unreadable is loud.** A missing ledger has nothing to reconcile. A present but unparseable ledger is a fault. Schema drift on the anti-laundering record is never silent.
- **A missing dispositions file is not a pass.** It yields an empty claim map and a warning. Each inherited OPEN entry then falls through to "unaccounted" and is named by id. An unreadable or unparseable file blocks at once.
- **The manifest decides if the gate runs.** One corrupt byte in the workspace manifest blocks the cycle. A missing manifest, while the registry binds this lane, is a blocking finding.
- **Arming never depends on one workspace file alone.** The registry also records the lineage. When the two records name different ancestors, the gate refuses to pick one.
- **An unreadable registry creates no lineage.** The registry fallback is a miss at that point, not a manufactured lineage. This is the known ceiling, recorded in the continuation ledger doc.
- **The registry lookup is scoped to this lane.** The lane's identity is its pinned scope. Other lanes' bindings in the same root registry must not block ordinary cycles.
- **Status comes from the claims only.** The graded agent can rewrite the workspace ledger. So a planted FIXED row must satisfy nothing. Inherited rows take their status from the ancestor and the claims.
- **FIXED needs a resolving cite. DEFERRED needs a reason.** A rejected FIXED row is written back with no evidence and no reason. An unverifiable FIXED row is the laundering.
- **A clean grade vouches only for a verified ancestor.** The vouched lineage is the immediate ancestor plus the origin cycle. The ancestor ledger must exist and have entries. The unblocked missing or empty branch verified nothing. A deleted ancestor ledger cannot demote the closure gate.
- **Vouching reuses the loaded ancestor.** The grade loads the ancestor once. The vouch reads the same document.
- **The merged ledger lands before the verdict.** The operator must read what this cycle disposed of, even on a run that blocks. The write-back comes first. The unaccounted check comes second.
- **Entries transition and are never deleted.** An unaccounted entry stays OPEN.
- **The first current row wins the index.** On a duplicated id, the first row is the index target. An inherited id whose text differs is shadowed. The ancestor's text is restored as OPEN.
- **The written-back origin is the ancestor's.** If the ancestor has no origin cycle, the current ledger's origin cycle is used.
- **Emit mints nothing for an empty rejection.** An empty ledger on every cycle makes every later cycle look like a continuation. That makes the reconcile gate vacuous.
- **Emit never blocks.** The ledger is a record at emit time, not a gate. A read or write fault is one warning carrying the graded wire. The verdict stands.
- **Prescriptions carry a prefix.** Rows minted from a WARN's prescriptions have the prescription prefix. An operator can then tell a named fix from a defect.
- **A signal carries at most 64 ids.** Sixty-four ids of 33 runes break the line cap, and the normalizer cuts silently. The full list stays in the diagnostic and in the written-back ledger.
- **The prompt keeps agent text on one capped line.** Text from a prior cycle's verdict cannot add a heading. That heading looks like mechanism-authored prompt structure.
- **MISSING and INCOMPLETE stay distinct.** MISSING means the artifact is absent. INCOMPLETE means it exists but is short. The operator action differs for each.
- **Collaborators are required.** A ledger with a nil reader or a nil resolver is refused at first use. It is not disarmed.

## Findings

- **cycle-1285 F2**: one byte of garbage in a 0644 workspace file retired every inherited defect. The manifest decided arming, and a corrupt manifest did not block. The manifest now blocks. The deletion half is the registry witness above.
- **cycle-1282 DEF-1**: the `current` workspace file is writable by the graded agent. A planted FIXED row satisfied the gate. Inherited status now comes only from the ancestor and the claims.
- **cycle-1282 DEF-3**: a 32-bit content hash can collide with a brute-forced preimage in a benign defect. Ids now use sixteen bytes, and the merge also checks text per id.
- **cycle-1282 DEF-6**: an agent-authored sentinel with thousands of defects, or a megabyte-long line, made the ledger quadratic on the audit hot path. The caps and the recorded overflow row answer it.
- **cycle-1342 F4**: the per-id switch blocked correctly. It never failed loudly by name on the artifact itself. The preflight now names MISSING or INCOMPLETE. It stays silent when no OPEN row exists, because a preflight that fires on every continuation proves nothing.
- **cycle-1399**: the auditor cited `["a.go:1", "b.go:2"]`. A string-typed field refused the whole document. The shape is now tolerated. The claim is still checked.
- **cycle-1403 Task 3**: the agent that rewrites the dispositions file does not read Go. So the schema example is shown inline on rejection.
- **cycle-1502**: a closure claim inside the vouched lineage is advisory. Vouching only trusts a verified ancestor, so a deleted ancestor ledger cannot demote the backstop.
- **F3, scout report Hypothesis 2**: prescriptions looked like defects in the ledger. They now carry a prefix.
- **G1 to G5 goldens, captured on 8e8f080f**: the ledger bytes, the diagnostic scenarios, the write-back bytes and the prompt block are replayed byte for byte. A change to any of them changes the wire that the dossier reads.
- **F6, follow-up**: the resolver splits on a bare `;`, which is looser than the join. A hand-written `a.go;b.go` still splits. The two tokens are not tied in code. The joined bytes are pinned by a test. Making the resolver its own leaf ties them.
