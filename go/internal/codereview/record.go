package codereview

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualityindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeFindings signalcenter.Code = "REVIEW_FINDINGS"
	CodeSkipped  signalcenter.Code = "REVIEW_SKIPPED"
)

const SkipMalformed = "malformed"

var severityFields = []string{"CRITICAL", "HIGH", "MEDIUM", "LOW"}

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleReview, CodeFindings, "a code-review dispatch completed and its findings were recorded in the cycle's defect-ledger.json (source code-review); INFO, or WARN when the rows could not be recorded (fields.error) or the findings_repair or quality_index config fell back (fields.config_warning); fields.findings (counts by severity), verdict, would_repair (any finding or a scores gap: the round-1 projection of the loop's decision), stage, threshold, round, recorded, scores (the quality-index vector), gaps (empty when the scores qualify), overflow")
	signalcenter.RegisterCode(signalcenter.ModuleReview, CodeSkipped, "a code-review dispatch was skipped and recorded nothing, so the walk continued and the audit stays the only gate; WARN; fields.reason malformed (the contract-correction ladder exhausted on the report, or its verdict stayed non-canonical after every retry), fields.round")
}

type Request struct {
	Workspace    string
	Cycle, Round int
}

type Settings struct {
	Repair policy.JudgeRepairConfig
	Index  policy.QualityIndexConfig
}

type Outcome struct {
	Request
	Settings Settings
	Findings []Finding
	Scores   qualityindex.Scores
	Overflow int
	Err      error
}

func Record(req Request, cfg Settings) Outcome {
	out := Outcome{Request: req, Settings: cfg}
	raw, err := os.ReadFile(filepath.Join(req.Workspace, phasecontract.ArtifactFilename(PhaseName)))
	if err != nil {
		out.Err = fmt.Errorf("read the review report: %w", err)
		return out
	}
	out.Findings = Parse(string(raw))
	out.Scores, _ = qualityindex.ParseScores(string(raw))
	if len(out.Findings) == 0 {
		return out
	}
	out.Overflow, out.Err = appendRows(req, Rows(out.Findings, req.Round))
	return out
}

func appendRows(req Request, rows []defectledger.Entry) (int, error) {
	doc, existed, err := defectledger.Read(req.Workspace)
	if err != nil {
		return 0, fmt.Errorf("read the defect ledger: %w", err)
	}
	if !existed {
		doc.OriginCycle = req.Cycle
	}
	doc, added, overflow := defectledger.Append(doc, rows, req.Cycle)
	if !added {
		return overflow, nil
	}
	if err := defectledger.Write(req.Workspace, doc); err != nil {
		return overflow, fmt.Errorf("write the defect ledger: %w", err)
	}
	return overflow, nil
}

func (o Outcome) Event() signalcenter.Event {
	verdict := Verdict(o.Findings, o.Settings.Repair.Threshold)
	qualifies, gaps := qualityindex.Qualifies(o.Scores, o.Settings.Index.Thresholds)
	fields := map[string]string{"findings": o.severityCounts(), "verdict": verdict, "would_repair": strconv.FormatBool(len(o.Findings) > 0 || !qualifies)}
	fields["stage"], fields["threshold"], fields["round"] = o.Settings.Repair.Stage, o.Settings.Repair.Threshold, strconv.Itoa(o.Round)
	fields["scores"], fields["gaps"], fields["recorded"] = scoreVector(o.Scores), gapList(gaps), strconv.FormatBool(o.Err == nil)
	severity := signalcenter.SeverityInfo
	if o.Err != nil {
		fields["error"], severity = o.Err.Error(), signalcenter.SeverityWarn
	}
	if warnings := append(append([]string(nil), o.Settings.Repair.Warnings...), o.Settings.Index.Warnings...); len(warnings) > 0 {
		fields["config_warning"], severity = strings.Join(warnings, "; "), signalcenter.SeverityWarn
	}
	if o.Overflow > 0 {
		fields["overflow"] = strconv.Itoa(o.Overflow)
	}
	return signalcenter.Event{
		Cycle: o.Cycle, Phase: PhaseName, Module: signalcenter.ModuleReview, Origin: "codereview.Record",
		Kind: signalcenter.KindPhaseOutcome, Severity: severity, Code: CodeFindings,
		Reason: fmt.Sprintf("code-review round %d: %d finding(s), verdict %s at threshold %s, scores qualify %t (stage %s)", o.Round, len(o.Findings), verdict, o.Settings.Repair.Threshold, qualifies, o.Settings.Repair.Stage),
		Fields: fields,
	}
}

func Skipped(req Request, reason string) signalcenter.Event {
	return signalcenter.Event{
		Cycle: req.Cycle, Phase: PhaseName, Module: signalcenter.ModuleReview, Origin: "codereview.Skipped",
		Kind: signalcenter.KindPhaseOutcome, Severity: signalcenter.SeverityWarn, Code: CodeSkipped,
		Reason: fmt.Sprintf("code-review round %d skipped (%s): nothing was recorded and the walk continued", req.Round, reason),
		Fields: map[string]string{"reason": reason, "round": strconv.Itoa(req.Round)},
	}
}

func (o Outcome) severityCounts() string {
	counts := map[string]int{}
	for _, f := range o.Findings {
		counts[f.Severity]++
	}
	parts := make([]string, len(severityFields))
	for i, severity := range severityFields {
		parts[i] = strings.ToLower(severity) + "=" + strconv.Itoa(counts[severity])
	}
	return strings.Join(parts, ",")
}

func scoreVector(scores qualityindex.Scores) string {
	if len(scores) == 0 {
		return ""
	}
	parts := make([]string, 0, len(scores))
	for _, key := range qualityindex.Keys() {
		s, ok := scores[key]
		switch {
		case !ok:
		case s.NA:
			parts = append(parts, key+"=N/A")
		default:
			parts = append(parts, key+"="+strconv.Itoa(s.Value))
		}
	}
	return strings.Join(parts, ",")
}

func gapList(gaps []qualityindex.Gap) string {
	parts := make([]string, len(gaps))
	for i, g := range gaps {
		if g.Reason != "" {
			parts[i] = g.Dimension + "=" + strings.ReplaceAll(g.Reason, " ", "-")
			continue
		}
		parts[i] = fmt.Sprintf("%s=%d<%d", g.Dimension, g.Score, g.Threshold)
	}
	return strings.Join(parts, ",")
}
