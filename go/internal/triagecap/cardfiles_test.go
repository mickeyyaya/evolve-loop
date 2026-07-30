package triagecap

// cardfiles_test.go — RED contract for triage-cards-carry-files (inbox weight
// 0.89, campaign convergence-2026-07).
//
// Live evidence (batch-14, cycles 1133/1134): cycle-1130's triage-decision.json
// top_n card {id: surface-verdict-conflict-in-audit-classify, action: "capture
// pre-override agent verdict in Classify (go/internal/phases/audit/audit.go)…"}
// carried NO files[]. That {id, action} shape is exactly what
// ProjectDecisionJSON emits — the orchestrator's projection IS the de-facto
// writer, since the agent "in practice almost never" authors the companion
// (project.go header). With no files[], fleet.TodosFromTriage falls back to the
// id island, so the card looked disjoint from a backfilled card whose files[]
// named the SAME audit.go: two concurrent lanes editing one file, the 948
// lost-work class the disjointness planner exists to prevent.
//
// The fix is at the WRITER and it is STRUCTURED, never inferred: the report item
// carries a `files=` metadata field (the agent already names the paths in prose)
// and the projection parses it. Guessing paths out of the action prose is
// explicitly out of bounds — a wrong inferred file is worse than an island.
//
// RED today: projTopN has no Files field and MissingCardFilesWarning does not
// exist — this file does not compile.

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

// cardFilesReport is a production-shaped report whose FIRST top_n item declares
// its footprint and whose SECOND names a path in prose only — the exact defect
// shape observed on cycle-1130.
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

// TestProjectDecisionJSON_TopNCardsCarryDeclaredFiles is the writer contract: a
// declared files= footprint must reach the companion's top_n card as files[],
// which is the ONLY channel the fleet disjointness planner can read (menus are
// exact-file-overlap only, PR #366).
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
	// The action prose must survive intact: files= is metadata, and the em-dash
	// tail is already excluded from the action by actionOf.
	if !strings.Contains(got.TopN[0].Action, "Reconcile the auditor verdict") {
		t.Errorf("card action = %q, want the prose preserved", got.TopN[0].Action)
	}
	if strings.Contains(got.TopN[0].Action, "files=") {
		t.Errorf("card action = %q leaked the files= metadata into the prose", got.TopN[0].Action)
	}
}

// TestProjectDecisionJSON_CardWithoutFilesInfersNothing is the NEGATIVE twin and
// the item's explicit boundary: a card that declares no files= must project NO
// files[] — the paths named in its prose must NOT be harvested. A wrong inferred
// file merges two genuinely disjoint lanes (or splits one), and the planner then
// trusts a fiction. Absent is honest; guessed is not.
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
	// omitempty: the key must be absent, not an empty array, so a consumer can
	// distinguish "declared nothing" from "declared an empty footprint".
	if strings.Contains(string(body), `"files": []`) {
		t.Errorf("projected an empty files array:\n%s", body)
	}
}

// TestProjectDecisionJSON_DeclaredFilesRejectMalformedTokens pins the shape
// filter: only repo-relative paths are projected. An absolute path or a `..`
// escape would hand the planner a footprint it cannot match against any other
// card's repo-relative files (and, for `..`, one that names something outside
// the repo at all).
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

// TestMissingCardFilesWarning_NamesFilelessCardsThatCitePaths is the loud channel
// the item asks for: a committed card whose action names a repo path but which
// declares no files= must be WARNed about, by id. Silence here is what let the
// defect live for a whole batch.
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

// TestMissingCardFilesWarning_SilentWhenNothingToSay is the NEGATIVE twin: full
// compliance, a card with no path in its prose (documentation/research work
// legitimately has no footprint), and an artifact with no top_n section at all
// must every one of them be silent. A gate that always warns is not a signal.
func TestMissingCardFilesWarning_SilentWhenNothingToSay(t *testing.T) {
	if msg := MissingCardFilesWarning(compliantCardFilesReport(), ""); msg != "" {
		t.Errorf("all cards declare files=, yet warning = %q", msg)
	}

	// A footprint-free card that carries the CONTRACT-REQUIRED evidence= pointer.
	// evidence pointers are routinely file paths (the floor counters read them on
	// purpose), so a prose scan that does not drop the evidence VALUE warns on
	// nearly every legitimately file-less card — research, doc reads — and a
	// warning that fires on compliant work is one an agent learns to ignore.
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

// TestMissingCardFilesWarning_UnusableDeclarationIsNotSilent covers the offence
// that is WORSE than an omission: a card that declares a footprint no consumer can
// match (an unsubstituted template placeholder, a glob, an absolute path). It looks
// compliant, so nothing downstream complains, yet it overlaps nothing — two lanes
// on one file with no diagnostic anywhere.
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

// TestMissingCardFilesWarning_AgentCompanionIsTheAuthority pins the source of
// truth: ship/postship PREFERS an agent-authored triage-decision.json over the
// projection, so the companion is what the lane planner reads. A companion whose
// cards declare files[] must silence the report-based check (no false alarm), and a
// companion with the live cycle-1130 shape ({id, action} only) must be caught even
// when the report is unreadable.
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

	// An absent companion must fall back to the report, never go silent.
	if msg := MissingCardFilesWarning(cardFilesReport, filepath.Join(dir, "absent.json")); msg == "" {
		t.Error("an absent companion silenced the report-based check")
	}
}

// compliantCardFilesReport is cardFilesReport with BOTH cards declaring their
// footprint — the shape the contract now requires.
func compliantCardFilesReport() string {
	return strings.Replace(cardFilesReport,
		"- surface-verdict-conflict-in-audit-classify: capture pre-override agent verdict in Classify (go/internal/phases/audit/audit.go) — priority=H, source=scout",
		"- surface-verdict-conflict-in-audit-classify: capture pre-override agent verdict — priority=H, files=go/internal/phases/audit/audit.go, source=scout", 1)
}

// writeCardCompanion writes a raw triage-decision.json body. (The sibling
// writeCompanion in declarative_floors_test.go writes a committed_floors list —
// different shape, hence a second helper rather than an overload.)
func writeCardCompanion(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestSplitDeclaredFiles_ToleratesTheSpellingsAgentsWrite pins the parse against
// the shapes real agent markdown carries. A value regex that stops at the first
// space loses paths (a partial footprint is still an overlap on the dropped file)
// AND leaves them in the item text, where the floor scanners read them as package
// mentions. Every separator splits; the field ends at the next `, key=`.
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

// TestCapReviewer_WarnsOnFilelessCards is the PRODUCTION-CALLER proof:
// MissingCardFilesWarning is reached through triagecap.CapReviewer.Review — the
// deliverable-review seam cmd_cycle.go chains for every triage phase (the clamp
// is enforce by compiled default), so the warning lands on the real pipeline path
// and not only in a helper. It must NOT block: provenance is a WARN, and trading a
// silent overlap risk for a hard cycle failure is not an improvement.
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

	// Compliant report: silence on this channel...
	compliantRR, compliantLogged := reviewWith(compliantCardFilesReport())
	if strings.Contains(compliantLogged, "usable files= footprint") {
		t.Errorf("compliant report still WARNed: %q", compliantLogged)
	}
	// ...and the VERDICT must be identical either way. This is the real invariant:
	// footprint provenance is a WARN, so declaring (or omitting) files= must not
	// change what the capacity clamp decides — neither by blocking a file-less card
	// nor by letting a declared path inflate the floor count into a rejection.
	if rr.Approve != compliantRR.Approve || rr.Reason != compliantRR.Reason {
		t.Errorf("the clamp verdict changed with the footprint declaration:\n file-less: approve=%v reason=%q\n declared: approve=%v reason=%q",
			rr.Approve, rr.Reason, compliantRR.Approve, compliantRR.Reason)
	}

	// An overpacked report must still be REJECTED for capacity, with the WARN
	// riding alongside — the provenance check must not swallow or precede the
	// clamp's own verdict.
	overpacked := readFixture(t, "triage-cycle283.md")
	overRR, overLogged := reviewWith(overpacked)
	if overRR.Approve {
		t.Error("the overpacked cycle-283 fixture must still be rejected at enforce — the new WARN must not suppress the clamp")
	}
	if strings.Contains(overLogged, "usable files= footprint") && !strings.Contains(overLogged, "overpacked") {
		t.Errorf("the capacity reject vanished from the logs: %q", overLogged)
	}
}

// TestDeclaredFilesNeverInflateFloorCount is the regression the new metadata
// field could silently introduce: the floor counters scan item text for known
// package names, so a files= list naming packages must be stripped like every
// other metadata field (source=, priority=, evidence=). Otherwise declaring a
// footprint would raise the cycle's committed-floor count and trip the capacity
// clamp — a new gate failure caused purely by better provenance.
func TestDeclaredFilesNeverInflateFloorCount(t *testing.T) {
	pkgs := []string{"core", "bridge", "guards", "triagecap"}
	const companion = "/nonexistent/companion.json"
	for _, tc := range []struct{ name, bare, withFiles string }{
		{
			// A genuinely floor-bearing item: the footprint must not add packages.
			name: "floor-bearing item",
			bare: `## top_n (commit to THIS cycle)
- raise-core-coverage: raise core coverage to 90% — priority=H, source=scout
`,
			withFiles: `## top_n (commit to THIS cycle)
- raise-core-coverage: raise core coverage to 90% — priority=H, files=go/internal/core/orchestrator.go;go/internal/bridge/bridge.go, source=scout
`,
		},
		{
			// THE trap: the item is NOT floor-bearing (no coverage/floor word), but
			// the declared path contains "floors". floorWordRE runs on the raw item,
			// so an unstripped footprint flips this card into a floor-bearing one —
			// a phantom committed floor, straight into the capacity clamp, caused
			// purely by declaring better provenance.
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
