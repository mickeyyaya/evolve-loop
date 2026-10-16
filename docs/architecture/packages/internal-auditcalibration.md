# internal/auditcalibration

## Purpose

`internal/auditcalibration.Generate` reads the dossiers directory and the runs directory and returns a deterministic Markdown calibration report. The sample is the set of canonical `cycle-N` run directories.

## Design

- The report is deterministic so that two runs on one corpus give the same bytes.

## Invariants

- Only canonical `cycle-N` directories enter the sample.
