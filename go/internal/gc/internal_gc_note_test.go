package gc

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const (
	gcNoteRelPath = "../../../docs/architecture/packages/internal-gc.md"
	gcPlanLink    = "(../../plans/gc-closes-the-legacy-gap-2026-11.md)"
)

var gcAmendedDecisionIDs = []string{"D2", "D4", "D6", "D7", "D9"}

const gcAmendedNoteFixture = `# internal/gc

> Amended by: [gc closes the legacy gap](../../plans/gc-closes-the-legacy-gap-2026-11.md) (D2, D4, D6, D7, D9).

## Invariants

- **The hook touches a dev worktree or a non-cycle branch only at gc.wave_end.scope: full.** It acts under D2 and D7.
- **A branch is deleted only after D2.** The branch is ended (D6) or landed (D4).
- **Nothing live is ever touched.** A live run dir is excluded.
`

func namesDecision(text, id string) bool {
	return regexp.MustCompile(`\b` + id + `\b`).MatchString(text)
}

func noteBullets(section string) []string {
	var bullets []string
	for _, line := range strings.Split(section, "\n") {
		if strings.HasPrefix(line, "- **") || len(bullets) == 0 {
			bullets = append(bullets, line)
			continue
		}
		bullets[len(bullets)-1] += "\n" + line
	}
	return bullets
}

func invariantsSection(note string) (string, bool) {
	_, after, found := strings.Cut(note, "\n## Invariants\n")
	if !found {
		return "", false
	}
	section, _, _ := strings.Cut(after, "\n## ")
	return section, true
}

func amendedByProblems(note string) []string {
	var line string
	for _, candidate := range strings.Split(note, "\n") {
		if strings.Contains(candidate, "Amended by") {
			line = candidate
			break
		}
	}
	if line == "" {
		return []string{"the note has no Amended by line"}
	}
	var problems []string
	if !strings.Contains(line, gcPlanLink) {
		problems = append(problems, "the Amended by line does not link gc-closes-the-legacy-gap-2026-11.md")
	}
	for _, id := range gcAmendedDecisionIDs {
		if !namesDecision(line, id) {
			problems = append(problems, "the Amended by line does not name "+id)
		}
	}
	return problems
}

func hookInvariantProblems(bullets []string) []string {
	for _, bullet := range bullets {
		title, _, _ := strings.Cut(strings.TrimPrefix(bullet, "- **"), "**")
		if !strings.Contains(strings.ToLower(title), "hook") {
			continue
		}
		if strings.Contains(bullet, "gc.wave_end.scope") && strings.Contains(bullet, "full") &&
			namesDecision(bullet, "D2") && namesDecision(bullet, "D7") {
			return nil
		}
	}
	return []string{"no hook invariant names gc.wave_end.scope at full under D2 and D7"}
}

func branchInvariantProblems(bullets []string) []string {
	for _, bullet := range bullets {
		if strings.Contains(bullet, "deleted only after") &&
			namesDecision(bullet, "D2") && namesDecision(bullet, "D4") && namesDecision(bullet, "D6") {
			return nil
		}
	}
	return []string{"no invariant says a branch is deleted only after D2, when it is ended (D6) or landed (D4)"}
}

func amendedNoteProblems(note string) []string {
	if strings.TrimSpace(note) == "" {
		return []string{"the note is empty"}
	}
	problems := amendedByProblems(note)
	section, found := invariantsSection(note)
	if !found {
		return append(problems, "the note has no Invariants section")
	}
	lowered := strings.ToLower(note)
	for _, stale := range []string{"an unmerged branch is never deleted", "hook in enforce never touches a dev worktree"} {
		if strings.Contains(lowered, stale) {
			problems = append(problems, "the note still says: "+stale)
		}
	}
	bullets := noteBullets(section)
	problems = append(problems, hookInvariantProblems(bullets)...)
	return append(problems, branchInvariantProblems(bullets)...)
}

func mutateNote(t *testing.T, old, replacement string) string {
	t.Helper()
	if !strings.Contains(gcAmendedNoteFixture, old) {
		t.Fatalf("fixture does not contain %q", old)
	}
	return strings.Replace(gcAmendedNoteFixture, old, replacement, 1)
}

func TestInternalGCNote_AmendedByLinksThePlan(t *testing.T) {
	raw, err := os.ReadFile(filepath.FromSlash(gcNoteRelPath))
	if err != nil {
		t.Fatalf("read the gc package note: %v", err)
	}
	if problems := amendedNoteProblems(string(raw)); len(problems) > 0 {
		t.Fatalf("internal-gc.md is not amended:\n- %s", strings.Join(problems, "\n- "))
	}
}

func TestInternalGCNote_ANoteWithEveryAmendmentPasses(t *testing.T) {
	if problems := amendedNoteProblems(gcAmendedNoteFixture); len(problems) > 0 {
		t.Fatalf("the fixture must pass, got:\n- %s", strings.Join(problems, "\n- "))
	}
}

func TestInternalGCNote_EachMissingAmendmentFailsTheCheck(t *testing.T) {
	amendedLine := "> Amended by: [gc closes the legacy gap](../../plans/gc-closes-the-legacy-gap-2026-11.md) (D2, D4, D6, D7, D9).\n"
	cases := []struct {
		name string
		note string
		want string
	}{
		{"no Amended by line", mutateNote(t, amendedLine, ""), "no Amended by line"},
		{"Amended by links another plan", mutateNote(t, "gc-closes-the-legacy-gap-2026-11.md", "workspace-hygiene-2026-07.md"), "does not link"},
		{"Amended by omits D7", mutateNote(t, "D6, D7, D9", "D6, D9"), "does not name D7"},
		{"Amended by names D20 instead of D2", mutateNote(t, "(D2, D4", "(D20, D4"), "does not name D2"},
		{"old branch wording returns", mutateNote(t, "- **Nothing live", "- **An unmerged branch is never deleted.** Old.\n- **Nothing live"), "still says: an unmerged branch is never deleted"},
		{"old hook wording returns", mutateNote(t, "- **Nothing live", "- **The batch hook in enforce never touches a dev worktree.** Old.\n- **Nothing live"), "still says: hook in enforce"},
		{"hook invariant omits the scope", mutateNote(t, "gc.wave_end.scope: full", "the wave scope"), "no hook invariant"},
		{"hook invariant omits D7", mutateNote(t, "under D2 and D7", "under D2"), "no hook invariant"},
		{"branch invariant omits D4", mutateNote(t, "landed (D4)", "landed"), "no invariant says a branch"},
		{"no Invariants section", mutateNote(t, "## Invariants", "## Rules"), "no Invariants section"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			problems := amendedNoteProblems(tc.note)
			if len(problems) == 0 {
				t.Fatalf("the check passed a note with the defect %q", tc.name)
			}
			if !strings.Contains(strings.Join(problems, "\n"), tc.want) {
				t.Fatalf("problems %v do not mention %q", problems, tc.want)
			}
		})
	}
}

func TestInternalGCNote_AnEmptyNoteFailsWithAClearMessage(t *testing.T) {
	for _, note := range []string{"", " \n\t\n"} {
		problems := amendedNoteProblems(note)
		if len(problems) != 1 || problems[0] != "the note is empty" {
			t.Fatalf("empty note %q gave %v, want the single message \"the note is empty\"", note, problems)
		}
	}
}
