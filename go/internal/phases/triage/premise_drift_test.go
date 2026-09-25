package triage

// premise_drift_test.go — F40 (cycle 1691): lane 1691 picked an item filed
// 2026-08-16 whose premise #535 had made unreachable on 2026-09-09; triage
// committed it, the builder "fixed" a non-bug and opened a fail-open, and the
// audit caught it a full cycle later. Six commits had touched the item's own
// declared files after it was filed, and #535 touched its package — evidence
// nothing put in front of triage. The host now gathers that drift (Core Rule
// 5: deterministic evidence in code, the judgment stays triage's) so a stale
// premise can be dropped at triage, where a drop ends the lane cleanly (F30).

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const staleItemFile = "2026-08-16T19-30-00Z-warn-gap.json"

// premiseRepo is a repo whose ship package moved after the item was filed on
// 2026-08-16: one commit before filing, one on the item's own file after it,
// one elsewhere in the same package after it (#535's shape).
func premiseRepo(t *testing.T, item string) string {
	t.Helper()
	dir := t.TempDir()
	boundsRunGit(t, dir, nil, "init", "-q", "-b", "main")
	boundsRunGit(t, dir, nil, "config", "user.email", "acs@example.invalid")
	boundsRunGit(t, dir, nil, "config", "user.name", "acs")
	commitAt(t, dir, "go/internal/ship/consume.go", "v1", "feat: the consumption gate", "2026-08-01T00:00:00", "")
	commitAt(t, dir, "go/internal/ship/consume.go", "v2", "fix: a touch before the item was filed", "2026-08-10T00:00:00", "")
	commitAt(t, dir, "go/internal/ship/consume.go", "v3", "fix(ship): single-source committed-inbox-id resolution", "2026-09-01T00:00:00", "")
	commitAt(t, dir, "go/internal/ship/audit.go", "a1", "fix: restore audit authority (ReadVerdict)", "2026-09-09T00:00:00", "")
	inbox := filepath.Join(dir, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, staleItemFile), []byte(item), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// commitAt commits body to file with the given message at committer date ts
// (author date authorTS, or ts when empty).
func commitAt(t *testing.T, dir, file, body, msg, ts, authorTS string) {
	t.Helper()
	p := filepath.Join(dir, file)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if authorTS == "" {
		authorTS = ts
	}
	boundsRunGit(t, dir, nil, "add", file)
	boundsRunGit(t, dir, []string{"GIT_AUTHOR_DATE=" + authorTS, "GIT_COMMITTER_DATE=" + ts}, "commit", "-q", "-m", msg)
}

const staleItem = `{"id":"warn-gap","title":"t","kind":"bug","created_at":"2026-08-16T19:30:00Z",
 "files":["go/internal/ship/consume.go (gate condition, both ship paths)","go/internal/ship/gone.go"]}`

func TestPremiseDriftSection_PutsTheDriftSinceFilingInFrontOfTriage(t *testing.T) {
	dir := premiseRepo(t, staleItem)
	section := premiseDriftSection(context.Background(), dir, "warn-gap")
	for _, want := range []string{
		"- premise_drift: evidence to re-verify, not a verdict",
		"warn-gap (filed 2026-08-16)",
		"1 commit(s) since filing touched its declared paths:",
		"fix(ship): single-source committed-inbox-id resolution", // on its own file, after filing
		"1 more commit(s) since filing touched only its packages (go/internal/ship/):",
		"fix: restore audit authority (ReadVerdict)", // #535's shape, by SUBJECT (review M4)
		"not at HEAD: go/internal/ship/gone.go",
	} {
		if !strings.Contains(section, want) {
			t.Errorf("section lacks %q:\n%s", want, section)
		}
	}
	for _, before := range []string{"a touch before the item was filed", "the consumption gate"} {
		if strings.Contains(section, before) {
			t.Errorf("a commit from before the filing is not drift (%q):\n%s", before, section)
		}
	}
}

// TestPremiseDriftSection_SeesTheItemWhereTheClaimLeftIt (review M2): triage
// claims before it selects, and a re-dispatched triage recomposes its prompt —
// the evidence must survive the move into processing/cycle-N/.
func TestPremiseDriftSection_SeesTheItemWhereTheClaimLeftIt(t *testing.T) {
	dir := premiseRepo(t, staleItem)
	claimed := filepath.Join(dir, ".evolve", "inbox", "processing", "cycle-7")
	if err := os.MkdirAll(claimed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(dir, ".evolve", "inbox", staleItemFile), filepath.Join(claimed, staleItemFile)); err != nil {
		t.Fatal(err)
	}
	if s := premiseDriftSection(context.Background(), dir, "warn-gap"); !strings.Contains(s, "warn-gap (filed 2026-08-16)") {
		t.Fatalf("a claimed item keeps its drift evidence:\n%s", s)
	}
}

// TestPremiseDriftSection_ACommitNamingTheIdIsTheStrongestSignal (review M4):
// a cycle ship that consumed the item lists its consumed file in the "## Actual
// diff" footer every ship commit carries (phases/ship/gitops.go) — found
// wherever the change landed, even outside the declared packages. A LONGER id
// that merely starts with this one is not a mention (re-review MINOR 2).
func TestPremiseDriftSection_ACommitNamingTheIdIsTheStrongestSignal(t *testing.T) {
	dir := premiseRepo(t, staleItem)
	footer := "evolve-cycle: goal=abc\n\n---\n## Actual diff (v8.34.0+)\n\nFiles modified (1):\n- A\t.evolve/inbox/consumed/2026-08-16T19-30-00Z-warn-gap.json"
	commitAt(t, dir, "go/internal/acssuite/evidence.go", "e1", footer, "2026-09-10T00:00:00", "")
	commitAt(t, dir, "go/internal/other/x.go", "o1", "fix: a sibling item warn-gap-v2 landed", "2026-09-11T00:00:00", "")
	s := premiseDriftSection(context.Background(), dir, "warn-gap")
	if !strings.Contains(s, "1 commit(s) since filing name its id:") || !strings.Contains(s, "evolve-cycle: goal=abc") {
		t.Fatalf("the consuming ship's footer names the id — listed first-class:\n%s", s)
	}
	if strings.Contains(s, "warn-gap-v2") {
		t.Errorf("a longer id is not a mention of this one:\n%s", s)
	}
}

// TestPremiseDriftSection_ShowsCommitterDatesAndSkipsTreeWidePackages (review
// M4, go-review MINOR): the date shown is the committer date --since filters
// on (a rebased commit's older author date would look like pre-filing
// evidence), and a file's one-segment parent ("go/") is never expanded into
// the whole tree's churn.
func TestPremiseDriftSection_ShowsCommitterDatesAndSkipsTreeWidePackages(t *testing.T) {
	item := `{"id":"warn-gap","kind":"bug","created_at":"2026-08-16T00:00:00Z","files":["go/top.go"]}`
	dir := premiseRepo(t, item)
	commitAt(t, dir, "go/top.go", "t1", "fix: rebased onto the filing", "2026-09-20T00:00:00", "2026-08-02T00:00:00")
	s := premiseDriftSection(context.Background(), dir, "warn-gap")
	if !strings.Contains(s, "2026-09-20 fix: rebased onto the filing") || strings.Contains(s, "2026-08-02") {
		t.Errorf("the committer date is the one shown:\n%s", s)
	}
	if strings.Contains(s, "touched only its packages") {
		t.Errorf("a one-segment parent is not expanded:\n%s", s)
	}
}

// TestPremiseDriftSection_TextIsPromptSafe: agent-authorable text (commit
// subjects, ids, declared paths) is control-stripped and length-capped, and a
// declared path outside the repository is reported, never queried or stat-ed
// — and it does not blank the rest of the item's evidence (go-review MAJOR).
func TestPremiseDriftSection_TextIsPromptSafe(t *testing.T) {
	item := `{"id":"warn-gap","kind":"bug","created_at":"2026-08-16T00:00:00Z","files":["go/internal/ship/consume.go","../../../../etc/hosts"]}`
	dir := premiseRepo(t, item)
	long := "fix: \x1b[31m" + strings.Repeat("x", 150)
	commitAt(t, dir, "go/internal/ship/consume.go", "v9", long, "2026-09-21T00:00:00", "")
	s := premiseDriftSection(context.Background(), dir, "warn-gap")
	if strings.Contains(s, "\x1b") {
		t.Errorf("a control character reached the prompt:\n%q", s)
	}
	if !strings.Contains(s, "…") || strings.Contains(s, strings.Repeat("x", 150)) {
		t.Errorf("a long subject is capped:\n%s", s)
	}
	if !strings.Contains(s, "declared outside the repository: ../../../../etc/hosts") {
		t.Errorf("an out-of-repo declared path is reported, never queried:\n%s", s)
	}
}

// TestPremiseDriftSection_SilentWithoutDriftVisibleWhenUnavailable: an item
// filed after the last change to its surface, with every declared path present,
// renders nothing; so do no scope, an unknown id and an item with no filing
// date. A git failure is VISIBLE (review m2) — silence would read as "no
// drift" — and still never blocks triage.
func TestPremiseDriftSection_SilentWithoutDriftVisibleWhenUnavailable(t *testing.T) {
	fresh := `{"id":"warn-gap","kind":"bug","created_at":"2026-09-20T00:00:00Z","files":["go/internal/ship/consume.go"]}`
	if s := premiseDriftSection(context.Background(), premiseRepo(t, fresh), "warn-gap"); s != "" {
		t.Errorf("no drift ⇒ no section:\n%s", s)
	}
	dir := premiseRepo(t, staleItem)
	for name, scope := range map[string]string{"no scope": "", "unknown id": "not-queued"} {
		if s := premiseDriftSection(context.Background(), dir, scope); s != "" {
			t.Errorf("%s ⇒ no section:\n%s", name, s)
		}
	}
	undated := premiseRepo(t, `{"id":"warn-gap","kind":"bug","files":["go/internal/ship/consume.go"]}`)
	if err := os.Rename(filepath.Join(undated, ".evolve", "inbox", staleItemFile), filepath.Join(undated, ".evolve", "inbox", "warn-gap.json")); err != nil {
		t.Fatal(err)
	}
	if s := premiseDriftSection(context.Background(), undated, "warn-gap"); s != "" {
		t.Errorf("an item with no filing date has no 'since' ⇒ no section:\n%s", s)
	}
	notRepo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(notRepo, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(notRepo, ".evolve", "inbox", staleItemFile), []byte(staleItem), 0o644); err != nil {
		t.Fatal(err)
	}
	if s := premiseDriftSection(context.Background(), notRepo, "warn-gap"); !strings.Contains(s, "warn-gap (filed 2026-08-16): drift unavailable") {
		t.Errorf("a git failure is visible, not silent:\n%s", s)
	}
}

// TestComposePrompt_RendersPremiseDriftOnlyForAFleetLane: the section rides
// the lane's own scope; a sequential cycle's prompt stays byte-identical.
func TestComposePrompt_RendersPremiseDriftOnlyForAFleetLane(t *testing.T) {
	dir := premiseRepo(t, staleItem)
	lane := core.PhaseRequest{ProjectRoot: dir, Context: map[string]string{"fleet_scope": "warn-gap"}}
	if got := (hooks{}).ComposePrompt("BODY", lane); !strings.Contains(got, "- premise_drift:") {
		t.Errorf("a fleet lane's triage prompt carries its scope's drift:\n%s", got)
	}
	sequential := core.PhaseRequest{ProjectRoot: dir, Context: map[string]string{}}
	if got := (hooks{}).ComposePrompt("BODY", sequential); strings.Contains(got, "premise_drift") {
		t.Errorf("a sequential cycle renders no premise_drift section:\n%s", got)
	}
}
