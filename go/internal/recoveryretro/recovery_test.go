package recoveryretro

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
)

const utf8Item = "inboxbatch-utf8-and-resolution"

func tasks(ids ...string) *[]string { return &ids }

func failDossier(cycle int, fingerprint, class, reason string, phases ...dossier.PhaseRecord) *dossier.Dossier {
	return &dossier.Dossier{
		Cycle: cycle, FinalVerdict: dossier.VerdictFail, Tasks: tasks(utf8Item), Phases: phases,
		Failure: &dossier.FailureRecord{Fingerprint: fingerprint, PreClass: class, Reasons: []string{reason}},
	}
}

func phase(name, model string) dossier.PhaseRecord {
	return dossier.PhaseRecord{Name: name, Verdict: dossier.VerdictPass, ResolvedModel: model}
}

func utf8History() ([]*dossier.Dossier, *dossier.Dossier) {
	pass := &dossier.Dossier{
		Cycle: 1809, FinalVerdict: dossier.VerdictPass, Tasks: tasks(utf8Item),
		CommitSHA: "31ed49bcf4d028bba15655bef5009221fbad0cf8",
		Phases:    []dossier.PhaseRecord{phase("scout", "flash"), phase("build", "opus"), phase("audit", "opus")},
	}
	history := []*dossier.Dossier{
		failDossier(1804, "build|gate-block|9af1ff95a11a", "gate-block",
			"review gate: phase \"build\" deliverable rejected after 2 correction(s): build handoff floor: 1 deterministic check failure(s) — fix these exactly before handoff: Explanation Documentation: Changed Ar", phase("scout", "flash"), phase("build", "flash")),
		failDossier(1806, "worktree|unknown|38ddaf05594b", "unknown",
			"worktree provisioning failed: worktree base ref (cycle 1806): git fetch origin: rc=1 err=<nil>: error: cannot lock ref `refs/remotes/origin/train/wave64-boundary`: is at e1d9ae9e57443c9e5ec12486b322cf"),
		failDossier(1808, "build|gate-block|0b91bd88eb78", "gate-block",
			"review gate: phase \"build\" deliverable rejected after 2 correction(s): build handoff floor: 1 deterministic check failure(s) — fix these exactly before handoff: ./acs/cycle1808 (-tags acs): unit tes", phase("scout", "flash"), phase("build", "flash")),
		pass,
	}
	return history, pass
}

func TestRecoveries_ThreeFailuresThenAPassIsOneRecoveryWithEachTry(t *testing.T) {
	history, pass := utf8History()

	got := Recoveries(history, pass)

	if len(got) != 1 {
		t.Fatalf("Recoveries = %d, want 1", len(got))
	}
	r := got[0]
	if r.TaskID != utf8Item || r.Pass.Cycle != 1809 || r.Pass.CommitSHA != pass.CommitSHA {
		t.Fatalf("recovery = %+v, want item %s shipped in 1809", r, utf8Item)
	}
	var cycles []int
	for _, f := range r.Failures {
		cycles = append(cycles, f.Cycle)
	}
	if want := []int{1804, 1806, 1808}; !equalInts(cycles, want) {
		t.Fatalf("failure cycles = %v, want %v", cycles, want)
	}
	if r.Failures[1].Class != "unknown" || r.Failures[1].Fingerprint != "worktree|unknown|38ddaf05594b" {
		t.Fatalf("failure 1806 = %+v, want the class and the fingerprint of the dossier", r.Failures[1])
	}
}

func TestRecoveries_OneFailureIsNoRecovery(t *testing.T) {
	history, pass := utf8History()

	if got := Recoveries(history[2:], pass); len(got) != 0 {
		t.Fatalf("Recoveries with one failure = %+v, want none", got)
	}
}

func TestRecoveries_AnEarlierPassResetsTheFailureCount(t *testing.T) {
	history, pass := utf8History()
	earlierPass := &dossier.Dossier{Cycle: 1805, FinalVerdict: dossier.VerdictPass, Tasks: tasks(utf8Item)}
	withReset := append([]*dossier.Dossier{earlierPass}, history...)

	if got := Recoveries(withReset, pass); len(got) != 1 || len(got[0].Failures) != 2 {
		t.Fatalf("Recoveries after an earlier pass = %+v, want one recovery with the 2 failures after cycle 1805", got)
	}
}

func TestRecoveries_IgnoresOtherItemsLaterCyclesAndAFailedPass(t *testing.T) {
	history, pass := utf8History()
	other := &dossier.Dossier{Cycle: 1807, FinalVerdict: dossier.VerdictFail, Tasks: tasks("another-item")}
	later := failDossier(1810, "build|x|1", "gate-block", "later")
	noTasks := &dossier.Dossier{Cycle: 1803, FinalVerdict: dossier.VerdictFail}
	mixed := append([]*dossier.Dossier{other, later, noTasks, nil}, history...)

	got := Recoveries(mixed, pass)
	if len(got) != 1 || len(got[0].Failures) != 3 {
		t.Fatalf("Recoveries = %+v, want 3 failures of the item before cycle 1809", got)
	}
	if got := Recoveries(history, history[0]); got != nil {
		t.Fatalf("Recoveries for a FAIL dossier = %+v, want nil", got)
	}
	if got := Recoveries(history, nil); got != nil {
		t.Fatalf("Recoveries for no dossier = %+v, want nil", got)
	}
	untasked := &dossier.Dossier{Cycle: 1809, FinalVerdict: dossier.VerdictPass}
	if got := Recoveries(history, untasked); got != nil {
		t.Fatalf("Recoveries for a pass with no tasks = %+v, want nil", got)
	}
}

func TestRecoveries_AFailDossierWithNoFailureRecordStillCounts(t *testing.T) {
	history, pass := utf8History()
	bare := &dossier.Dossier{Cycle: 1807, FinalVerdict: dossier.VerdictFail, Tasks: tasks(utf8Item)}
	got := Recoveries(append([]*dossier.Dossier{bare}, history...), pass)

	if len(got) != 1 || len(got[0].Failures) != 4 || got[0].Failures[2].Cycle != 1807 || got[0].Failures[2].Class != "" {
		t.Fatalf("Recoveries = %+v, want the bare FAIL of 1807 in cycle order with an empty class", got)
	}
}

func TestRenderRecovery_ShowsEachTryTheMarkAndTheDifference(t *testing.T) {
	history, pass := utf8History()
	r := Recoveries(history, pass)[0]
	r.Rungs = []string{"build R1"}

	doc := RenderRecovery(r)

	for _, want := range []string{
		"# Recovery of `inboxbatch-utf8-and-resolution` in cycle 1809",
		"| 1804 | FAIL | `gate-block` | `build\\|gate-block\\|9af1ff95a11a` |",
		"| 1806 | FAIL | `unknown` | `worktree\\|unknown\\|38ddaf05594b` |",
		"| infrastructure? |",
		"fix these exactly before handoff: ./acs/cycle1808 (-tags acs): unit tes` |",
		"| 1809 | PASS | — | — | commit `31ed49bc` | |",
		"Phases in the passed try only: `audit`.",
		"`build`: `flash` to `opus`.",
		"The passed try used these ladder rungs: `build R1`.",
		"The 3 failed tries have 3 fingerprints.",
		"## Judgment",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("RenderRecovery lacks %q:\n%s", want, doc)
		}
	}
	if strings.Count(doc, "| infrastructure? |") != 1 {
		t.Errorf("only cycle 1806 is infrastructure, got:\n%s", doc)
	}
	if strings.Contains(doc, "cannot lock ref `refs") {
		t.Errorf("a backtick of the agent text breaks the code span:\n%s", doc)
	}
}

func TestRenderRecovery_SaysWhenItFindsNoDifference(t *testing.T) {
	same := []dossier.PhaseRecord{phase("build", "opus")}
	r := Recovery{
		TaskID:   "item",
		Failures: []Try{{Cycle: 1, Verdict: dossier.VerdictFail, Fingerprint: "f", Models: modelsOf(same)}, {Cycle: 2, Verdict: dossier.VerdictFail, Fingerprint: "f", Models: modelsOf(same)}},
		Pass:     Try{Cycle: 3, Verdict: dossier.VerdictPass, Models: modelsOf(same)},
	}

	doc := RenderRecovery(r)

	for _, want := range []string{
		"The two tries ran the same phases.",
		"No phase changed its model.",
		"The passed try used no ladder rung above `R0`.",
		"The 2 failed tries have 1 fingerprint.",
		"The host found no difference in phases, models or rungs.",
		"| 3 | PASS | — | — | — | |",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("RenderRecovery lacks %q:\n%s", want, doc)
		}
	}
}

func TestRenderRecovery_NamesThePhasesOnlyTheLastFailureRan(t *testing.T) {
	r := Recovery{
		TaskID:   "item",
		Failures: []Try{{Cycle: 1}, {Cycle: 2, Models: modelsOf([]dossier.PhaseRecord{phase("build", "a"), phase("tdd", "a")})}},
		Pass:     Try{Cycle: 3, Models: modelsOf([]dossier.PhaseRecord{phase("build", "")})},
	}

	doc := RenderRecovery(r)

	if !strings.Contains(doc, "Phases in the last failed try only: `tdd`.") || !strings.Contains(doc, "No phase changed its model.") {
		t.Fatalf("RenderRecovery =\n%s", doc)
	}
}

func TestRenderRecovery_PassesTheStrictSTELint(t *testing.T) {
	history, pass := utf8History()
	r := Recoveries(history, pass)[0]

	assertSTEClean(t, RenderRecovery(r))
	r.Rungs = nil
	r.Failures[len(r.Failures)-1].Models = r.Pass.Models
	assertSTEClean(t, RenderRecovery(r))
}

func TestRecoveryName_IsTheCycleAndASafeItemSlug(t *testing.T) {
	r := Recovery{TaskID: "Item With/Space_x", Pass: Try{Cycle: 7}}

	if got, want := RecoveryName(r), "cycle-7-recovery-item-with-space-x"; got != want {
		t.Fatalf("RecoveryName = %q, want %q", got, want)
	}
}

func assertSTEClean(t *testing.T, doc string) {
	t.Helper()
	words, err := stelint.LoadStandard(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("LoadStandard: %v", err)
	}
	if findings := stelint.Check([]byte(doc), stelint.Options{Words: words}); len(findings) != 0 {
		t.Fatalf("STE findings %+v in:\n%s", findings, doc)
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestRenderRecovery_AShortOrMissingCommitStaysWhole(t *testing.T) {
	r := Recovery{TaskID: "item", Failures: []Try{{Cycle: 1}, {Cycle: 2}}, Pass: Try{Cycle: 3, CommitSHA: "abc"}}

	if doc := RenderRecovery(r); !strings.Contains(doc, "**Shipped:** cycle 3, commit `abc`.") {
		t.Fatalf("RenderRecovery =\n%s", doc)
	}
	r.Pass.CommitSHA = ""
	if doc := RenderRecovery(r); !strings.Contains(doc, "**Shipped:** cycle 3, with no commit in the dossier.") || !strings.Contains(doc, "with no recorded class") {
		t.Fatalf("RenderRecovery =\n%s", doc)
	}
}

func TestRecoveries_OnlyTheFailuresAfterTheLastPassCount(t *testing.T) {
	d := func(cycle int, verdict string) *dossier.Dossier {
		return &dossier.Dossier{Cycle: cycle, FinalVerdict: verdict, Tasks: tasks(utf8Item)}
	}
	pass := d(5, dossier.VerdictPass)
	history := []*dossier.Dossier{d(1, dossier.VerdictFail), d(2, dossier.VerdictFail), d(3, dossier.VerdictPass), d(4, dossier.VerdictFail), pass}

	if got := Recoveries(history, pass); len(got) != 0 {
		t.Fatalf("Recoveries = %+v, want none: only cycle 4 failed after the pass of cycle 3", got)
	}
}

func TestRecoveryName_AnEmptyOrLongItemStillGivesASafeName(t *testing.T) {
	if got := RecoveryName(Recovery{TaskID: "///", Pass: Try{Cycle: 7}}); got != "cycle-7-recovery-item" {
		t.Fatalf("RecoveryName(empty slug) = %q", got)
	}
	long := strings.Repeat("ab-", 40)
	got := RecoveryName(Recovery{TaskID: long, Pass: Try{Cycle: 7}})
	if slugPart := strings.TrimPrefix(got, "cycle-7-recovery-"); len(slugPart) > maxSlugLen || strings.HasSuffix(slugPart, "-") || !strings.HasPrefix(long, slugPart) {
		t.Fatalf("RecoveryName(long) = %q, want a prefix of the item of at most %d bytes with no dash at the end", got, maxSlugLen)
	}
}

func TestRenderJudgmentLesson_AnEmptyActionIsADash(t *testing.T) {
	doc := RenderJudgmentLesson(JudgmentLesson{Cycle: 3, Phase: "plan-review"})

	if !strings.Contains(doc, "## Lesson\n\n—\n") || strings.Contains(doc, "``") {
		t.Fatalf("RenderJudgmentLesson with no action =\n%s", doc)
	}
}
