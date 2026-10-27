package agentcmd

import (
	"bytes"
	"encoding/json"
	"io"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/launchrecord"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const threatModel = "## Threat Model\nA diff that adds one function.\n"

func runWithSignals(t *testing.T, report string) (Envelope, int, *signalcenter.Center, string) {
	t.Helper()
	root := project(t)
	center := signalcenter.New()
	deps := depsWith(&fakeBridge{artifact: report})
	deps.Signals = func(string, io.Writer) (*signalcenter.Center, func()) { return center, func() {} }
	var stdout, stderr bytes.Buffer
	code := New(deps)(runArgs(root, "--json"), strings.NewReader(diff), &stdout, &stderr)
	var env Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout is not one JSON envelope: %v\n%s", err, stdout.String())
	}
	return env, code, center, root
}

func mismatchSignals(center *signalcenter.Center) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range center.Recent() {
		if e.Code == CodeVerdictMismatch {
			out = append(out, e)
		}
	}
	return out
}

func TestAgentRun_AReportThatListsOneHighIsAFailVerdictThoughTheFieldSaysPass(t *testing.T) {
	report := threatModel + "## Findings\n- [HIGH] go/x.go:1 — the error is dropped — wrap it\n## Verdict\nPASS\n"

	env, code, center, root := runWithSignals(t, report)

	if env.Verdict != core.VerdictFAIL || code != exitFail {
		t.Fatalf("verdict %q exit %d, want FAIL and exit 1", env.Verdict, code)
	}
	if env.Findings != (reportdoc.SeverityCounts{High: 1}) {
		t.Errorf("envelope findings = %+v, want one HIGH", env.Findings)
	}
	signals := mismatchSignals(center)
	if len(signals) != 1 || signals[0].RunID != env.RunLabel || !strings.Contains(signals[0].Reason, env.RunLabel) {
		t.Fatalf("mismatch signals = %+v, want one that names the run %s", signals, env.RunLabel)
	}
	wantFields := map[string]string{"field_verdict": "PASS", "stated": "PASS", "critical": "0", "high": "1"}
	if !maps.Equal(signals[0].Fields, wantFields) {
		t.Errorf("signal fields = %+v, want %+v", signals[0].Fields, wantFields)
	}
	if !slices.Contains(env.Diagnostics, core.Diagnostic{Severity: "error", Message: signals[0].Reason}) {
		t.Errorf("diagnostics = %+v, want the error diagnostic of the mismatch", env.Diagnostics)
	}
	rec, err := launchrecord.Read(filepath.Join(root, ".evolve"), env.RunLabel)
	if err != nil || rec.Verdict != core.VerdictFAIL {
		t.Errorf("launch record verdict %q err=%v, want FAIL so that commit-gate refuses the run", rec.Verdict, err)
	}
	if entries := ledgerEntries(t, root); len(entries) != 1 || entries[0].ExitCode != exitFail {
		t.Errorf("ledger = %+v, want one entry with exit 1", entries)
	}
}

func TestAgentRun_AReportThatStatesFailIsAFailVerdict(t *testing.T) {
	env, code, center, _ := runWithSignals(t, threatModel+"## Findings\nNone.\n## Verdict\nVerdict: FAIL\n")

	if env.Verdict != core.VerdictFAIL || code != exitFail || len(mismatchSignals(center)) != 1 {
		t.Errorf("verdict %q exit %d signals %d, want FAIL, exit 1 and one mismatch signal", env.Verdict, code, len(mismatchSignals(center)))
	}
}

func TestAgentRun_AVerdictSectionWithNoStatedVerdictIsAFailVerdict(t *testing.T) {
	env, code, center, _ := runWithSignals(t, threatModel+"## Findings\nNone.\n## Verdict\nIt is a PASS for me\n")

	if env.Verdict != core.VerdictFAIL || code != exitFail || len(mismatchSignals(center)) != 1 {
		t.Errorf("verdict %q exit %d signals %d, want FAIL, exit 1 and one mismatch signal", env.Verdict, code, len(mismatchSignals(center)))
	}
}

func TestAgentRun_AReportThatStatesWarnIsAWarnVerdictWithNoSignal(t *testing.T) {
	env, code, center, _ := runWithSignals(t, threatModel+"## Findings\n- [MEDIUM] go/x.go:1 — a name — rename\n## Verdict\nVerdict: WARN\n")

	if env.Verdict != core.VerdictWARN || code != 0 || len(mismatchSignals(center)) != 0 {
		t.Errorf("verdict %q exit %d signals %d, want WARN, exit 0 and no mismatch signal", env.Verdict, code, len(mismatchSignals(center)))
	}
	if env.Findings != (reportdoc.SeverityCounts{Medium: 1}) {
		t.Errorf("envelope findings = %+v, want one MEDIUM", env.Findings)
	}
}

func TestAgentRun_AMatchingPassReportKeepsPassWithNoSignal(t *testing.T) {
	env, code, center, _ := runWithSignals(t, passReport)

	if env.Verdict != core.VerdictPASS || code != 0 || len(mismatchSignals(center)) != 0 {
		t.Errorf("verdict %q exit %d signals %d, want PASS, exit 0 and no signal", env.Verdict, code, len(mismatchSignals(center)))
	}
}

func TestReportVerdict_AProfileWithNoVerdictSectionKeepsTheFieldVerdict(t *testing.T) {
	spec := phasespec.PhaseSpec{Name: "code-simplifier", Classify: &phasespec.ClassifyRules{RequireSections: []string{"Simplifications"}}}

	got := reportVerdict(spec, core.VerdictPASS, "## Simplifications\n[HIGH] not a review finding\n")

	if got.verdict != core.VerdictPASS || got.mismatch {
		t.Errorf("reportVerdict = %+v, want the field PASS and no mismatch", got)
	}
}

func TestReportVerdict_AFieldFailStaysFailWithNoMismatch(t *testing.T) {
	spec := phasespec.PhaseSpec{Name: "go-reviewer", Classify: &phasespec.ClassifyRules{RequireSections: []string{"Findings", "Verdict"}}}

	got := reportVerdict(spec, core.VerdictFAIL, "## Findings\nNone.\n## Verdict\nPASS\n")

	if got.verdict != core.VerdictFAIL || got.mismatch {
		t.Errorf("reportVerdict = %+v, want FAIL and no mismatch", got)
	}
}

func TestAgentRun_ADispatchErrorEmitsNoMismatchSignal(t *testing.T) {
	root := project(t)
	center := signalcenter.New()
	deps := depsWith(&fakeBridge{artifact: threatModel + "## Findings\n[HIGH] a\n## Verdict\nPASS\n", err: errBridgeDown})
	deps.Signals = func(string, io.Writer) (*signalcenter.Center, func()) { return center, func() {} }
	var stdout, stderr bytes.Buffer

	code := New(deps)(runArgs(root), strings.NewReader(diff), &stdout, &stderr)

	if code != exitDispatch || len(mismatchSignals(center)) != 0 {
		t.Errorf("exit %d signals %d, want 21 and no mismatch signal", code, len(mismatchSignals(center)))
	}
}

func TestReportVerdict_AVerdictStatedOnlyInTheSentinelCounts(t *testing.T) {
	spec := phasespec.PhaseSpec{Name: "go-reviewer", Classify: &phasespec.ClassifyRules{RequireSections: []string{"Findings", "Verdict"}}}
	report := "## Findings\nNone.\n## Verdict\n<!-- evolve-verdict: {\"phase\":\"go-reviewer\",\"verdict\":\"PASS\",\"schema_version\":1} -->\n"

	got := reportVerdict(spec, core.VerdictPASS, report)

	if got.verdict != core.VerdictPASS || got.stated != core.VerdictPASS || got.mismatch {
		t.Errorf("reportVerdict = %+v, want the stated PASS of the sentinel and no mismatch", got)
	}
}

func TestReportVerdict_AFieldWarnWithABlockingFindingIsFail(t *testing.T) {
	spec := phasespec.PhaseSpec{Name: "go-reviewer", Classify: &phasespec.ClassifyRules{RequireSections: []string{"Findings", "Verdict"}}}

	got := reportVerdict(spec, core.VerdictWARN, "## Findings\n- [HIGH] a.go:1 — x\n## Verdict\nWARN\n")

	if got.verdict != core.VerdictFAIL || !got.mismatch {
		t.Errorf("reportVerdict = %+v, want FAIL and a mismatch", got)
	}
}

func TestReportVerdict_TheSentinelWinsOverAProseVerdictLine(t *testing.T) {
	spec := phasespec.PhaseSpec{Name: "go-reviewer", Classify: &phasespec.ClassifyRules{RequireSections: []string{"Findings", "Verdict"}}}
	report := "## Findings\nNone.\n## Verdict\nVerdict: PASS\n<!-- evolve-verdict: {\"phase\":\"go-reviewer\",\"verdict\":\"FAIL\",\"schema_version\":1} -->\n"

	got := reportVerdict(spec, core.VerdictPASS, report)

	if got.verdict != core.VerdictFAIL || got.stated != core.VerdictFAIL || !got.mismatch {
		t.Errorf("reportVerdict = %+v, want the sentinel FAIL over the prose PASS", got)
	}
}

func TestReportVerdict_AFieldFailIsKeptWhateverTheBodySays(t *testing.T) {
	spec := phasespec.PhaseSpec{Name: "go-reviewer", Classify: &phasespec.ClassifyRules{RequireSections: []string{"Findings", "## Verdict"}}}
	for _, body := range []string{
		"## Findings\n- [MEDIUM] a.go:1 — x\n## Verdict\nVerdict: WARN\n",
		"## Findings\n- [HIGH] a.go:1 — x\n## Verdict\nVerdict: FAIL\n",
	} {
		got := reportVerdict(spec, core.VerdictFAIL, body)

		if got.verdict != core.VerdictFAIL || got.mismatch {
			t.Errorf("reportVerdict(FAIL, %q) = %+v, want FAIL and no mismatch", body, got)
		}
	}
}

func TestReportVerdict_AHeadingMarkedVerdictSectionDeclaresAReview(t *testing.T) {
	spec := phasespec.PhaseSpec{Name: "go-reviewer", Classify: &phasespec.ClassifyRules{RequireSections: []string{"## Findings", "## Verdict"}}}

	got := reportVerdict(spec, core.VerdictPASS, "## Findings\n- [HIGH] a.go:1 — x\n## Verdict\nPASS\n")

	if got.verdict != core.VerdictFAIL || !got.mismatch {
		t.Errorf("reportVerdict = %+v, want FAIL and a mismatch", got)
	}
}
