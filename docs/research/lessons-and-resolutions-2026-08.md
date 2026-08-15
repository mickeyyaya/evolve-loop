# Evolve-Loop: Lessons & Resolutions — the August 2026 Hardening & Alignment Campaign

> **Landing path:** `docs/research/lessons-and-resolutions-2026-08.md`
> (same home as the canonical July lessons and retrospectives).
> **Scope:** cycles ~1221–1469, releases v22.10.0 → v22.17.0, August 2026.
> **Audience:** developers and agents working on the pipeline's security boundary and parsing semantics.

---

## 1. Executive Summary

August 2026 focused on deliverable alignment, robust parser hardening, and eliminating subtle race conditions/state inconsistencies across concurrent lane executions. A major theme of the campaign has been resolving *grammar alignment* errors, where safety guards and extractors interpret attacker-controlled bytes via conflicting models.

---

## 2. Hardening Defect Families & Lessons

### 2.1 Grammar Discrepancies & Ambiguity Guard Evasion (Cycle 1427 / 1432)

*   **Symptom:** The deliverable-salvage ambiguity guard was bypassed by nested/array decoys, allowing incorrect or conflicting verdicts to launder through the gate as approvals.
*   **Root Cause:** The safety guard (`candidateCount`) and the extractor it authorized parsed the same attacker-controlled bytes using two different grammars:
    1.  A brace balancer counted top-level JSON objects.
    2.  A flat regular expression selected the first inner object matching the target signature.
    Thus, a count of exactly one top-level object authorized a selection that the top-level count never saw.
*   **Wrong Turns:** Relying on separate stateful scans (string-aware and brace-only) to detect duplicates. Unpaired quotes or unmatched braces in prose could desynchronize either reading, reducing the count and silencing the ambiguity guard.
*   **Resolution:** Taking the max of three readings, including a stateless `verdictKeyCount` that counts `"verdict":` keys directly to prevent scan desynchronization.
*   **Lesson:** **(1) ONE PRIMITIVE, NOT TWO AGREEING ONES.** A safety guard and the authorized consumer must parse input using the same primitive parser rather than relying on two separate grammars that supposedly agree.

### 2.2 Stale Continuation Bindings & Absorbing FAILs (Cycle 1412 / 1418)

*   **Symptom:** Declining a stale workspace snapshot without a manifest caused the defect-ledger gate to automatically fail every future lane execution on that scope (immortal registry bindings).
*   **Root Cause:** The continuation registry retained declining bindings indefinitely. Concurrent lanes rebinding concurrently created race windows.
*   **Resolution:** Implemented `DeleteRegistryEntryIfCycle` using single-lock file synchronization and released declined registry bindings upon adoption decline.

### 2.3 Persona-Strip Truncation / Lobotomy (Cycles 1390–1429)

*   **Symptom:** Audit prompt execution failed on missing/truncated verdict rules and output-path contracts.
*   **Root Cause:** Relocation of reference markers within prompt templates caused `CompactPrompts` to aggressively strip constitutional guidelines and mandatory contracts.
*   **Resolution:** Moved reference index markers strictly to EOF and added a fleet-wide keep-guard test suite to prevent truncation regression.
