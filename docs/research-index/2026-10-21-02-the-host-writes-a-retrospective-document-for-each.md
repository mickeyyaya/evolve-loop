## 2026-10-21 — the host writes a retrospective document for each recovery and each judgment lesson (ADR-0151)

| Document | What it records |
|---|---|
| [research/recovery-retrospective-2026-10.md](research/recovery-retrospective-2026-10.md) | Research: the detector rule finds 12 recoveries in the dossiers, and no record of a recovery exists. The dossier, not the ledger, is the exact record of a try, and a ship is a PASS with a commit. It also surveys Safety-II, blameless postmortems, flaky-test recovery and agent memory from success and failure pairs. Findings F2.1 to F2.13, P3.1 to P3.7. |
| [plans/recovery-retrospective-2026-10.md](plans/recovery-retrospective-2026-10.md) | Plan: four designs with weighted scores. The host writes the documents from the committed dossiers, at stage `shadow` first. Decisions D1 to D15, components C1 to C6. |
| [architecture/adr/0151-the-host-writes-a-retrospective-document-for-each-recovery-and-each-judgment-lesson.md](architecture/adr/0151-the-host-writes-a-retrospective-document-for-each-recovery-and-each-judgment-lesson.md) | Decision (Proposed): a recovery and a judgment lesson each get a document and a lesson YAML. A judgment todo leaves the carryover log only when its document is on `main`. |
| [architecture/recovery-retrospective.md](architecture/recovery-retrospective.md) | Spec of record: the recovery document, the judgment-lesson document, the lesson YAML, the closeout write and the `enforce` stage. |
| [architecture/packages/internal-recoveryretro.md](architecture/packages/internal-recoveryretro.md) | Package note: the renderers and the one writer, `WriteShadow`. |
