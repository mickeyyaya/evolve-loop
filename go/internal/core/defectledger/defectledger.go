// Package defectledger is unit 09 of the component breakdown (ADR-0103): the
// audit phase's anti-laundering record — the defect/prescription ledger's
// schema and vocabulary (the ONE home audit, carryover and the adoption seeder
// project from), its Center-free reader and writer, and the continuation
// disposition gate: EMIT (a rejecting audit's OPEN rows), ARM (is this cycle a
// continuation, and which record says so — the workspace manifest or the
// root-owned registry keyed by the lane-scope pin), GRADE (the disposition
// diff against the ancestor's ledger, written back BEFORE the verdict),
// PREFLIGHT (the artifact-level MISSING / INCOMPLETE finding) and the prompt
// block that tells a continuation its inherited ids.
//
// A Ledger takes its two hidden couplings as explicit collaborators — the
// lane-identity reader and the citation-resolution policy (a Strategy the
// audit package keeps) — and the Signal Center through an accessor. It never
// writes stderr, never persists beyond the two artifacts it owns, takes no
// context, and reports its failure modes as audit.warning under module audit
// while the graded diagnostics stay the byte-identical wire the dossier and the
// bookkeeping regrade read. Design:
// docs/architecture/decomposition/09-defectledger.md.
package defectledger

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// The unit's codes — the AUDIT_LEDGER_ family under the shared audit tag;
// fields.step names the gate step on every event, fields.blocked whether the
// diagnostic beside it forces the verdict.
const (
	CodeEmitFailed             signalcenter.Code = "AUDIT_LEDGER_EMIT_FAILED"
	CodeOverflow               signalcenter.Code = "AUDIT_LEDGER_OVERFLOW"
	CodeManifestUnreadable     signalcenter.Code = "AUDIT_LEDGER_MANIFEST_UNREADABLE"
	CodeManifestMissing        signalcenter.Code = "AUDIT_LEDGER_MANIFEST_MISSING"
	CodeLineageDisagrees       signalcenter.Code = "AUDIT_LEDGER_LINEAGE_DISAGREES"
	CodeLedgerUnreadable       signalcenter.Code = "AUDIT_LEDGER_UNREADABLE"
	CodeAncestorEmpty          signalcenter.Code = "AUDIT_LEDGER_ANCESTOR_EMPTY"
	CodeWritebackFailed        signalcenter.Code = "AUDIT_LEDGER_WRITEBACK_FAILED"
	CodeDefectsUnaccounted     signalcenter.Code = "AUDIT_LEDGER_DEFECTS_UNACCOUNTED"
	CodeDispositionsMissing    signalcenter.Code = "AUDIT_LEDGER_DISPOSITIONS_MISSING"
	CodeDispositionsIncomplete signalcenter.Code = "AUDIT_LEDGER_DISPOSITIONS_INCOMPLETE"
	CodeDispositionsUnreadable signalcenter.Code = "AUDIT_LEDGER_DISPOSITIONS_UNREADABLE"
	CodePromptDegraded         signalcenter.Code = "AUDIT_LEDGER_PROMPT_DEGRADED"
)

// codeSeverity is the schema-1.0 "severity fixed per code" table: the twelve
// faults WARN (one console line each beside the phase.outcome line), the
// prompt degrade INFO (stream-only — the same fault blocks at Classify under
// its own WARN code, so the console never doubles).
var codeSeverity = map[signalcenter.Code]signalcenter.Severity{
	CodeEmitFailed: signalcenter.SeverityWarn, CodeOverflow: signalcenter.SeverityWarn,
	CodeManifestUnreadable: signalcenter.SeverityWarn, CodeManifestMissing: signalcenter.SeverityWarn,
	CodeLineageDisagrees: signalcenter.SeverityWarn, CodeLedgerUnreadable: signalcenter.SeverityWarn,
	CodeAncestorEmpty: signalcenter.SeverityWarn, CodeWritebackFailed: signalcenter.SeverityWarn,
	CodeDefectsUnaccounted: signalcenter.SeverityWarn, CodeDispositionsMissing: signalcenter.SeverityWarn,
	CodeDispositionsIncomplete: signalcenter.SeverityWarn, CodeDispositionsUnreadable: signalcenter.SeverityWarn,
	CodePromptDegraded: signalcenter.SeverityInfo,
}

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleAudit, CodeEmitFailed, "a rejecting audit's defect ledger could not be read or written while recording this cycle's defects (fields.op = read | parse | write); the verdict stands, a later continuation has nothing to reconcile against — repair the workspace ledger; fields.step=emit, path")
	signalcenter.RegisterCode(signalcenter.ModuleAudit, CodeOverflow, "a rejecting audit carried more defects than the ledger cap; the first rows are recorded and ONE synthetic OPEN row stands for the truncated tail (fields.overflow, cap) — fix the emitter or widen the cap; fields.step=emit, path")
	signalcenter.RegisterCode(signalcenter.ModuleAudit, CodeManifestUnreadable, "the workspace continuation-manifest.json is present but unreadable, so the continuation cannot be graded against its lineage; the cycle is blocked from PASS (cycle-1285 F2) — repair the manifest; fields.step=arm, workspace")
	signalcenter.RegisterCode(signalcenter.ModuleAudit, CodeManifestMissing, "the root-owned continuation registry binds this lane's scope to an ancestor but the workspace holds no manifest — it was deleted or never written; inherited defects are reconciled from the registry binding and the cycle is blocked; the missing manifest is the finding; fields.step=arm, registry_path, ancestor_cycle")
	signalcenter.RegisterCode(signalcenter.ModuleAudit, CodeLineageDisagrees, "the workspace manifest and the root-owned registry name different ancestors; the rewritable copy is the suspect and the cycle is blocked until the disagreement is resolved; fields.step=arm, manifest_cycle, registry_cycle")
	signalcenter.RegisterCode(signalcenter.ModuleAudit, CodeLedgerUnreadable, "the ancestor's or this cycle's own defect-ledger.json is present but unreadable (fields.which = ancestor | own, op = read | parse); the continuation is blocked — repair the file named by fields.path; fields.step=grade, ancestor_cycle")
	signalcenter.RegisterCode(signalcenter.ModuleAudit, CodeAncestorEmpty, "the ancestor left no reconcilable defect-ledger.json (absent or empty), so NO inherited defect is enforced this cycle; expected for an ancestor that predates the ledger, but a deleted ledger looks identical — recorded, never assumed benign; fields.step=grade, ancestor_cycle, path")
	signalcenter.RegisterCode(signalcenter.ModuleAudit, CodeWritebackFailed, "the reconciled ledger could not be written back into the workspace; an invisible disposition is not a disposition, so the cycle is blocked; fields.step=grade, path")
	signalcenter.RegisterCode(signalcenter.ModuleAudit, CodeDefectsUnaccounted, "inherited defects are neither FIXED with resolving evidence nor DEFERRED with a reason (fields.count; the first ids in fields.ids, ids_truncated when more); the cycle is blocked — disposition each id in defect-dispositions.json (the per-id reasons are in the diagnostic and the written-back ledger); fields.step=grade, ancestor_cycle, path")
	signalcenter.RegisterCode(signalcenter.ModuleAudit, CodeDispositionsMissing, "a continuation owing dispositions holds no defect-dispositions.json at all (fields.open inherited OPEN ids); the cycle is blocked — author the file from scratch, one entry per inherited id; fields.step=preflight, ancestor_cycle, path")
	signalcenter.RegisterCode(signalcenter.ModuleAudit, CodeDispositionsIncomplete, "defect-dispositions.json covers only some inherited OPEN ids (fields.open, covered, the first uncovered ids, uncovered_truncated when more); the cycle is blocked — finish the file that exists; fields.step=preflight, ancestor_cycle, path")
	signalcenter.RegisterCode(signalcenter.ModuleAudit, CodeDispositionsUnreadable, "defect-dispositions.json is present but cannot be read or parsed (fields.op = read | parse; an object, number or bool evidence shape is a parse fault); the cycle is blocked — the diagnostic carries the expected schema; fields.step=read, path")
	signalcenter.RegisterCode(signalcenter.ModuleAudit, CodePromptDegraded, "while composing the audit prompt the continuation manifest (fields.reason=manifest; fallback = registry | none) or the ancestor ledger (fields.reason=ledger, op = read | parse) could not be read, so the auditor was NOT told its inherited ids; stream-only — the same fault blocks at Classify under its own WARN code; fields.step=prompt, path")
}

// Request is the Parameter Object the gate reads — exactly the four fields of
// the audit phase's request it ever touched. The audit seam projects it ONCE.
type Request struct {
	Cycle       int
	Workspace   string
	ProjectRoot string
	Worktree    string
}

// Rejection is a rejecting verdict's structured block: the defects and the
// prescriptions (a named fix for a foreseen risk, not something itself
// wrong). The audit seam parses the verdict sentinel into it.
type Rejection struct {
	Defects       []string
	Prescriptions []string
}

// Verdict is the Result Object of Reconcile and Emit: the diagnostics to
// surface (every message starts "defect ledger: " — the wire the bookkeeping
// regrade classifies; the leaf is their ONE author, the host appends them
// verbatim), whether the cycle must be blocked from PASS (never on Emit), and
// the lineage cycles whose closure claims a graded, unblocked reconcile
// vouches for.
type Verdict struct {
	Diagnostics   []cyclestate.Diagnostic
	Blocked       bool
	LineageCycles []int
}

// Resolver is the injected citation policy: does a closure claim's evidence
// name a real, in-repo, non-self file, and why not. The audit package owns
// the four-rule resolver; the grade never runs without one.
type Resolver func(evidence string, req Request) (ok bool, why string)

// LaneScopeReader is the lane-identity port: the pinned todo ids of the lane
// that owns workspace, nil when the pin is absent, malformed or empty (the
// recorded ceiling — no lineage can then be recovered from the registry).
type LaneScopeReader func(workspace string) []string

// Ledger owns the gate. Its collaborators are explicit and REQUIRED at
// construction: a forgotten reader would silently re-open the cycle-1285 F2
// deletion hole, a forgotten resolver would grade FIXED without touching
// disk — so a nil one panics at first use rather than disarming.
type Ledger struct {
	laneScope LaneScopeReader
	resolve   Resolver
	signals   func() *signalcenter.Center
}

// Option configures a Ledger at construction (functional options).
type Option func(*Ledger)

// New builds the gate over its two required collaborators.
func New(laneScope LaneScopeReader, resolve Resolver, opts ...Option) *Ledger {
	l := &Ledger{laneScope: laneScope, resolve: resolve}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// WithSignals installs the accessor of the Signal Center the unit reports
// through — read at every use, because a root's Center may be installed after
// the phase is built. A nil accessor, or one returning nil, is the Null Object.
func WithSignals(c func() *signalcenter.Center) Option {
	return func(l *Ledger) { l.signals = c }
}

// SignalsWired reports whether the ledger currently reaches a Center.
func (l *Ledger) SignalsWired() bool { return l.center() != nil }

func (l *Ledger) center() *signalcenter.Center {
	if l.signals == nil {
		return nil
	}
	return l.signals()
}

// emit is the unit's ONE producer: an audit.warning under module audit at the
// code's fixed severity, stamped with the cycle, the audit phase and the
// exported method that owns the diagnostic. A nil Center is a no-op.
func (l *Ledger) emit(origin string, req Request, code signalcenter.Code, reason string, fields map[string]string) {
	l.center().Emit(signalcenter.Event{
		Cycle: req.Cycle, Phase: string(cyclestate.PhaseAudit), Module: signalcenter.ModuleAudit, Origin: origin,
		Kind: signalcenter.KindAuditWarning, Severity: codeSeverity[code], Code: code, Reason: reason, Fields: fields,
	})
}

// idListCap bounds the id lists a signal carries: 64 ids × 33 runes would
// breach the line cap and Normalize would cut silently; the full list lives in
// the diagnostic and the written-back ledger.
const idListCap = 8

// boundedIDs joins at most idListCap ids and says whether more were dropped.
func boundedIDs(ids []string) (head, truncated string) {
	truncated = "false"
	if len(ids) > idListCap {
		ids, truncated = ids[:idListCap], "true"
	}
	return strings.Join(ids, ", "), truncated
}

func errorDiag(message string) cyclestate.Diagnostic {
	return cyclestate.Diagnostic{Severity: "error", Message: message}
}

func warningDiag(message string) cyclestate.Diagnostic {
	return cyclestate.Diagnostic{Severity: "warning", Message: message}
}

func blockedOn(message string) Verdict {
	return Verdict{Diagnostics: []cyclestate.Diagnostic{errorDiag(message)}, Blocked: true}
}
