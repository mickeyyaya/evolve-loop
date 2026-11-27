package skillcheck

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
)

func profileNamed(t *testing.T, name string) HostProfile {
	t.Helper()
	for _, p := range HostProfiles() {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("no host profile %q", name)
	return HostProfile{}
}

func skillWith(name, description string) string {
	return "---\nname: " + name + "\ndescription: " + description + "\n---\n\nBody.\n"
}

func TestHostProfiles_AreClaudeCodeAndAgyOnly(t *testing.T) {
	var names []string
	for _, p := range HostProfiles() {
		names = append(names, p.Name)
	}
	if !slices.Equal(names, []string{"claude-code", "agy"}) {
		t.Fatalf("profiles = %v, want [claude-code agy] and no codex", names)
	}
}

func TestHostProfiles_CarryTheDocumentedLimits(t *testing.T) {
	cases := []struct {
		name, root, file string
		limit            int
	}{
		{"claude-code", "skills", "CLAUDE.md", 1536},
		{"agy", ".agents/skills", "GEMINI.md", 1024},
	}
	for _, c := range cases {
		p := profileNamed(t, c.name)
		if p.SkillRoot != c.root || p.DescriptionLimit != c.limit {
			t.Errorf("%s: root=%q limit=%d, want %q %d", c.name, p.SkillRoot, p.DescriptionLimit, c.root, c.limit)
		}
		want := []string{"name", "description", "license", "compatibility", "metadata", "allowed-tools", "argument-hint"}
		if !slices.Equal(p.Frontmatter, want) {
			t.Errorf("%s: frontmatter = %v, want %v", c.name, p.Frontmatter, want)
		}
		if len(p.Budgets) != 1 || p.Budgets[0].File != c.file || p.Budgets[0].Bytes != 16384 {
			t.Errorf("%s: budgets = %+v, want one budget of 16384 for %s", c.name, p.Budgets, c.file)
		}
	}
}

func TestHostFileBudget_ReadsTheProfileTable(t *testing.T) {
	cases := map[string]bool{"CLAUDE.md": true, "GEMINI.md": true, "AGENTS.md": false}
	for file, want := range cases {
		if got := hostFileBudget(file) > 0; got != want {
			t.Errorf("hostFileBudget(%q) > 0 = %v, want %v", file, got, want)
		}
	}
}

func TestInstructionRenders_TakeTheirBudgetFromTheHostProfiles(t *testing.T) {
	for _, r := range instructionRenders() {
		if r.byteBudget != hostFileBudget(r.target) {
			t.Errorf("%s: budget %d, want %d", r.target, r.byteBudget, hostFileBudget(r.target))
		}
	}
}

func TestSkillProblems_TheDescriptionLimit(t *testing.T) {
	claude := profileNamed(t, "claude-code")
	cases := []struct {
		name    string
		length  int
		wantErr bool
	}{
		{"at the limit", 1536, false},
		{"one over the limit", 1537, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			writeRel(t, root, "skills/big/SKILL.md", skillWith("big", strings.Repeat("a", c.length)))
			problems, err := SkillProblems(root, claude)
			if err != nil {
				t.Fatal(err)
			}
			if !c.wantErr {
				if len(problems) != 0 {
					t.Fatalf("problems = %v, want none", problems)
				}
				return
			}
			if len(problems) != 1 || !strings.Contains(problems[0], "skills/big/SKILL.md") || !strings.Contains(problems[0], "1537") || !strings.Contains(problems[0], "1536") || !strings.Contains(problems[0], "claude-code") {
				t.Fatalf("problems = %v, want one line naming the skill, 1537, 1536 and claude-code", problems)
			}
		})
	}
}

func TestSkillProblems_TheLimitCountsCharactersNotBytes(t *testing.T) {
	root := t.TempDir()
	writeRel(t, root, "skills/wide/SKILL.md", skillWith("wide", strings.Repeat("é", 1536)))
	problems, err := SkillProblems(root, profileNamed(t, "claude-code"))
	if err != nil || len(problems) != 0 {
		t.Fatalf("problems=%v err=%v, want none for 1536 characters", problems, err)
	}
}

func TestSkillProblems_AFrontmatterKeyOutsideTheAllowlist(t *testing.T) {
	root := t.TempDir()
	writeRel(t, root, "skills/odd/SKILL.md", "---\nname: odd\ndescription: ok\nargument-hint: x\nbogus-key: 1\n---\n\nBody.\n")
	problems, err := SkillProblems(root, profileNamed(t, "claude-code"))
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "bogus-key") || !strings.Contains(problems[0], "skills/odd/SKILL.md") {
		t.Fatalf("problems = %v, want one line naming bogus-key", problems)
	}
}

func TestSkillProblems_SkipsWhatIsNotASkill(t *testing.T) {
	root := t.TempDir()
	writeRel(t, root, "skills/README.md", "text")
	writeRel(t, root, "skills/empty/other.md", "text")
	writeRel(t, root, "skills/broken/SKILL.md", "---\nname: broken\ndescription: never closed\n")
	problems, err := SkillProblems(root, profileNamed(t, "claude-code"))
	if err != nil || len(problems) != 0 {
		t.Fatalf("problems=%v err=%v, want none", problems, err)
	}
}

func TestSkillProblems_AnAbsentSkillRootIsNoProblem(t *testing.T) {
	problems, err := SkillProblems(t.TempDir(), profileNamed(t, "agy"))
	if err != nil || len(problems) != 0 {
		t.Fatalf("problems=%v err=%v, want none", problems, err)
	}
}

func TestSkillProblems_AnUnreadableSkillRootIsAnError(t *testing.T) {
	root := t.TempDir()
	writeRel(t, root, "skills", "a file, not a directory")
	if _, err := SkillProblems(root, profileNamed(t, "claude-code")); err == nil || errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err = %v, want a read error that is not ErrNotExist", err)
	}
}

func TestHostSkillProblems_AgyUsesItsOwnLimit(t *testing.T) {
	root := t.TempDir()
	writeRel(t, root, ".agents/skills/wide/SKILL.md", skillWith("wide", strings.Repeat("a", 1025)))
	problems, err := hostSkillProblems(root)
	if err != nil || len(problems) != 1 || !strings.Contains(problems[0], "agy") {
		t.Fatalf("problems=%v err=%v, want one agy problem", problems, err)
	}
}

func TestHostSkillProblems_ReturnsTheReadError(t *testing.T) {
	root := t.TempDir()
	writeRel(t, root, "skills", "file")
	if _, err := hostSkillProblems(root); err == nil {
		t.Fatal("want the read error")
	}
}

func TestRunAndCheck_RefuseAnOverLongDescription(t *testing.T) {
	root := prepareSkillsTree(t)
	writeRel(t, root, "skills/loop/SKILL.md", skillWith("loop", strings.Repeat("a", 1537)))

	var out, errBuf bytes.Buffer
	for _, write := range []bool{false, true} {
		errBuf.Reset()
		if code := Run(root, write, &out, &errBuf); code != 2 || !strings.Contains(errBuf.String(), "DESCRIPTION: skills/loop/SKILL.md") {
			t.Fatalf("Run(write=%v) = %d, stderr=%q; want 2 and a DESCRIPTION line", write, code, errBuf.String())
		}
	}
	drift, err := Check(root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(drift, func(s string) bool { return strings.HasPrefix(s, "DESCRIPTION: skills/loop/SKILL.md") }) {
		t.Fatalf("Check = %v, want the DESCRIPTION problem", drift)
	}
}

func TestRunAndCheck_ReportAHostSkillRootReadFault(t *testing.T) {
	root := prepareSkillsTree(t)
	writeRel(t, root, ".agents/skills", "file")
	var out, errBuf bytes.Buffer
	if code := Run(root, false, &out, &errBuf); code != 1 {
		t.Fatalf("Run = %d, want 1", code)
	}
	if _, err := Check(root); err == nil {
		t.Fatal("Check: want the read error")
	}
}

func TestTheLiveTreeSkillsPassTheirHostProfiles(t *testing.T) {
	problems, err := hostSkillProblems(repoRoot(t))
	if err != nil || len(problems) != 0 {
		t.Fatalf("problems=%v err=%v, want none", problems, err)
	}
}

func TestParseHostProfiles(t *testing.T) {
	var syntaxErr *json.SyntaxError
	if _, err := parseHostProfiles([]byte("{]")); !errors.As(err, &syntaxErr) {
		t.Fatalf("err = %v, want a JSON syntax error", err)
	}
	profiles, err := parseHostProfiles(hostProfilesJSON)
	if err != nil || len(profiles) != 2 {
		t.Fatalf("profiles=%v err=%v, want the two embedded profiles", profiles, err)
	}
}

func TestFileBudget_DecodesFromTheProfileData(t *testing.T) {
	profiles, err := parseHostProfiles([]byte(`{"profiles":[{"name":"x","budgets":[{"file":"A.md","bytes":7}]}]}`))
	if err != nil || len(profiles) != 1 {
		t.Fatalf("profiles=%v err=%v", profiles, err)
	}
	if want := (FileBudget{File: "A.md", Bytes: 7}); profiles[0].Budgets[0] != want {
		t.Fatalf("budget = %+v, want %+v", profiles[0].Budgets[0], want)
	}
}

func TestParseHostProfiles_ARenamedFieldIsAnError(t *testing.T) {
	for _, raw := range []string{
		`{"profiles":[{"name":"x","budgets":[{"file":"A.md","byte":7}]}]}`,
		`{"profile":[]}`,
	} {
		if _, err := parseHostProfiles([]byte(raw)); err == nil {
			t.Errorf("parseHostProfiles(%s) = nil error, want an unknown-field error", raw)
		}
	}
}

func TestParseHostProfiles_TheAllowlistExtendsTheSpecList(t *testing.T) {
	profiles, err := parseHostProfiles([]byte(`{"spec_frontmatter":["a","b"],"profiles":[{"name":"x","host_frontmatter":["h"]},{"name":"y"}]}`))
	if err != nil || len(profiles) != 2 {
		t.Fatalf("profiles=%v err=%v", profiles, err)
	}
	if !slices.Equal(profiles[0].Frontmatter, []string{"a", "b", "h"}) || !slices.Equal(profiles[1].Frontmatter, []string{"a", "b"}) {
		t.Fatalf("allowlists = %v and %v", profiles[0].Frontmatter, profiles[1].Frontmatter)
	}
}

func TestMustParseHostProfiles(t *testing.T) {
	if got := mustParseHostProfiles(hostProfilesJSON); len(got) != 2 {
		t.Fatalf("profiles = %v, want two", got)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("want a panic on malformed data")
		}
	}()
	mustParseHostProfiles([]byte("{"))
}

func TestHostProfiles_ReturnsACopy(t *testing.T) {
	first := HostProfiles()
	first[0].Name = "changed"
	if HostProfiles()[0].Name != "claude-code" {
		t.Fatal("a caller changed the shared table")
	}
}

func TestBudgetProblems_PinTheBudgetOfEachHost(t *testing.T) {
	for _, file := range []string{"CLAUDE.md", "GEMINI.md"} {
		for _, c := range []struct {
			size int
			want int
		}{{16384, 0}, {16385, 1}} {
			diffs := []commandDiff{{rel: file, next: strings.Repeat("a", c.size)}}
			problems := budgetProblems(diffs, instructionRenders())
			if len(problems) != c.want {
				t.Errorf("%s at %d bytes: problems = %v, want %d", file, c.size, problems, c.want)
			}
			if c.want == 1 && !strings.HasPrefix(problems[0], "BUDGET: "+file) {
				t.Errorf("%s: problem = %q", file, problems[0])
			}
		}
	}
}

func TestSkillProblems_TheFrontmatterProblemsComeInSortedOrder(t *testing.T) {
	root := t.TempDir()
	writeRel(t, root, "skills/odd/SKILL.md", "---\nname: odd\ndescription: ok\nzeta-key: 1\nalpha-key: 2\n---\n\nBody.\n")
	problems, err := SkillProblems(root, profileNamed(t, "claude-code"))
	if err != nil || len(problems) != 2 || !strings.Contains(problems[0], "alpha-key") || !strings.Contains(problems[1], "zeta-key") {
		t.Fatalf("problems=%v err=%v, want alpha-key then zeta-key", problems, err)
	}
}
