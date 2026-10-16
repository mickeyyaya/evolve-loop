# Comment history: `internal/docsfloor`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d9

### `go/internal/docsfloor/docsfloor.go:1` — above `package docsfloor`

```text
// Package docsfloor mechanizes operating-policy §3 rule 2's documentation half:
// an architecture-labeled change must touch documentation.
//
// The prose rule ("architecture changes additionally get an adversarial
// architect review"; `always_full_documentation` / `doc_stewardship_policy`)
// had zero compiled enforcement — a change could rewrite the trust kernel and
// ship with no ADR, no design doc, nothing. This package is the SpineFloor-
// shaped counterpart for that rule: a pure decision function, stage-dialed from
// `.evolve/policy.json` (`docs_floor.stage`, compiled default "enforce"), whose
// verdict the build handoff floor surfaces.
//
// Deliberately WARN, never REJECT. "Is this doc adequate" is editorial; only
// "is there a doc at all" is mechanical, so the gate reports the mechanical
// half loudly and leaves the judgement to the auditor. Design: ADR-0077.
//
// Zero intra-repo imports (stdlib only) so every layer — core, policy
// consumers, CLI — can depend on it without an import cycle.
```

### `go/internal/docsfloor/docsfloor.go:56` — above `var docPrefixes = []string{"docs/"}`

```text
// docPrefixes are the repo-relative path prefixes that count as a
// documentation touch. Kept narrow on purpose: ADRs, design docs and operating
// policy all live under docs/, so one prefix covers the whole surface without
// letting an incidental README edit anywhere in the tree satisfy the floor.
```

### `go/internal/docsfloor/docsfloor.go:94` — above `var strictArchitectureSurfaces = append(append([]string{}, architectureSurfaces...),`

```text
// strictArchitectureSurfaces extends architectureSurfaces with the trust-kernel
// surfaces that carry no vocabulary of their own but decide what ships: the
// guard chain, the ship path and the fleet scheduler/composer. LabelArchitecture
// deliberately stays narrower (it drives the WARN-only ADR-0077 verdict on the
// broad "did this touch the kernel" question); this list backs
// IsArchitectureClass, the blocking-grade classification, which pays for the
// extra surfaces with the precision rules below.
```

### `go/internal/docsfloor/docsfloor.go:154` — above `func HasDocsDelta(changedFiles []string) bool {`

```text
// HasDocsDelta reports whether a change set touches any DocsRoots path — the
// mechanical "is there a doc at all" half of the floor. Whether the doc is
// ADEQUATE stays the auditor's editorial call (ADR-0077).
```

### `go/internal/docsfloor/docsfloor_test.go:180` — above `func TestHasDocsDelta(t *testing.T) {`

```text
// TestHasDocsDelta pins what satisfies the floor: the architecture docs tree
// (DocsRoots[0]) or the operator runtime reference (DocsRoots[1]) — and nothing
// else, so an incidental doc edit elsewhere cannot buy off an ADR.
```
