package deliverable

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// triageSpecWithClaim is the registry's triage declaration in miniature: the report plus the claim effect.
func triageSpecWithClaim(effects ...string) phasespec.PhaseSpec {
	return phasespec.PhaseSpec{
		Name: "triage", Role: "triage",
		Outputs: phasespec.IO{Files: []string{".evolve/runs/cycle-{cycle}/triage-report.md"}},
		Effects: effects,
	}
}

const validTriageReport = "# Triage Decision\n\n## top_n\n- x: fix the thing — priority=H, files=a.go, source=scout\n"

// claimFixture lays out a workspace committed to item "x" and an inbox whose
// copy of "x" sits where the case says. cycle is the verifying cycle.
type claimFixture struct {
	decision string // triage-decision.json body; "" = none written
	itemIn   string // "" = no inbox file for x; "inbox" = root; "cycle-N" = processing/cycle-N/
}

func (f claimFixture) layout(t *testing.T) (ws, evolveDir string) {
	t.Helper()
	ws = t.TempDir()
	evolveDir = t.TempDir()
	writeFile(t, ws, "triage-report.md", validTriageReport)
	if f.decision != "" {
		writeFile(t, ws, "triage-decision.json", f.decision)
	}
	inbox := filepath.Join(evolveDir, "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if f.itemIn != "" {
		dir := inbox
		if cycle, isClaim := inboxbatch.ParseProcessingCycle(f.itemIn); isClaim {
			dir = inboxbatch.ProcessingCycleDir(inbox, cycle)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, dir, "2026-01-01T00-00-00Z-x.json", `{"id":"x","title":"fixture"}`)
	}
	return ws, evolveDir
}

const committedToX = `{"cycle":7,"top_n":[{"id":"x"}],"deferred":[]}`

func TestVerify_DeclaredEffectInboxClaim(t *testing.T) {
	const cycle = 7
	for _, tc := range []struct {
		name     string
		fx       claimFixture
		wantCode string // "" = OK
		mentions []string
	}{
		{"committed item still in the inbox root", claimFixture{committedToX, "inbox"}, CodeMissingEffect, []string{"inbox-claim", "x", strconv.Itoa(cycle)}},
		{"committed item claimed by this cycle", claimFixture{committedToX, "cycle-" + strconv.Itoa(cycle)}, "", nil},
		{"committed item held by another cycle", claimFixture{committedToX, "cycle-3"}, CodeMissingEffect, []string{"x", "cycle-3"}},
		{"committed id has no inbox file (scout-originated)", claimFixture{committedToX, ""}, "", nil},
		{"empty commitment owes no claim", claimFixture{`{"cycle":7,"top_n":[]}`, "inbox"}, "", nil},
		{"no decision recorded owes no claim", claimFixture{"", "inbox"}, "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws, evolveDir := tc.fx.layout(t)
			roots := phasecontract.Roots{Workspace: ws, EvolveDir: evolveDir, Cycle: cycle}
			res, err := VerifyWithStage("triage", roots, resolverFor(triageSpecWithClaim("inbox-claim")), config.StageOff)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantCode == "" {
				if !res.OK {
					t.Fatalf("want OK, got violations: %+v", res.Violations)
				}
				return
			}
			if res.OK || !hasCode(res, tc.wantCode) {
				t.Fatalf("triage declares the inbox-claim effect and the %s, but the gate returned OK=%v violations=%+v (want %s) — the spine would run on an unclaimed commitment",
					tc.name, res.OK, res.Violations, tc.wantCode)
			}
			for _, m := range tc.mentions {
				if !violationMentions(res, m) {
					t.Errorf("violation must mention %q: %+v", m, res.Violations)
				}
			}
		})
	}
}

func TestVerify_UnboundDeclaredEffect(t *testing.T) {
	ws, evolveDir := claimFixture{committedToX, "cycle-7"}.layout(t)
	roots := phasecontract.Roots{Workspace: ws, EvolveDir: evolveDir, Cycle: 7}
	res, err := VerifyWithStage("triage", roots, resolverFor(triageSpecWithClaim("teleport")), config.StageOff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.OK || !hasCode(res, CodeUnboundEffect) || !violationMentions(res, "teleport") {
		t.Fatalf("an unbound effect must be reported as %s naming it, got OK=%v violations=%+v", CodeUnboundEffect, res.OK, res.Violations)
	}
}

func TestVerify_DeclaredEffectNeedsCycleInRoots(t *testing.T) {
	ws, evolveDir := claimFixture{committedToX, "inbox"}.layout(t)
	roots := phasecontract.Roots{Workspace: ws, EvolveDir: evolveDir} // no Cycle
	if _, err := VerifyWithStage("triage", roots, resolverFor(triageSpecWithClaim("inbox-claim")), config.StageOff); err == nil {
		t.Fatal("verifying a declared effect without the cycle in Roots must error (fail open), not decide")
	}
}
