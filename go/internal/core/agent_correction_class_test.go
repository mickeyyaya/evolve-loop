package core

import (
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentclass"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/phaseidentity"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/testaudit"
)

var (
	backtickSpan  = regexp.MustCompile("`([^`]+)`")
	selfCheckSpan = regexp.MustCompile(`<self-check>(.+?)</self-check>`)
	runFirstSpan  = regexp.MustCompile(`Run (evolve [^.]+?) first`)
)

func correctionCommands(text string) []string {
	var out []string
	for _, re := range []*regexp.Regexp{backtickSpan, selfCheckSpan, runFirstSpan} {
		for _, m := range re.FindAllStringSubmatch(text, -1) {
			if cmd := strings.TrimSpace(m[1]); strings.HasPrefix(cmd, "evolve ") {
				out = append(out, cmd)
			}
		}
	}
	return out
}

func notClassA(commands []string) []string {
	var bad []string
	for _, cmd := range commands {
		if agentclass.Of(strings.Fields(cmd)[1:]).Class != agentclass.Allowed {
			bad = append(bad, cmd)
		}
	}
	return bad
}

func agentFacingCorrectionTexts(t *testing.T) map[string]string {
	t.Helper()
	triage, ok := phasecontract.For("triage")
	if !ok {
		t.Fatal("no triage contract")
	}
	texts := map[string]string{
		"agentclass.Brief":             agentclass.Brief("/ws"),
		"RenderContractBlock":          phasecontract.RenderContractBlock(triage),
		"RenderContractTail":           phasecontract.RenderContractTail(triage, "/ws/triage-report.md", "/ws"),
		"selfCheckFindingsMessage":     selfCheckFindingsMessage([]string{"a finding"}),
		"testAuditLine stale row":      testAuditLine(&testaudit.RowError{Kind: testaudit.RowStale, Finding: testaudit.Finding{Package: "internal/x", Test: "TestX", Rule: "r"}}),
		"testAuditLine missing row":    testAuditLine(&testaudit.RowError{Kind: testaudit.RowMissing, Finding: testaudit.Finding{Package: "internal/x", Test: "TestX", Rule: "r"}}),
		"steLintRemedy":                steLintRemedy,
		"graduation prescription":      ciparity.GraduationPrescription([]string{"./internal/x"}),
		"phaseidentity.Authority":      phaseidentity.Authority(),
		"audit gate remedy: gofmt":     AuditGateRemedy("gofmt"),
		"audit gate remedy: solution":  AuditGateRemedy("solution-contract"),
		"audit gate remedy: skills":    AuditGateRemedy("skills-drift"),
		"audit gate remedy: apicover":  AuditGateRemedy("apicover-enforce gate"),
		"audit gate remedy: graduated": AuditGateRemedy("apicover new-package graduation gate"),
	}
	return texts
}

func TestAgentCorrections_AreClassA(t *testing.T) {
	t.Parallel()
	for name, text := range agentFacingCorrectionTexts(t) {
		commands := correctionCommands(text)
		if bad := notClassA(commands); len(bad) > 0 {
			t.Errorf("%s names a command that a cell may not run: %q (class is not A)", name, bad)
		}
	}
}

func TestAgentCorrections_TheExtractorFindsEachNamedCommand(t *testing.T) {
	t.Parallel()
	texts := agentFacingCorrectionTexts(t)
	want := map[string]string{
		"RenderContractTail":       "evolve phase verify triage --workspace <your workspace dir>",
		"agentclass.Brief":         "evolve agent capabilities --json",
		"selfCheckFindingsMessage": "evolve selfcheck build",
	}
	for name, cmd := range want {
		found := false
		for _, got := range correctionCommands(texts[name]) {
			found = found || got == cmd
		}
		if !found {
			t.Errorf("%s: the extractor did not find %q in %v", name, cmd, correctionCommands(texts[name]))
		}
	}
}

func TestAgentCorrections_NegativeControlsFlagAStateChangingVerb(t *testing.T) {
	t.Parallel()
	bad := notClassA(correctionCommands("run `evolve test audit --rebaseline x` and `evolve gc`"))
	if len(bad) != 2 {
		t.Errorf("flagged %q, want both state-changing verbs", bad)
	}
	prose := "The evolve verbs of this cell. Run a class A verb for each evolve verb."
	if cmds := correctionCommands(prose); len(cmds) != 0 {
		t.Errorf("prose parsed as commands: %q", cmds)
	}
}
