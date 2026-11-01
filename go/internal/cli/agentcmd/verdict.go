package agentcmd

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeVerdictMismatch signalcenter.Code = "REVIEW_AGENT_RUN_VERDICT_MISMATCH"

const verdictSection = "Verdict"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleReview, CodeVerdictMismatch, "evolve agent run found that the report body of a review fails while the verdict field passes: the body states FAIL, states no verdict, or lists a CRITICAL or HIGH finding; the run verdict is FAIL; run_id names the run, fields.field_verdict the field, fields.stated the stated verdict, fields.critical and fields.high the counts")
}

type derivedVerdict struct {
	verdict  string
	stated   string
	findings reportdoc.SeverityCounts
	mismatch bool
}

func declaresVerdict(spec phasespec.PhaseSpec) bool {
	return spec.Classify != nil && slices.ContainsFunc(spec.Classify.RequireSections, func(s string) bool {
		return strings.TrimSpace(strings.TrimLeft(s, "#")) == verdictSection
	})
}

func statedVerdict(report string) string {
	sentinel, _ := phasecontract.ParseVerdictSentinel(report)
	return cmp.Or(sentinel, reportdoc.Verdict(report))
}

func reportVerdict(spec phasespec.PhaseSpec, field, report string) derivedVerdict {
	if !declaresVerdict(spec) {
		return derivedVerdict{verdict: field}
	}
	d := derivedVerdict{verdict: field, stated: statedVerdict(report), findings: reportdoc.CountSeverities(report)}
	if field != core.VerdictPASS && field != core.VerdictWARN {
		return d
	}
	switch {
	case d.findings.Blocking() > 0, d.stated == core.VerdictFAIL, d.stated == "":
		d.verdict, d.mismatch = core.VerdictFAIL, true
	case d.stated == core.VerdictWARN:
		d.verdict = core.VerdictWARN
	}
	return d
}

func mismatchEvent(r standaloneRun, field string, d derivedVerdict) signalcenter.Event {
	stated := cmp.Or(d.stated, "no verdict")
	return signalcenter.Event{
		RunID: r.label, Phase: r.spec.Name, Module: signalcenter.ModuleReview, Origin: "agentcmd.reportVerdict",
		Kind: signalcenter.KindPhaseOutcome, Code: CodeVerdictMismatch, Severity: signalcenter.SeverityWarn,
		Reason: fmt.Sprintf("agent run %s: the verdict field is %s, but the report body states %s and lists %d CRITICAL and %d HIGH; the run verdict is FAIL",
			r.label, field, stated, d.findings.Critical, d.findings.High),
		Fields: map[string]string{
			"field_verdict": field, "stated": stated,
			"critical": strconv.Itoa(d.findings.Critical), "high": strconv.Itoa(d.findings.High),
		},
	}
}

func (out launchOutcome) withReportVerdict(r standaloneRun, signals *signalcenter.Center) launchOutcome {
	field := out.resp.Verdict
	d := reportVerdict(r.spec, field, out.report)
	out.verdict, out.findings, out.resp.Verdict = d.verdict, d.findings, d.verdict
	if d.mismatch {
		event := mismatchEvent(r, field, d)
		signals.Emit(event)
		out.resp.Diagnostics = append(slices.Clip(out.resp.Diagnostics), core.Diagnostic{Severity: "error", Message: event.Reason})
	}
	return out
}
