package deliverable

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualityindex"
)

type resolverF4 map[string]phasecontract.Contract

func (r resolverF4) Resolve(name string) (phasecontract.Contract, bool) {
	c, ok := r[name]
	return c, ok
}

func TestVerifyPrimary_UnreadableDispatchedArtifactIsInfraNotAViolation(t *testing.T) {
	dir := t.TempDir()
	c := phasecontract.Contract{Phase: "audit", ArtifactName: "audit-report.md", Kind: phasecontract.KindMarkdown}

	res, err := verifyPrimary("audit", c, phasecontract.Roots{Workspace: dir, DispatchedArtifact: dir}, config.StageOff)

	if err == nil || !strings.Contains(err.Error(), "deliverable: read "+dir) {
		t.Fatalf("err = %v", err)
	}
	if len(res.Violations) != 0 || res.OK {
		t.Errorf("a read fault is ambiguity, never an agent violation: %+v", res)
	}
}

func TestThresholdsFor_DefaultsWithoutAnEvolveDirAndPolicyErrorsWithOne(t *testing.T) {
	want, _ := qualityindex.ResolveThresholds(nil)

	got, err := thresholdsFor(phasecontract.Roots{})

	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%v err=%v, want the defaults %v", got, err, want)
	}

	root := t.TempDir()
	evolve := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolve, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := thresholdsFor(phasecontract.Roots{EvolveDir: evolve}); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("absent policy keeps the defaults: got=%v err=%v", got, err)
	}
	if err := os.WriteFile(filepath.Join(evolve, "policy.json"), []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := thresholdsFor(phasecontract.Roots{EvolveDir: evolve}); err == nil || !strings.Contains(err.Error(), "quality-index thresholds") {
		t.Fatalf("a broken policy must surface, got %v", err)
	}
}

func TestHostEffects_PerformIgnoresUnknownPhasesAndEffectsWithoutAHostHalf(t *testing.T) {
	var calls []claimCall
	h := &HostEffects{
		resolver: resolverF4{"odd": {Phase: "odd", Effects: []string{"no-such-effect", phasecontract.EffectInboxClaim}}},
		claim:    recordingClaimer(&calls),
	}

	if err := h.Perform(context.Background(), core.ReviewInput{Phase: "never-registered", Cycle: 3}); err != nil {
		t.Fatalf("an unregistered phase owes nothing: %v", err)
	}
	if len(calls) != 0 {
		t.Fatalf("no claim may run for an unregistered phase: %v", calls)
	}

	ws := t.TempDir()
	writeFile(t, ws, "triage-decision.json", `{"top_n":[{"id":"a"}]}`)
	if err := h.Perform(context.Background(), core.ReviewInput{Phase: "odd", Cycle: 3, Workspace: ws, ProjectRoot: t.TempDir()}); err != nil {
		t.Fatalf("an effect with no host half is skipped, not an error: %v", err)
	}
	if len(calls) != 1 || !reflect.DeepEqual(calls[0].ids, []string{"a"}) {
		t.Fatalf("the declared effects after the skipped one must still run: %v", calls)
	}
}

func TestSalvageVerdictWith_ReVerifiesUnderTheCallersContractAndRefusesWhenItCannotApprove(t *testing.T) {
	const malformed = "## Verdict\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\",} -->\n"
	newRes := func() Result {
		return Result{Phase: "audit", Content: malformed, Violations: []Violation{{Code: CodeBadVerdict, Message: "no parseable verdict"}}}
	}
	base := phasecontract.Contract{Phase: "audit", ArtifactName: "audit-report.md", Kind: phasecontract.KindMarkdown, Verdicts: []string{"PASS", "WARN", "FAIL"}}

	t.Run("a phase the resolver does not know is refused", func(t *testing.T) {
		got, ok := salvageVerdictWith(newRes(), resolverF4{}, phasecontract.Roots{}, config.StageOff)
		if ok || got.OK || got.Content != malformed {
			t.Fatalf("ok=%v result=%+v", ok, got)
		}
	})
	t.Run("a grammar that cannot be checked refuses the repair", func(t *testing.T) {
		root := t.TempDir()
		evolve := filepath.Join(root, ".evolve")
		if err := os.MkdirAll(evolve, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(evolve, "policy.json"), []byte("{broken"), 0o644); err != nil {
			t.Fatal(err)
		}
		c := base
		c.Grammars = []string{phasespec.GrammarCodeReviewReport}
		got, ok := salvageVerdictWith(newRes(), resolverF4{"audit": c}, phasecontract.Roots{EvolveDir: evolve}, config.StageOff)
		if ok || got.OK || got.Content != malformed {
			t.Fatalf("ok=%v result=%+v", ok, got)
		}
	})
	t.Run("a repaired report that still breaks the contract is refused", func(t *testing.T) {
		c := base
		c.Sections = []phasecontract.Section{{Canonical: "## Required", Accepted: []string{"## Required"}}}
		got, ok := salvageVerdictWith(newRes(), resolverF4{"audit": c}, phasecontract.Roots{}, config.StageOff)
		if ok || got.OK || got.Content != malformed {
			t.Fatalf("ok=%v result=%+v", ok, got)
		}
	})
	t.Run("a repaired report that satisfies the contract is approved with the repaired bytes", func(t *testing.T) {
		got, ok := salvageVerdictWith(newRes(), resolverF4{"audit": base}, phasecontract.Roots{}, config.StageOff)
		if !ok || !got.OK || got.Violations != nil || strings.Contains(got.Content, ",}") {
			t.Fatalf("ok=%v result=%+v", ok, got)
		}
	})
}
