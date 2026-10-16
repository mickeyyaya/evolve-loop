# internal/contextfillcorrelate

## Purpose

`internal/contextfillcorrelate` answers one question: do cycles that ran hot fail more often? It joins the per-phase context-window fill ratio of a cycle with the final verdict in the cycle dossier. The ratio comes from `contextfill` and is stored on `phasetiming.Entry.ContextFillRatio`. The derive and store steps (parts 1 and 2 of the `context-fill-telemetry-and-cap` inbox item) existed first. Nothing asked the question that the number exists to answer.

## Design

- **A leaf package.** It imports the standard library, `internal/contextfill` (the one definition of the hot boundary) and `internal/phasetiming` (the one reader of the timing log). It does not derive the ratio again.
- **`Correlate` is the pure join.** It puts each cycle in a bucket by its peak per-phase fill ratio. It counts the verdicts of each bucket.
- **`Load` reads the real corpus.** It reads each `knowledge-base/cycles/cycle-*.json` dossier for `final_verdict`. It joins by cycle id to `.evolve/runs/cycle-<n>/phase-timing.json`.
- **Band edges.** The last edge is `contextfill.HotThreshold`. A second copy of 0.85 here can drift. `Bucket.Min` is inclusive and `Bucket.Max` is exclusive. The top bucket is open-ended.
- **Ratios above 1.0 stay.** `contextfill.FillRatio` reports overruns on purpose. A ratio at or above the last edge lands in the top band.
- **The open-ended `Max` is `+Inf`, written as JSON `null`.** JSON cannot hold infinity. `bucketWire` carries `Max` as a pointer. `UnmarshalJSON` decodes `null` back to `+Inf`. A `Report` then round-trips through the `--json` projection of the CLI. `TestAPICoverNamedExports` pins this.
- **`FailRate` of an empty bucket is a finite 0**, not a 0/0 NaN.
- **`readDossier` reads each file once.** An earlier pair of accessors decoded every dossier twice. That was about 1450 whole-file reads over a corpus of 726 dossiers, for two fields of one struct.

## Invariants

- **Absent evidence is never data.** A cycle with no usable fill ratio, or with no final verdict, lands in `Report.NoData`. It never goes into the lowest bucket as a made-up 0.0. That value creates the correlation that the package measures.
- **An empty `Fills` means unknown, not "ran empty".** `Fills` holds only the ratios present in the timing log. The field is `omitempty`, so absent and 0.0 look the same on disk. A peak that is not positive counts as absent. `TestCorrelateNeverFabricatesAZeroRatio` pins this.
- **Conservation.** `CyclesJoined + len(NoData)` equals the number of dossiers read. Every dossier on disk yields one `CycleFill`.
- **A bad input degrades to no data.** An unreadable dossier or a missing timing log gives an empty row. `Correlate` reports that row as no data. Load never drops a cycle. `TestLoadAccountsForACorruptDossier` pins this.
- **The verdict set is closed.** `recognisedVerdicts` is checked before a row counts as joined. An unknown verdict can come from schema drift, a typo or a future value such as `BLOCKED`. If it raised `Bucket.Cycles`, it raised the denominator of `FailRate` and never the numerator. That dilutes the correlation.
- **Unknown verdicts go to `NoData`.** An empty verdict is the most common unknown one. The tally switch is therefore exhaustive by construction. WARN has its own arm and does not move the FAIL rate. `TestCorrelateExcludesUnrecognisedVerdicts` and `TestCorrelateTalliesEveryRecognisedVerdict` pin this.
- **The file name is the cycle identity.** The glob makes the file name unique on disk. The `cycle` field in the body is free text that no writer validates. Trust in the body once let two dossiers claim one id. One cycle was then tallied twice, while the row count still balanced.
- **The body id is a fallback only.** It applies when the file name does not parse, and only for an id that no other dossier holds. A corrupt dossier then still has its slot (`cycleFromPath`). `TestLoadRejectsADuplicateBodyCycleID` and `TestLoadFallsBackToBodyIDOnlyForAnUnclaimedID` pin this.
- **An empty corpus is an error.** A root with no dossiers fails loudly. It must not read as a zero correlation. `TestLoadRequiresACorpus` pins this.
