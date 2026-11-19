---
score_cap:
  - criterion: "EvidenceArchive local adapter returns an archived source by its source_sha256"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'Archive' ./internal/evidencex/ | grep -q '^ok'"
  - criterion: "EvidenceArchive refuses a write above its size cap with ErrArchiveTooLarge and leaves no file"
    max_if_missing: 7
    evidence: "cd go && grep -q 'ErrArchiveTooLarge' internal/evidencex/archive.go && go test -count=1 -run 'Archive' ./internal/evidencex/ | grep -q '^ok'"
---

# Eval: EvidenceArchive port and local adapter for the full evidence mode (I13)

> Pins the contract of decision D17 of the incident knowledge base plan: the `full` evidence mode writes each source through the `EvidenceArchive` port, keyed by `source_sha256`, and the local adapter refuses a write above its size cap. The source is the I13 inbox item of cycle 2537.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| round-trip | an archived source is found by its digest | 6/10 | `go test -run Archive ./internal/evidencex/` |
| cap-refusal | a write above the cap is refused, nothing is stored | 7/10 | `go test -run Archive ./internal/evidencex/` |
