package triagecap

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// cardFilesReport's first top_n item declares its footprint; the second names a path in prose only.
const cardFilesReport = `<!-- challenge-token: abc -->
<!-- ANCHOR:triage_decision -->
# Triage Decision — Cycle 1167

cycle_size_estimate: small
phase_skip: []

## top_n (commit to THIS cycle)
- verdict-coherence-auditor-vs-egps: Reconcile the auditor verdict with EGPS — priority=H, files=go/internal/phases/audit/audit.go;go/internal/phases/audit/classify.go, source=scout
- surface-verdict-conflict-in-audit-classify: capture pre-override agent verdict in Classify (go/internal/phases/audit/audit.go) — priority=H, source=scout

## deferred (carry to NEXT cycle's carryoverTodos)
- ledger-seal-io-coverage: Cover writeSegment branches — priority=M, defer_reason=package variety

## Rationale
Two audit-surface items this cycle.
`

func TestProjectDecisionJSON_TopNCardsCarryDeclaredFiles(t *testing.T) {
	body, err := ProjectDecisionJSON(cardFilesReport, 1167)
	if err != nil {
		t.Fatalf("ProjectDecisionJSON: %v", err)
	}
	var got projectedDecision
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("projected JSON invalid: %v\n%s", err, body)
	}
	if len(got.TopN) != 2 {
		t.Fatalf("projected %d top_n cards, want 2: %s", len(got.TopN), body)
	}
	want := []string{"go/internal/phases/audit/audit.go", "go/internal/phases/audit/classify.go"}
	if len(got.TopN[0].Files) != len(want) {
		t.Fatalf("card %q projected files=%v, want %v — a file-less card is invisible to the "+
			"fleet disjointness planner", got.TopN[0].ID, got.TopN[0].Files, want)
	}
	for i := range want {
		if got.TopN[0].Files[i] != want[i] {
			t.Errorf("files[%d] = %q, want %q (declaration order preserved)", i, got.TopN[0].Files[i], want[i])
		}
	}
	if !strings.Contains(got.TopN[0].Action, "Reconcile the auditor verdict") {
		t.Errorf("card action = %q, want the prose preserved", got.TopN[0].Action)
	}
	if strings.Contains(got.TopN[0].Action, "files=") {
		t.Errorf("card action = %q leaked the files= metadata into the prose", got.TopN[0].Action)
	}
}

func TestProjectDecisionJSON_CardWithoutFilesInfersNothing(t *testing.T) {
	body, err := ProjectDecisionJSON(cardFilesReport, 1167)
	if err != nil {
		t.Fatalf("ProjectDecisionJSON: %v", err)
	}
	var got projectedDecision
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("projected JSON invalid: %v\n%s", err, body)
	}
	if len(got.TopN[1].Files) != 0 {
		t.Errorf("card %q projected files=%v from prose alone, want none — inferring a path at "+
			"projection time is worse than an island", got.TopN[1].ID, got.TopN[1].Files)
	}
	if strings.Contains(string(body), `"files": []`) {
		t.Errorf("projected an empty files array:\n%s", body)
	}
}

func TestProjectDecisionJSON_DeclaredFilesRejectMalformedTokens(t *testing.T) {
	report := strings.Replace(cardFilesReport,
		"files=go/internal/phases/audit/audit.go;go/internal/phases/audit/classify.go",
		"files=/etc/hosts;../outside.go;;go/internal/core/ok.go", 1)
	body, err := ProjectDecisionJSON(report, 1167)
	if err != nil {
		t.Fatalf("ProjectDecisionJSON: %v", err)
	}
	var got projectedDecision
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("projected JSON invalid: %v\n%s", err, body)
	}
	if len(got.TopN[0].Files) != 1 || got.TopN[0].Files[0] != "go/internal/core/ok.go" {
		t.Errorf("projected files=%v, want only [go/internal/core/ok.go] — absolute paths, `..` "+
			"escapes and empty tokens are not repo-relative footprints", got.TopN[0].Files)
	}
}

func TestMissingCardFilesWarning_NamesFilelessCardsThatCitePaths(t *testing.T) {
	msg := MissingCardFilesWarning(cardFilesReport, "")
	if msg == "" {
		t.Fatal("a file-less top_n card naming go/internal/phases/audit/audit.go in prose produced NO warning")
	}
	if !strings.Contains(msg, "surface-verdict-conflict-in-audit-classify") {
		t.Errorf("warning %q does not name the offending card id — an unattributable warning is unactionable", msg)
	}
	if strings.Contains(msg, "verdict-coherence-auditor-vs-egps") {
		t.Errorf("warning %q names the card that DID declare files= — a warning that fires on compliant cards trains the agent to ignore it", msg)
	}
	if !strings.Contains(msg, "files=") {
		t.Errorf("warning %q does not name the files= field the agent must add", msg)
	}
	if !strings.Contains(msg, "go/internal/phases/audit/audit.go") {
		t.Errorf("warning %q does not quote the path spotted in the prose — the agent needs to see what it already knows", msg)
	}
}

func TestMissingCardFilesWarning_SilentWhenNothingToSay(t *testing.T) {
	if msg := MissingCardFilesWarning(compliantCardFilesReport(), ""); msg != "" {
		t.Errorf("all cards declare files=, yet warning = %q", msg)
	}

	// The card's only path is its contract-required evidence= pointer.
	noPaths := `## top_n (commit to THIS cycle)
- research-token-frontier: read the vendor changelog and summarize — priority=L, evidence=go/internal/clihealth/clihealth.go, source=scout
`
	if msg := MissingCardFilesWarning(noPaths, ""); msg != "" {
		t.Errorf("a card whose only path is its contract-required evidence= pointer warned: %q", msg)
	}
	if msg := MissingCardFilesWarning("# no sections here", ""); msg != "" {
		t.Errorf("an artifact with no top_n section warned: %q", msg)
	}
	if msg := MissingCardFilesWarning("", ""); msg != "" {
		t.Errorf("an empty artifact warned: %q", msg)
	}
}

func TestMissingCardFilesWarning_UnusableDeclarationIsNotSilent(t *testing.T) {
	report := `## top_n (commit to THIS cycle)
- placeholder-card: do the thing — priority=H, files={repo/relative/path.go;second/path.go}, source=scout
`
	msg := MissingCardFilesWarning(report, "")
	if msg == "" {
		t.Fatal("a declaration whose every token is unusable produced NO warning — the card looks compliant and matches nothing")
	}
	if !strings.Contains(msg, "placeholder-card") || !strings.Contains(msg, "none of them a usable repo-relative path") {
		t.Errorf("warning = %q, want the id plus the reason the declaration was rejected", msg)
	}
}

func TestMissingCardFilesWarning_AgentCompanionIsTheAuthority(t *testing.T) {
	dir := t.TempDir()
	declared := filepath.Join(dir, "declared.json")
	writeCardCompanion(t, declared, `{"top_n":[{"id":"surface-verdict-conflict-in-audit-classify",`+
		`"action":"capture pre-override agent verdict in Classify (go/internal/phases/audit/audit.go)",`+
		`"files":["go/internal/phases/audit/audit.go"]}]}`)
	if msg := MissingCardFilesWarning(cardFilesReport, declared); msg != "" {
		t.Errorf("the companion declares files[] for the card, yet the report-based check still warned: %q", msg)
	}

	fileless := filepath.Join(dir, "fileless.json")
	writeCardCompanion(t, fileless, `{"top_n":[{"id":"surface-verdict-conflict-in-audit-classify",`+
		`"action":"capture pre-override agent verdict in Classify (go/internal/phases/audit/audit.go)"}]}`)
	msg := MissingCardFilesWarning("", fileless)
	if msg == "" || !strings.Contains(msg, "surface-verdict-conflict-in-audit-classify") {
		t.Errorf("the live cycle-1130 companion shape ({id, action}, no files[]) warned %q, want the card named", msg)
	}

	if msg := MissingCardFilesWarning(cardFilesReport, filepath.Join(dir, "absent.json")); msg == "" {
		t.Error("an absent companion silenced the report-based check")
	}
}

// compliantCardFilesReport is cardFilesReport with both cards declaring their footprint.
func compliantCardFilesReport() string {
	return strings.Replace(cardFilesReport,
		"- surface-verdict-conflict-in-audit-classify: capture pre-override agent verdict in Classify (go/internal/phases/audit/audit.go) — priority=H, source=scout",
		"- surface-verdict-conflict-in-audit-classify: capture pre-override agent verdict — priority=H, files=go/internal/phases/audit/audit.go, source=scout", 1)
}

func writeCardCompanion(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSplitDeclaredFiles_ToleratesTheSpellingsAgentsWrite(t *testing.T) {
	want := []string{"go/internal/core/a.go", "go/internal/bridge/b.go"}
	for _, rest := range []string{
		"do it — priority=H, files=go/internal/core/a.go;go/internal/bridge/b.go, source=scout",
		"do it — priority=H, files=go/internal/core/a.go; go/internal/bridge/b.go, source=scout",
		"do it — priority=H, files=go/internal/core/a.go, go/internal/bridge/b.go, source=scout",
		`do it — files=["go/internal/core/a.go", "go/internal/bridge/b.go"], source=scout`,
		"do it — files=go/internal/core/a.go, files=go/internal/bridge/b.go, source=scout",
		"do it — priority=H, files=go/internal/core/a.go go/internal/bridge/b.go",
	} {
		got := filesOf(rest)
		if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
			t.Errorf("filesOf(%q) = %v, want %v", rest, got, want)
		}
		if _, stripped := splitDeclaredFiles(rest); strings.Contains(stripped, "/a.go") || strings.Contains(stripped, "/b.go") {
			t.Errorf("stripped item %q still carries a declared path — the floor scanners would read it as a package mention", stripped)
		}
	}
}

func TestCapReviewer_WarnsOnFilelessCards(t *testing.T) {
	reviewWith := func(artifact string) (core.ReviewResult, string) {
		var logs []string
		r := newTestReviewer(config.StageEnforce, nil, nil)
		r.logf = func(f string, a ...any) { logs = append(logs, fmt.Sprintf(f, a...)) }
		rr := r.Review(context.Background(), reviewIn(writeTriageWorkspace(t, artifact)))
		return rr, strings.Join(logs, "\n")
	}

	rr, logged := reviewWith(cardFilesReport)
	if !strings.Contains(logged, "surface-verdict-conflict-in-audit-classify") || !strings.Contains(logged, "files=") {
		t.Errorf("Review logged %q, want a WARN naming the file-less card and the files= field", logged)
	}

	compliantRR, compliantLogged := reviewWith(compliantCardFilesReport())
	if strings.Contains(compliantLogged, "usable files= footprint") {
		t.Errorf("compliant report still WARNed: %q", compliantLogged)
	}
	if rr.Approve != compliantRR.Approve || rr.Reason != compliantRR.Reason {
		t.Errorf("the clamp verdict changed with the footprint declaration:\n file-less: approve=%v reason=%q\n declared: approve=%v reason=%q",
			rr.Approve, rr.Reason, compliantRR.Approve, compliantRR.Reason)
	}

	overpacked := readFixture(t, "triage-cycle283.md")
	overRR, overLogged := reviewWith(overpacked)
	if overRR.Approve {
		t.Error("the overpacked cycle-283 fixture must still be rejected at enforce — the new WARN must not suppress the clamp")
	}
	if strings.Contains(overLogged, "usable files= footprint") && !strings.Contains(overLogged, "overpacked") {
		t.Errorf("the capacity reject vanished from the logs: %q", overLogged)
	}
}

func TestDeclaredFilesNeverInflateFloorCount(t *testing.T) {
	pkgs := []string{"core", "bridge", "guards", "triagecap"}
	const companion = "/nonexistent/companion.json"
	for _, tc := range []struct{ name, bare, withFiles string }{
		{
			name: "floor-bearing item",
			bare: `## top_n (commit to THIS cycle)
- raise-core-coverage: raise core coverage to 90% — priority=H, source=scout
`,
			withFiles: `## top_n (commit to THIS cycle)
- raise-core-coverage: raise core coverage to 90% — priority=H, files=go/internal/core/orchestrator.go;go/internal/bridge/bridge.go, source=scout
`,
		},
		{
			// Not floor-bearing on its own; only the declared path contains the trigger word "floor".
			name: "footprint path containing a floor trigger word",
			bare: `## top_n (commit to THIS cycle)
- cut-flake-rate: cut the flake rate by 40% — priority=H, source=scout
`,
			withFiles: `## top_n (commit to THIS cycle)
- cut-flake-rate: cut the flake rate by 40% — priority=H, files=go/internal/triagecap/floors.go, source=scout
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := CountCommittedFloors(tc.bare, pkgs)
			if got := CountCommittedFloors(tc.withFiles, pkgs); got != want {
				t.Errorf("declaring files= changed the committed floor count: %d vs %d — a footprint is "+
					"what the work TOUCHES, never a coverage commitment", got, want)
			}
			if got, wantPkgs := CommittedFloorPackages(tc.withFiles, companion, pkgs), CommittedFloorPackages(tc.bare, companion, pkgs); len(got) != len(wantPkgs) {
				t.Errorf("declaring files= changed the counted floor packages: %v vs %v", got, wantPkgs)
			}
			deferredBare := strings.Replace(tc.bare, "## top_n (commit to THIS cycle)", "## deferred (carry over)", 1)
			deferredFiles := strings.Replace(tc.withFiles, "## top_n (commit to THIS cycle)", "## deferred (carry over)", 1)
			if got, wantPkgs := DeferredFloorPackages(deferredFiles, pkgs), DeferredFloorPackages(deferredBare, pkgs); len(got) != len(wantPkgs) {
				t.Errorf("declaring files= changed the DEFERRED floor packages: %v vs %v — Gate C would block "+
					"predicates on a package the card merely edits", got, wantPkgs)
			}
		})
	}
}
