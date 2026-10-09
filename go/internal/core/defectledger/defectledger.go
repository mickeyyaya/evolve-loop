// Package defectledger holds the audit phase defect ledger and its continuation disposition gate.
// See docs/architecture/packages/internal-core-defectledger.md.
package defectledger

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

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

type Request struct {
	Cycle       int
	Workspace   string
	ProjectRoot string
	Worktree    string
}

type Rejection struct {
	Defects       []string
	Prescriptions []string
}

type Verdict struct {
	Diagnostics   []cyclestate.Diagnostic
	Blocked       bool
	LineageCycles []int
}

type Resolver func(evidence string, req Request) (ok bool, why string)

type LaneScopeReader func(workspace string) []string

type Ledger struct {
	laneScope LaneScopeReader
	resolve   Resolver
	signals   func() *signalcenter.Center
}

type Option func(*Ledger)

func New(laneScope LaneScopeReader, resolve Resolver, opts ...Option) *Ledger {
	l := &Ledger{laneScope: laneScope, resolve: resolve}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

func WithSignals(c func() *signalcenter.Center) Option {
	return func(l *Ledger) { l.signals = c }
}

func (l *Ledger) SignalsWired() bool { return l.center() != nil }

func (l *Ledger) center() *signalcenter.Center {
	if l.signals == nil {
		return nil
	}
	return l.signals()
}

func (l *Ledger) emit(origin string, req Request, code signalcenter.Code, reason string, fields map[string]string) {
	l.center().Emit(signalcenter.Event{
		Cycle: req.Cycle, Phase: string(cyclestate.PhaseAudit), Module: signalcenter.ModuleAudit, Origin: origin,
		Kind: signalcenter.KindAuditWarning, Severity: codeSeverity[code], Code: code, Reason: reason, Fields: fields,
	})
}

const idListCap = 8

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
