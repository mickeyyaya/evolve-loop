//go:build acs

// Package cycle2245 holds the ACS predicates of practice M5, the notes of each CLI family.
package cycle2245

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/practicewatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/skillcheck"
	"github.com/mickeyyaya/evolve-loop/go/internal/skilloverlay"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	claudeNotes = "model-notes-claude"
	agyNotes    = "model-notes-agy"
)

func familyNotesPolicy() policy.Policy {
	return policy.Policy{Overlays: &policy.OverlaysPolicy{Rules: []policy.OverlayRule{
		{Families: []string{"claude"}, Skills: []string{claudeNotes}},
		{Families: []string{"agy"}, Skills: []string{agyNotes}},
	}}}
}

const familyNotesPolicyJSON = `{"overlays":{"rules":[
{"families":["claude"],"skills":["model-notes-claude"]},
{"families":["agy"],"skills":["model-notes-agy"]}
]}}`

func TestC2245_001_ADispatchGetsTheNotesOfItsOwnFamilyOnly(t *testing.T) {
	pol := familyNotesPolicy()
	cases := []struct {
		cli  string
		want []string
	}{
		{"claude", []string{claudeNotes}},
		{"claude-tmux", []string{claudeNotes}},
		{"claude-p", []string{claudeNotes}},
		{"agy", []string{agyNotes}},
		{"agy-tmux", []string{agyNotes}},
	}
	for _, tc := range cases {
		for _, phase := range []string{"scout", "tdd", "build", "audit"} {
			got := pol.ResolveOverlays(policy.OverlayDispatch{Phase: phase, CLI: tc.cli, Model: "m", Tier: "balanced"})
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("cli=%q phase=%s resolved %v, want exactly %v", tc.cli, phase, got, tc.want)
			}
		}
	}
}

func TestC2245_002_TheLaunchPathReadsTheFamilyRuleFromPolicyJSON(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(familyNotesPolicyJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := map[string][]string{
		"claude-tmux": {claudeNotes},
		"agy-tmux":    {agyNotes},
		"codex-tmux":  nil,
	}
	for cli, want := range cases {
		got := policy.ResolveLaunchOverlaysFailOpen(root, "build", cli, "m")
		if len(got) == 0 && len(want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ResolveLaunchOverlaysFailOpen(cli=%q) = %v, want %v", cli, got, want)
		}
	}
}

func TestC2245_003_EachFamilyHasANotesFileThatTheMaterializerLoadsAlone(t *testing.T) {
	root := acsassert.RepoRoot(t)
	skillsDir := filepath.Join(root, "skills")
	pol := familyNotesPolicy()

	claudePrefix, claudeMissing := skilloverlay.Materialize(skillsDir,
		pol.ResolveOverlays(policy.OverlayDispatch{Phase: "build", CLI: "claude-tmux", Tier: "balanced"}))
	if len(claudeMissing) != 0 || !strings.Contains(claudePrefix, "PRELOADED SKILL: "+claudeNotes) {
		t.Errorf("claude dispatch: missing=%v prefix=%q, want the claude notes loaded", claudeMissing, claudePrefix)
	}
	if strings.Contains(claudePrefix, agyNotes) {
		t.Errorf("a claude dispatch prefix carries the agy notes:\n%s", claudePrefix)
	}

	agyPrefix, agyMissing := skilloverlay.Materialize(skillsDir,
		pol.ResolveOverlays(policy.OverlayDispatch{Phase: "build", CLI: "agy-tmux", Tier: "balanced"}))
	if len(agyMissing) != 0 || !strings.Contains(agyPrefix, "PRELOADED SKILL: "+agyNotes) {
		t.Errorf("agy dispatch: missing=%v prefix=%q, want the agy notes loaded", agyMissing, agyPrefix)
	}
	if strings.Contains(agyPrefix, claudeNotes) {
		t.Errorf("an agy dispatch prefix carries the claude notes:\n%s", agyPrefix)
	}
	if claudePrefix == agyPrefix {
		t.Errorf("the two families load the same notes text")
	}
}

func TestC2245_004_AFamilyWithoutNotesGetsNoOverlayAndNoError(t *testing.T) {
	root := acsassert.RepoRoot(t)
	skillsDir := filepath.Join(root, "skills")
	pol := familyNotesPolicy()
	for _, cli := range []string{"codex", "codex-tmux", "ollama-tmux", "gemini", "", "claudex", "my-claude", "Claude"} {
		got := pol.ResolveOverlays(policy.OverlayDispatch{Phase: "build", CLI: cli, Tier: "balanced"})
		if len(got) != 0 {
			t.Errorf("cli=%q resolved %v, want no overlay", cli, got)
		}
		prefix, missing := skilloverlay.Materialize(skillsDir, got)
		if prefix != "" || len(missing) != 0 {
			t.Errorf("cli=%q materialized prefix=%q missing=%v, want neither", cli, prefix, missing)
		}
	}
}

func TestC2245_005_NoFamilyNotesRideTheDefaultPreload(t *testing.T) {
	for _, s := range policy.CompiledDefaultOverlaySkills() {
		if strings.HasPrefix(s, "model-notes-") {
			t.Errorf("the compiled default preloads %q: an unprotected skill would reach every prompt (audit H1 of cycle 2116)", s)
		}
	}
	var none policy.Policy
	code := map[string]string{"deliverable_kind": "code"}
	for _, cli := range []string{"claude-tmux", "agy-tmux", "codex-tmux"} {
		for _, phase := range []string{"scout", "tdd", "build", "audit", "code-review"} {
			for _, got := range none.ResolveOverlays(policy.OverlayDispatch{Phase: phase, CLI: cli, Tier: "balanced", WritesSource: true, Signals: code}) {
				if strings.HasPrefix(got, "model-notes-") {
					t.Errorf("an absent overlays block resolved %q for cli=%s phase=%s", got, cli, phase)
				}
			}
		}
	}
}

func TestC2245_006_TheNotesFilesAreTrackedAndNoManifestProblemNamesThem(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, name := range []string{claudeNotes, agyNotes} {
		dir := filepath.Join(root, "skills", name)
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) == 0 {
			t.Fatalf("RED: skills/%s has no notes file: %v", name, err)
		}
		_, _, ignored, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", filepath.Join("skills", name, entries[0].Name()))
		if ignored == 0 {
			t.Errorf("skills/%s/%s is gitignored and would be dropped at ship", name, entries[0].Name())
		}
	}
	problems, err := skillcheck.ManifestProblems(root)
	if err != nil {
		t.Fatalf("ManifestProblems: %v", err)
	}
	for _, p := range problems {
		if strings.Contains(p, "model-notes") {
			t.Errorf("manifest problem: %s", p)
		}
	}
}

func TestC2245_007_TheSkillsCheckStaysCleanWithTheNotesFiles(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if _, err := os.Stat(filepath.Join(root, "skills", claudeNotes)); err != nil {
		t.Fatalf("RED: the notes of the claude family are absent: %v", err)
	}
	var stdout, stderr strings.Builder
	if code := skillcheck.Run(root, false, &stdout, &stderr); code != 0 {
		t.Errorf("skills check exit=%d\n%s", code, acsassert.OutputTail(stderr.String(), 12))
	}
}

func practiceWatchItems(t *testing.T, family string) string {
	t.Helper()
	body := "## 1.0\n- first\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	root := t.TempDir()
	opts := practicewatch.Options{
		Root:    root,
		Sources: []practicewatch.Source{{ID: "src-" + family, Family: family, Kind: practicewatch.KindGithubFile, Locator: srv.URL, Enabled: true}},
		Timeout: 3 * time.Second,
		Now:     func() time.Time { return time.Date(2026, 11, 9, 9, 0, 0, 0, time.UTC) },
		Inbox:   inboxmover.Options{ProjectRoot: root, Stderr: io.Discard, PriorityClasses: []string{"correctness", "hygiene"}},
	}
	practicewatch.Watch(context.Background(), opts)
	body = "## 1.1\n- shorter prompts\n## 1.0\n- first\n"
	report := practicewatch.Watch(context.Background(), opts)
	if len(report.Results) != 1 || report.Results[0].Status != practicewatch.StatusFiled {
		t.Fatalf("family %s: report %+v, want one filed item", family, report.Results)
	}
	scan, err := inboxbatch.ScanDir(filepath.Join(root, ".evolve", "inbox"))
	if err != nil || len(scan.Items) != 1 {
		t.Fatalf("family %s: inbox scan err=%v items=%d, want 1", family, err, len(scan.Items))
	}
	raw, err := os.ReadFile(filepath.Join(root, ".evolve", "inbox", scan.Items[0].Path))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Summary
}

func TestC2245_008_ThePracticeWatchItemNamesTheNotesFileOfItsFamilyOnly(t *testing.T) {
	claude := practiceWatchItems(t, "claude")
	if !strings.Contains(claude, "skills/"+claudeNotes+"/") {
		t.Errorf("the claude item does not name its notes file:\n%s", claude)
	}
	if strings.Contains(claude, agyNotes) {
		t.Errorf("the claude item names the agy notes:\n%s", claude)
	}
	agy := practiceWatchItems(t, "agy")
	if !strings.Contains(agy, "skills/"+agyNotes+"/") {
		t.Errorf("the agy item does not name its notes file:\n%s", agy)
	}
	if strings.Contains(agy, claudeNotes) {
		t.Errorf("the agy item names the claude notes:\n%s", agy)
	}
}

func TestC2245_009_AFamilyWithoutNotesGetsAWatchItemThatNamesNoNotesFile(t *testing.T) {
	codex := practiceWatchItems(t, "codex")
	if strings.Contains(codex, "model-notes-") {
		t.Errorf("the codex item names a notes file that does not exist:\n%s", codex)
	}
}
