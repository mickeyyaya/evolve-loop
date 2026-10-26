package agentcmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func writeSpec(t *testing.T, root, name, agent string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, ".evolve", "phases", "adversarial-review", "phase.json"))
	if err != nil {
		t.Fatal(err)
	}
	var spec map[string]any
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	spec["name"], spec["agent"] = name, agent
	delete(spec, "writes_source")
	out, _ := json.Marshal(spec)
	dir := filepath.Join(root, ".evolve", "phases", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "phase.json"), out, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAgentRun_AnUndeclaredWriterOnAWriteGrantingProfileIsRefused(t *testing.T) {
	root := project(t)
	writeSpec(t, root, "sneaky-writer", "evolve-test-amplification")
	before := snapshot(t, root)
	fb := &fakeBridge{artifact: passReport}
	var stdout, stderr bytes.Buffer

	code := New(depsWith(fb))([]string{"run", "sneaky-writer", "--project-root", root}, strings.NewReader(diff), &stdout, &stderr)

	if code != exitRefused || !strings.Contains(stderr.String(), "writes source") || len(fb.prompts) != 0 {
		t.Errorf("exit %d stderr %q prompts %d, want 3, a writes-source refusal and no dispatch", code, stderr.String(), len(fb.prompts))
	}
	if changed := changedPaths(before, snapshot(t, root)); len(changed) != 0 {
		t.Errorf("a refused run changed %v", changed)
	}
	var list bytes.Buffer
	New(depsWith(nil))([]string{"list", "--project-root", root}, nil, &list, &stderr)
	if strings.Contains(list.String(), "sneaky-writer") {
		t.Errorf("list names the undeclared writer:\n%s", list.String())
	}
}

func TestAgentRun_AProfileWithoutAConfinedSandboxIsRefused(t *testing.T) {
	root := project(t)
	writeSpec(t, root, "no-profile-review", "evolve-no-such-profile")
	var stdout, stderr bytes.Buffer

	code := New(depsWith(&fakeBridge{artifact: passReport}))([]string{"run", "no-profile-review", "--project-root", root}, strings.NewReader(diff), &stdout, &stderr)

	if code != exitRefused || !strings.Contains(stderr.String(), "writes source") {
		t.Errorf("exit %d stderr %q, want a fail-closed refusal for a missing profile", code, stderr.String())
	}
}

func TestConfinedToWorkspace(t *testing.T) {
	cases := []struct {
		name string
		s    *profiles.SandboxConfig
		want bool
	}{
		{"no sandbox", nil, false},
		{"disabled", &profiles.SandboxConfig{ReadOnlyRepo: true}, false},
		{"writable repo", &profiles.SandboxConfig{Enabled: true, WriteSubpaths: []string{".evolve/runs/cycle-*"}}, false},
		{"worktree grant", &profiles.SandboxConfig{Enabled: true, ReadOnlyRepo: true, WriteSubpaths: []string{"{worktree_path}", ".evolve/runs/cycle-*"}}, false},
		{"repo path grant", &profiles.SandboxConfig{Enabled: true, ReadOnlyRepo: true, WriteSubpaths: []string{"docs"}}, false},
		{"run dir only", &profiles.SandboxConfig{Enabled: true, ReadOnlyRepo: true, WriteSubpaths: []string{".evolve/runs/cycle-*"}}, true},
		{"run dir subpath", &profiles.SandboxConfig{Enabled: true, ReadOnlyRepo: true, WriteSubpaths: []string{".evolve/runs/cycle-*/notes"}}, true},
		{"dot-dot escape", &profiles.SandboxConfig{Enabled: true, ReadOnlyRepo: true, WriteSubpaths: []string{".evolve/runs/cycle-*/../../go"}}, false},
		{"absolute grant", &profiles.SandboxConfig{Enabled: true, ReadOnlyRepo: true, WriteSubpaths: []string{"/tmp/x"}}, false},
		{"dot-slash run dir", &profiles.SandboxConfig{Enabled: true, ReadOnlyRepo: true, WriteSubpaths: []string{"./.evolve/runs/cycle-*"}}, true},
		{"no grants", &profiles.SandboxConfig{Enabled: true, ReadOnlyRepo: true}, true},
	}
	for _, c := range cases {
		if got := confinedToWorkspace(c.s); got != c.want {
			t.Errorf("%s: confinedToWorkspace = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestAgentRun_TheInputCapCoversAllSourcesTogether(t *testing.T) {
	root := project(t)
	attached := filepath.Join(t.TempDir(), "half.txt")
	if err := os.WriteFile(attached, bytes.Repeat([]byte("b"), maxInputBytes/2+1), 0o644); err != nil {
		t.Fatal(err)
	}
	fb := &fakeBridge{artifact: passReport}
	var stdout, stderr bytes.Buffer

	code := New(depsWith(fb))(runArgs(root, "--file", attached), strings.NewReader(strings.Repeat("a", maxInputBytes/2+1)), &stdout, &stderr)

	if code != 11 || !strings.Contains(stderr.String(), "together") || len(fb.prompts) != 0 {
		t.Errorf("exit %d stderr %q prompts %d, want 11, the total limit and no dispatch", code, stderr.String(), len(fb.prompts))
	}
}

func TestAgentList_NamesEachConfinedProfileInNameOrder(t *testing.T) {
	root := project(t)
	writeSpec(t, root, "zz-second-review", "evolve-adversarial-review")
	writeSpec(t, root, "sneaky-writer", "evolve-test-amplification")
	var stdout, stderr bytes.Buffer

	code := New(depsWith(nil))([]string{"list", "--json", "--project-root", root}, nil, &stdout, &stderr)

	var got []Profile
	if code != 0 || json.Unmarshal(stdout.Bytes(), &got) != nil {
		t.Fatalf("exit %d stdout %q", code, stdout.String())
	}
	var names []string
	for _, p := range got {
		names = append(names, p.Name)
	}
	if strings.Join(names, ",") != "adversarial-review,zz-second-review" {
		t.Errorf("list = %v, want the two confined profiles in name order", names)
	}
}
