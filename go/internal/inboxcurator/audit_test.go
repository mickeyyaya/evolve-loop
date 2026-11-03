package inboxcurator

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

var auditNow = time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)

func filed(id, created string, acceptance ...string) inboxbatch.Item {
	it := item(id, "the title of "+id, []string{"go/internal/" + id + ".go"}, acceptance...)
	it.CreatedAt = created
	return it
}

func TestStaleProbeless_KeepsTheOldItemsThatNameNoTestOldestFirst(t *testing.T) {
	items := []inboxbatch.Item{
		filed("young", "2026-11-01T00:00:00Z", "the gate is quiet"),
		filed("old-b", "2026-09-02T00:00:00Z", "the gate is quiet"),
		filed("old-tested", "2026-09-01T00:00:00Z", "`TestGate_IsQuiet` passes"),
		filed("old-a", "2026-08-01T00:00:00Z"),
		filed("undated", ""),
	}

	got := StaleProbeless(items, auditNow.Add(-14*24*time.Hour))

	var ids []string
	for _, it := range got {
		ids = append(ids, it.ID)
	}
	if want := []string{"undated", "old-a", "old-b"}; !slices.Equal(ids, want) {
		t.Errorf("StaleProbeless = %v, want %v: an item that names a test has a probe, and an undated item is the oldest", ids, want)
	}
	if !Probeless(items[0]) || Probeless(items[2]) {
		t.Error("Probeless must be true for an item that names no test and false for an item that names one")
	}
}

func auditRepo(t *testing.T) (*gittest.Repo, string) {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write("README.md", "x\n")
	repo.Git("add", "README.md")
	repo.Git("commit", "-q", "-m", "base")
	repo.Write("go/internal/landed.go", "package internal\n")
	repo.Git("add", "go")
	repo.Git("commit", "-q", "-m", "feat: the landed change")
	return repo, repo.Git("rev-parse", "HEAD")
}

func TestCandidateSource_CandidateListsTheCommitsThatTouchTheDeclaredFilesSinceTheFilingAndTheGonePaths(t *testing.T) {
	repo, head := auditRepo(t)
	it := filed("landed", "2020-01-01T00:00:00Z", "the change lands")
	it.Files = []string{"go/internal/landed.go", "go/internal/gone.go"}
	it.Path = "/x/.evolve/inbox/landed.json"
	exists := func(p string) bool { return p == "go/internal/landed.go" }

	got, err := CandidateSource{Git: gitexec.Default(repo.Dir), Limit: 5, Exists: exists}.Candidate(context.Background(), it)

	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "landed" || got.Path != "landed.json" || got.Title != it.Title || got.FiledAt != "2020-01-01T00:00:00Z" {
		t.Errorf("candidate = %+v, want the id, the file name, the title and the filing time of the item", got)
	}
	if !slices.Equal(got.Landed, []string{head}) || !slices.Equal(got.Gone, []string{"go/internal/gone.go"}) {
		t.Errorf("candidate = %+v, want the landed head commit and the gone path", got)
	}
	if !slices.Equal(got.Files, it.Files) || !slices.Equal(got.Acceptance, it.Acceptance) {
		t.Errorf("candidate = %+v, want the files and the acceptance of the item", got)
	}

	young := filed("landed", "2099-01-01T00:00:00Z")
	young.Files = []string{"go/internal/landed.go"}
	later, err := CandidateSource{Git: gitexec.Default(repo.Dir), Limit: 5, Exists: exists}.Candidate(context.Background(), young)
	if err != nil || len(later.Landed) != 0 {
		t.Errorf("a commit before the filing is not a landing of the item: %+v, %v", later, err)
	}
}

func TestCandidateSource_CandidateAPathOutsideTheRepositoryIsGoneAndNeverReachesGitOrTheFileSystem(t *testing.T) {
	repo, head := auditRepo(t)
	it := filed("escape", "2020-01-01T00:00:00Z")
	it.Files = []string{"../../etc/passwd", "go/internal/landed.go"}
	var asked []string
	exists := func(p string) bool {
		asked = append(asked, p)
		return true
	}

	got, err := CandidateSource{Git: gitexec.Default(repo.Dir), Limit: 5, Exists: exists}.Candidate(context.Background(), it)

	if err != nil || !slices.Equal(got.Gone, []string{"../../etc/passwd"}) || !slices.Equal(got.Landed, []string{head}) {
		t.Errorf("candidate = %+v, %v; want the escaping path gone and the landed commit of the local path", got, err)
	}
	if !slices.Equal(asked, []string{"go/internal/landed.go"}) {
		t.Errorf("exists was asked %v; a path outside the repository must never be checked", asked)
	}
}

func TestCandidateSource_CandidateAnItemWithNoDeclaredPathHasNoLandedCommit(t *testing.T) {
	repo, _ := auditRepo(t)
	it := filed("bare", "")
	it.Files = nil

	got, err := CandidateSource{Git: gitexec.Default(repo.Dir), Limit: 5, Exists: func(string) bool { return false }}.Candidate(context.Background(), it)

	if err != nil || got.ID != "bare" || got.Landed != nil || got.Gone != nil || got.FiledAt != "" {
		t.Errorf("candidate = %+v, %v; want no landed commit, no gone path and no filing time", got, err)
	}
}

func TestCandidateSource_CandidateAGitFailureIsAnError(t *testing.T) {
	it := filed("landed", "")

	_, err := CandidateSource{Git: gitexec.Default(t.TempDir()), Limit: 5, Exists: func(string) bool { return true }}.Candidate(context.Background(), it)

	if err == nil || !strings.Contains(err.Error(), "landed") {
		t.Errorf("err = %v, want an error that names the item", err)
	}
}

func TestIsLandedCommit_IsTrueOnlyForACommitOfTheCheckedOutBranch(t *testing.T) {
	repo, head := auditRepo(t)
	git := gitexec.Default(repo.Dir)

	if !IsLandedCommit(context.Background(), git, head) {
		t.Error("the head commit must be landed")
	}
	if IsLandedCommit(context.Background(), git, "0123456789abcdef0123456789abcdef01234567") {
		t.Error("an unknown commit must not be landed")
	}
	if IsLandedCommit(context.Background(), git, "--all") {
		t.Error("a string that is not a commit id must not be landed")
	}
}

var auditCandidates = []AuditCandidate{
	{ID: "landed", Path: "landed.json", Landed: []string{"abc1234"}},
	{ID: "gone", Path: "gone.json", Gone: []string{"go/x.go"}},
	{ID: "open", Path: "open.json"},
}

func landedIn(shas ...string) func(string) bool {
	return func(sha string) bool { return slices.Contains(shas, sha) }
}

func TestParseAudit_AConsumeThatCitesALandedCommitIsAJudgmentProposal(t *testing.T) {
	raw := `{"schema":"evolve.inbox.audit/1","proposals":[
		{"id":"landed","verdict":"consume","reason":"the change landed","evidence":["abc1234"]},
		{"id":"gone","verdict":"retire","reason":"the file is gone","evidence":["go/x.go"]},
		{"id":"open","verdict":"keep","reason":"nothing landed"}]}`

	got, err := ParseAudit([]byte(raw), auditCandidates, landedIn("abc1234"))

	if err != nil || len(got.Refused) != 0 || len(got.Proposals) != 3 {
		t.Fatalf("ParseAudit = %+v, %v; want three proposals", got, err)
	}
	want := Proposal{Kind: ProposeConsume, ID: "landed", Path: "landed.json", Reason: "the change landed", Evidence: []string{"abc1234"}, Trigger: TriggerAudit}
	if p := got.Proposals[0]; p.Kind != want.Kind || p.ID != want.ID || p.Path != want.Path || p.Reason != want.Reason ||
		!slices.Equal(p.Evidence, want.Evidence) || p.Trigger != want.Trigger || p.Deterministic {
		t.Errorf("consume = %+v, want %+v: an LLM proposal is never deterministic", p, want)
	}
	if got.Proposals[1].Kind != ProposeRetire || got.Proposals[2].Kind != ProposeKeep {
		t.Errorf("proposals = %+v, want a retire and a keep", got.Proposals)
	}
}

func TestParseAudit_RefusesAnUnknownIDARepeatAnUnknownVerdictAnEmptyReasonAndAConsumeWithNoLandedCommit(t *testing.T) {
	raw := `{"schema":"evolve.inbox.audit/1","proposals":[
		{"id":"elsewhere","verdict":"consume","reason":"r","evidence":["abc1234"]},
		{"id":"landed","verdict":"consume","reason":"r","evidence":["fffffff","go/x.go"]},
		{"id":"landed","verdict":"keep","reason":"r"},
		{"id":"gone","verdict":"delete","reason":"r"},
		{"id":"open","verdict":"retire","reason":"  "},
		{"id":"open","verdict":"keep","reason":"r"}]}`

	got, err := ParseAudit([]byte(raw), auditCandidates, landedIn("abc1234"))

	if err != nil {
		t.Fatal(err)
	}
	if len(got.Proposals) != 2 || got.Proposals[0].ID != "landed" || got.Proposals[0].Kind != ProposeKeep || got.Proposals[1].ID != "open" {
		t.Errorf("proposals = %+v, want the keep of landed and the keep of open", got.Proposals)
	}
	wants := []string{`elsewhere: the id is not a candidate`, `landed: a consume cites no commit of the checked-out branch`, `gone: the verdict is not consume, retire or keep (verdict "delete")`, `open: the reason is empty`}
	if len(got.Refused) != len(wants) {
		t.Fatalf("refused = %q, want %d refusals", got.Refused, len(wants))
	}
	for i, w := range wants {
		if !strings.Contains(got.Refused[i], w) {
			t.Errorf("refusal %d = %q, want it to contain %q", i, got.Refused[i], w)
		}
	}

	again, err := ParseAudit([]byte(`{"schema":"evolve.inbox.audit/1","proposals":[{"id":"open","verdict":"keep","reason":"r"},{"id":"open","verdict":"keep","reason":"r"}]}`), auditCandidates, landedIn())
	if err != nil || len(again.Proposals) != 1 || len(again.Refused) != 1 || !strings.Contains(again.Refused[0], "open: the id has a second verdict") {
		t.Errorf("a second verdict for one id must be refused: %+v, %v", again, err)
	}
}

func TestParseAudit_ADocumentThatIsNotTheAuditSchemaIsAnError(t *testing.T) {
	for name, raw := range map[string]string{
		"not JSON":       `{`,
		"another schema": `{"schema":"evolve.diagnosis/1","proposals":[]}`,
		"unknown field":  `{"schema":"evolve.inbox.audit/1","proposals":[],"apply":true}`,
		"trailing value": `{"schema":"evolve.inbox.audit/1","proposals":[]} {}`,
	} {
		if _, err := ParseAudit([]byte(raw), auditCandidates, landedIn()); err == nil {
			t.Errorf("%s: ParseAudit = nil error, want an error", name)
		}
	}
}

func TestAuditContract_RendersEachVerdictAndEachRefusalRuleThatParseAuditChecks(t *testing.T) {
	contract := AuditContract()

	for _, v := range AuditVerdicts() {
		if !strings.Contains(contract, "`"+string(v.Kind)+"`") || !strings.Contains(contract, v.UseWhen) || !strings.Contains(contract, v.Evidence) {
			t.Errorf("the contract must render the verdict %+v:\n%s", v, contract)
		}
	}
	rules := AuditRules()
	if len(rules) != 5 {
		t.Fatalf("AuditRules() = %d rules, want the 5 rules that ParseAudit checks", len(rules))
	}
	for _, r := range rules {
		if !strings.Contains(contract, "- "+r) {
			t.Errorf("the contract must list the refusal rule %q:\n%s", r, contract)
		}
	}
	for _, f := range AuditCandidateFields() {
		if !strings.Contains(contract, "`"+f.Name+"`") || !strings.Contains(contract, f.Meaning) {
			t.Errorf("the contract must render the input field %+v", f)
		}
	}
	if !strings.Contains(contract, AuditSchema) {
		t.Errorf("the contract must name the schema %s", AuditSchema)
	}
}

func TestAuditCandidateFields_NameEachJSONFieldOfAnAuditCandidate(t *testing.T) {
	var tags []string
	ty := reflect.TypeOf(AuditCandidate{})
	for i := range ty.NumField() {
		tags = append(tags, ty.Field(i).Tag.Get("json"))
	}
	var names []string
	for _, f := range AuditCandidateFields() {
		names = append(names, f.Name)
	}
	if !slices.Equal(names, tags) {
		t.Errorf("AuditCandidateFields() = %v, want the JSON fields %v in order", names, tags)
	}
}
