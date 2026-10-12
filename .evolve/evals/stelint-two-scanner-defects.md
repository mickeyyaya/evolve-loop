---
score_cap:
  - criterion: "steDisplay keeps a root file named with two leading dots relative and a file outside the root absolute"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestSteDisplay_RootFilesNamedWithTwoLeadingDotsStayRelative$' ./cmd/evolve"
  - criterion: "evolve docs ste-lint prints ..notes.md relative to the project root"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestDocsSteLint_ADotDotPrefixedRootFileShowsItsRelativePath$' ./cmd/evolve"
  - criterion: "formatVerb treats indexed width and precision verbs as one code token"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestCheckGo_IndexedWidthAndPrecisionVerbsAreCodeTokens$' ./internal/stelint"
---

# Eval: stelint two scanner defects

> Pins two defects found in cycle 1860. `steDisplay` showed an absolute path for a root file such as `..notes.md`, and `formatVerb` rejected `%[3]*.[2]*[1]f`, so the verb letter was read as a prose word.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| dotdot-name | root file `..notes.md` stays relative | 6/10 | `go test -run TestSteDisplay_RootFilesNamedWithTwoLeadingDotsStayRelative ./cmd/evolve` |
| cli-output | CLI prints `..notes.md:1` | 6/10 | `go test -run TestDocsSteLint_ADotDotPrefixedRootFileShowsItsRelativePath ./cmd/evolve` |
| indexed-verb | indexed width and precision verb is one token | 7/10 | `go test -run TestCheckGo_IndexedWidthAndPrecisionVerbsAreCodeTokens ./internal/stelint` |
