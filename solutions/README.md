# solutions/ — document deliverables

This is where a **document cycle** (ADR-0099: a cycle whose `deliverable_kind` is `document`)
lands its work: one directory per inbox task id, `solutions/<slug>/`, holding candidate strategy
options, a recommendation, and the assumptions-and-evidence file every number cites.

The exact shape is **config**, declared once in `docs/architecture/phase-registry.json` under
`config.deliverable_kinds.document`, and judged by **one** deterministic engine
(`go/internal/solutioncheck`) that the build handoff floor, the audit gate, the Task Contract
the builder is handed, and this self-check all run:

```bash
evolve solution check <slug>      # or: evolve solution check solutions/<slug>
```

Exit 0 means the contract is satisfied; violations print one per line (exit 1). Quality —
whether the options are genuinely distinct, the numbers hold, the recommendation follows from
the evidence — is the audit phase's judgment, not this check's. See `docs/domain-adapters.md`
and `docs/architecture/adr/0099-deliverable-kinds.md`.
