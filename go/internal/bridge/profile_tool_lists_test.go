package bridge

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func writeToolListProfile(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "agent.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	return path
}

func TestLoadProfile_ParsesDisallowedTools(t *testing.T) {
	t.Parallel()
	path := writeToolListProfile(t, t.TempDir(), `{"name":"agent","allowed_tools":["Read"],"disallowed_tools":["Bash(curl:*)","WebFetch"]}`)

	p, err := LoadProfile(path)

	if err != nil {
		t.Fatalf("LoadProfile: %v", err)
	}
	if want := []string{"Bash(curl:*)", "WebFetch"}; !reflect.DeepEqual(p.DisallowedTools, want) {
		t.Fatalf("DisallowedTools = %v, want %v", p.DisallowedTools, want)
	}
	if want := []string{"Read"}; !reflect.DeepEqual(p.AllowedTools, want) {
		t.Fatalf("AllowedTools = %v, want %v", p.AllowedTools, want)
	}
}

func TestLoadProfile_ExpandsToolPolicySentinelsInBothLists(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	policyJSON := `{"policies":{"net":["Bash(curl:*)","Bash(wget:*)"],"read":["Read","Grep"]}}`
	if err := os.WriteFile(filepath.Join(dir, "tool-policy.json"), []byte(policyJSON), 0o644); err != nil {
		t.Fatalf("write policy: %v", err)
	}
	path := writeToolListProfile(t, dir, `{"name":"agent","allowed_tools":["$include_policy:read"],"disallowed_tools":["WebFetch","$include_policy:net"]}`)

	p, err := LoadProfile(path)

	if err != nil {
		t.Fatalf("LoadProfile: %v", err)
	}
	if want := []string{"WebFetch", "Bash(curl:*)", "Bash(wget:*)"}; !reflect.DeepEqual(p.DisallowedTools, want) {
		t.Fatalf("DisallowedTools = %v, want %v", p.DisallowedTools, want)
	}
	if want := []string{"Read", "Grep"}; !reflect.DeepEqual(p.AllowedTools, want) {
		t.Fatalf("AllowedTools = %v, want %v", p.AllowedTools, want)
	}
}

func TestLoadProfile_AToolPolicySentinelWithoutThePolicyFileFailsTheLoad(t *testing.T) {
	t.Parallel()
	path := writeToolListProfile(t, t.TempDir(), `{"name":"agent","disallowed_tools":["$include_policy:net"]}`)

	_, err := LoadProfile(path)

	if err == nil || !strings.Contains(err.Error(), "tool-policy.json") {
		t.Fatalf("LoadProfile err = %v, want a failure naming tool-policy.json", err)
	}
}

func TestRealize_ProjectsToolListsOnEveryTmuxDriverThatDeclaresThem(t *testing.T) {
	t.Parallel()
	intent := LaunchIntent{AllowedTools: []string{"Read", "Grep"}, DisallowedTools: []string{"Bash(curl:*)", "WebFetch"}}
	projecting := map[string][]string{
		"claude-tmux": {"--allowedTools", "Read", "Grep", "--disallowedTools", "Bash(curl:*)", "WebFetch"},
	}
	visited := map[string]bool{}
	for _, cli := range ManifestNames() {
		m, err := LoadManifest(cli)
		if err != nil {
			t.Fatalf("LoadManifest(%s): %v", cli, err)
		}
		if !m.IsTmux() {
			continue
		}
		visited[cli] = true
		t.Run(cli, func(t *testing.T) {
			flags := Realize(m, intent).LaunchFlags
			want, projects := projecting[cli]
			if projects {
				if !containsRun(flags, want) {
					t.Fatalf("%s flags = %v, want the run %v", cli, flags, want)
				}
				return
			}
			for _, tok := range []string{"--allowedTools", "--disallowedTools", "Bash(curl:*)", "WebFetch"} {
				if slices.Contains(flags, tok) {
					t.Fatalf("%s declares no tool-list param but its flags carry %q: %v", cli, tok, flags)
				}
			}
		})
	}
	for cli := range projecting {
		if !visited[cli] {
			t.Fatalf("%s is not an embedded tmux manifest; the projection proof is vacuous", cli)
		}
	}
}

func containsRun(flags, run []string) bool {
	for i := 0; i+len(run) <= len(flags); i++ {
		if slices.Equal(flags[i:i+len(run)], run) {
			return true
		}
	}
	return false
}

func TestEngineLaunch_ClaudeTmuxCarriesTheProfileDenyList(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	ws := t.TempDir()
	profile := writeToolListProfile(t, ws, `{"name":"agent","cli":"claude-tmux","allowed_tools":["Read"],"disallowed_tools":["Bash(curl:*)"]}`)
	artifact := filepath.Join(ws, "artifact.md")
	if err := os.WriteFile(artifact, []byte("DONE\n"), 0o644); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	tmux := &fakeTmux{paneSeq: []string{"❯"}}
	eng := NewEngine(Deps{Tmux: tmux, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil), CaptureBaseline: zeroBaselineCapture})

	resp, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: profile, Model: "sonnet", Prompt: "x",
		Workspace: ws, ArtifactPath: artifact, Agent: "agent",
	})

	if err != nil || resp.ExitCode != ExitOK {
		t.Fatalf("launch failed: exit=%d err=%v (stderr=%q)", resp.ExitCode, err, resp.Stderr)
	}
	got := launchedCmd(tmux, "claude")
	if !strings.Contains(got, " --allowedTools Read --disallowedTools 'Bash(curl:*)'") {
		t.Fatalf("claude-tmux launch must carry both profile tool lists; got %q", got)
	}
}

func TestRealProfiles_NoToolIsBothAllowedAndDenied(t *testing.T) {
	t.Parallel()
	paths, err := filepath.Glob(filepath.Join(realProfilesDir(t), "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("glob profiles: %v (found %d)", err, len(paths))
	}
	loaded := 0
	for _, path := range paths {
		if filepath.Base(path) == "tool-policy.json" {
			continue
		}
		if _, err := LoadProfile(path); err != nil {
			t.Errorf("%s: %v", filepath.Base(path), err)
		}
		loaded++
	}
	if loaded == 0 {
		t.Fatal("loaded no profiles; the disjointness proof is vacuous")
	}
}

func TestLoadProfile_RefusesAToolInBothLists(t *testing.T) {
	t.Parallel()
	path := writeToolListProfile(t, t.TempDir(), `{"name":"agent","allowed_tools":["Read","WebFetch"],"disallowed_tools":["Bash(curl:*)","WebFetch"]}`)

	_, err := LoadProfile(path)

	if err == nil || !strings.Contains(err.Error(), `"WebFetch"`) || !strings.Contains(err.Error(), "disallowed_tools") {
		t.Fatalf("LoadProfile err = %v, want a refusal naming WebFetch in both lists", err)
	}
}
