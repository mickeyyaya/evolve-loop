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
package docsfloor

import (
	"fmt"
	"strings"
)

// Verdict statuses. A gate that cannot apply reports StatusSkip rather than a
// vacuous pass, so "not judged" is never read as "judged clean".
const (
	StatusPass = "PASS"
	StatusWarn = "WARN"
	StatusSkip = "SKIP"
)

// Config is the gate's injected configuration. Stage is the rollout dial:
// "off" disables the gate entirely; "shadow" and "enforce" both evaluate
// (the floor's action is WARN at either stage — the split exists so a future
// promotion to a blocking verdict has a dial to turn). An empty Stage is
// treated as "enforce", matching policy.DocsFloorConfig's compiled default.
type Config struct {
	Stage string
}

// Input is the change under judgement: whether it is architecture-labeled and
// the repo-relative paths it touched.
type Input struct {
	ArchitectureLabeled bool
	ChangedFiles        []string
}

// Verdict is the gate's decision. Reason is always non-empty for StatusWarn —
// an unexplained warning is unactionable.
type Verdict struct {
	Status string
	Reason string
}

// docPrefixes are the repo-relative path prefixes that count as a
// documentation touch. Kept narrow on purpose: ADRs, design docs and operating
// policy all live under docs/, so one prefix covers the whole surface without
// letting an incidental README edit anywhere in the tree satisfy the floor.
var docPrefixes = []string{"docs/"}

// architectureSurfaces are the repo-relative prefixes whose modification makes
// a change architecture-labeled: the trust kernel (orchestrator state machine,
// policy resolution, routing config) plus the contract registry that every
// phase's vocabulary derives from. Touching these is what "architecture change"
// means operationally — it is derived from the diff rather than from an agent's
// self-report, because a self-reported label is exactly what a rushed change
// omits.
var architectureSurfaces = []string{
	"go/internal/core/",
	"go/internal/policy/",
	"go/internal/config/",
	"go/internal/phasecontract/",
	"go/internal/router/",
	"docs/architecture/phase-registry.json",
}

// LabelArchitecture reports whether a change set touches an architecture
// surface. Callers pass its result as Input.ArchitectureLabeled; it is exported
// so the label and the judgement stay separable (a caller with a better signal
// — an explicit operator label, say — can supply its own).
func LabelArchitecture(changedFiles []string) bool {
	for _, f := range changedFiles {
		p := normalizePath(f)
		for _, s := range architectureSurfaces {
			if strings.HasPrefix(p, s) {
				return true
			}
		}
	}
	return false
}

// strictArchitectureSurfaces extends architectureSurfaces with the trust-kernel
// surfaces that carry no vocabulary of their own but decide what ships: the
// guard chain, the ship path and the fleet scheduler/composer. LabelArchitecture
// deliberately stays narrower (it drives the WARN-only ADR-0077 verdict on the
// broad "did this touch the kernel" question); this list backs
// IsArchitectureClass, the blocking-grade classification, which pays for the
// extra surfaces with the precision rules below.
var strictArchitectureSurfaces = append(append([]string{}, architectureSurfaces...),
	"go/internal/guards/",
	"go/internal/ship/",
	"go/internal/fleet/",
	"go/internal/phases/ship/",
)

// IsArchitectureClass reports whether a change set is architecture-CLASS: the
// blocking-grade half of the docs floor, precise enough that a missing doc is a
// confirmed violation rather than a warning. It differs from LabelArchitecture
// in three ways, each of which removes a false positive that would otherwise
// block an ordinary cycle:
//
//  1. `_test.go` files never label. A test-only change to policy adds no
//     vocabulary and documents nothing new — demanding an ADR for it would tax
//     every regression test.
//  2. A package's namesake file (`go/internal/<pkg>/<pkg>.go`) labels even
//     outside the declared surfaces. A pure path classifier cannot see that a
//     package is NEW, and the namesake file is the deterministic proxy: it is
//     created exactly once, when the package is born.
//  3. A phase spec under `phases/` labels. Specs are vocabulary — a minted
//     phase with no doc is the same gap as an undocumented policy key.
//
// Fail-open by construction: anything not matched is not architecture-class, so
// bugfix, test-only and docs-only cycles are untouched.
func IsArchitectureClass(changedFiles []string) bool {
	for _, f := range changedFiles {
		p := normalizePath(f)
		if p == "" || strings.HasSuffix(p, "_test.go") {
			continue
		}
		for _, s := range strictArchitectureSurfaces {
			if strings.HasPrefix(p, s) {
				return true
			}
		}
		if isPackageNamesake(p) {
			return true
		}
		if strings.HasPrefix(p, "phases/") && strings.HasSuffix(p, ".json") {
			return true
		}
	}
	return false
}

// DocsRoots are the repo-relative paths that satisfy the blocking-grade floor:
// anything under the architecture docs tree (an ADR, a design doc,
// control-flags.md) or the operator runtime reference. Narrower than
// docPrefixes on purpose — a release-note or README edit is not a record of an
// architecture decision.
var DocsRoots = []string{"docs/architecture/", "docs/operations/runtime-reference.md"}

// HasDocsDelta reports whether a change set touches any DocsRoots path — the
// mechanical "is there a doc at all" half of the floor. Whether the doc is
// ADEQUATE stays the auditor's editorial call (ADR-0077).
func HasDocsDelta(changedFiles []string) bool {
	for _, f := range changedFiles {
		p := normalizePath(f)
		for _, root := range DocsRoots {
			if strings.HasPrefix(p, root) {
				return true
			}
		}
	}
	return false
}

// isPackageNamesake reports whether p is a Go package's namesake file under
// go/internal — `go/internal/<pkg>/<pkg>.go`. See IsArchitectureClass rule 2.
func isPackageNamesake(p string) bool {
	const root = "go/internal/"
	if !strings.HasPrefix(p, root) || !strings.HasSuffix(p, ".go") {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(p, root), "/")
	if len(parts) < 2 {
		return false
	}
	return parts[len(parts)-1] == parts[len(parts)-2]+".go"
}

// Evaluate applies the docs floor to one change.
//
// Decision order: stage off ⇒ SKIP; not architecture-labeled ⇒ SKIP; empty
// change set ⇒ SKIP (nothing to judge — an unreadable or empty diff must not
// manufacture a warning); ≥1 documentation touch ⇒ PASS; otherwise WARN.
func Evaluate(cfg Config, in Input) Verdict {
	stage := strings.ToLower(strings.TrimSpace(cfg.Stage))
	if stage == "off" {
		return Verdict{Status: StatusSkip, Reason: "docs floor stage is off"}
	}
	if !in.ArchitectureLabeled {
		return Verdict{Status: StatusSkip, Reason: "change is not architecture-labeled"}
	}
	if len(in.ChangedFiles) == 0 {
		return Verdict{Status: StatusSkip, Reason: "empty change set — nothing to judge"}
	}
	for _, f := range in.ChangedFiles {
		p := normalizePath(f)
		for _, prefix := range docPrefixes {
			if strings.HasPrefix(p, prefix) {
				return Verdict{
					Status: StatusPass,
					Reason: fmt.Sprintf("architecture change documented by %s", p),
				}
			}
		}
	}
	return Verdict{
		Status: StatusWarn,
		Reason: fmt.Sprintf(
			"architecture-labeled change touches %d file(s) but no %s file — add the ADR or design doc that explains the change (operating-policy §3.2)",
			len(in.ChangedFiles), strings.Join(docPrefixes, "/")),
	}
}

// normalizePath makes a path comparable to the prefix lists: forward slashes,
// no leading "./" or "/".
func normalizePath(p string) string {
	p = strings.ReplaceAll(strings.TrimSpace(p), "\\", "/")
	p = strings.TrimPrefix(p, "./")
	return strings.TrimPrefix(p, "/")
}
