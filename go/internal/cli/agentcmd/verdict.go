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

const CodeVerdictParseDefect signalcenter.Code = "REVIEW_AGENT_RUN_VERDICT_PARSE_DEFECT"

const verdictSection = "Verdict"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleReview, CodeVerdictMismatch, "evolve agent run found that the report body of a review fails while the verdict field passes: the body states FAIL, states no verdict, or lists a CRITICAL or HIGH finding; the run verdict is FAIL; run_id names the run, fields.field_verdict the field, fields.stated the stated verdict, fields.critical and fields.high the counts")
	signalcenter.RegisterCode(signalcenter.ModuleReview, CodeVerdictParseDefect, "evolve agent run found no stated verdict in a review report that lists no CRITICAL and no HIGH finding; the parser or the report format has a defect, and the run verdict stays FAIL; run_id names the run, fields.field_verdict the field verdict")
}

type derivedVerdict struct {
	verdict     string
	stated      string
	findings    reportdoc.SeverityCounts
	mismatch    bool
	parseDefect bool
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
		d.parseDefect = d.stated == "" && d.findings.Blocking() == 0
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
		out = out.withSignal(signals, mismatchEvent(r, field, d))
	}
	if d.parseDefect {
		out = out.withSignal(signals, parseDefectEvent(r, field))
	}
	return out
}

func (out launchOutcome) withSignal(signals *signalcenter.Center, event signalcenter.Event) launchOutcome {
	signals.Emit(event)
	out.resp.Diagnostics = append(slices.Clip(out.resp.Diagnostics), core.Diagnostic{Severity: "error", Message: event.Reason})
	return out
}

func parseDefectEvent(r standaloneRun, field string) signalcenter.Event {
	return signalcenter.Event{
		RunID: r.label, Phase: r.spec.Name, Module: signalcenter.ModuleReview, Origin: "agentcmd.reportVerdict",
		Kind: signalcenter.KindPhaseOutcome, Code: CodeVerdictParseDefect, Severity: signalcenter.SeverityWarn,
		Reason: fmt.Sprintf("agent run %s: the report body lists no CRITICAL and no HIGH finding, but the parser found no stated verdict; the run verdict is FAIL, so check the report format and the parser",
			r.label),
		Fields: map[string]string{"field_verdict": field},
	}
}

func (out launchOutcome) withLaunchVerdict() launchOutcome {
	session := out.cloudSession()
	out.verdict, out.resp.Verdict = core.VerdictPASS, core.VerdictPASS
	out.resp.Diagnostics = []core.Diagnostic{{Severity: "info", Message: fmt.Sprintf("cloud session %s %s: the deliverable is a later pull request, so the review contract of the profile does not apply", session.ID, session.Outcome)}}
	return out
}

func (out launchOutcome) withFenceVerdict() launchOutcome {
	if out.resp.Verdict != core.VerdictPASS || !slices.ContainsFunc(out.resp.Diagnostics, func(d core.Diagnostic) bool { return d.Severity == "error" }) {
		return out
	}
	out.verdict, out.resp.Verdict = core.VerdictFAIL, core.VerdictFAIL
	return out
}
