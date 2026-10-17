package looppreflight

import (
	"errors"
	"io/fs"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

const agyTrailConfig = `{"mcpServers": {"trail": {"command": "serve"}, "docs": {}}}`

func fakeFiles(files map[string]string) func(string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		body, ok := files[path]
		if !ok {
			return nil, fs.ErrNotExist
		}
		return []byte(body), nil
	}
}

func mcpPort(files map[string]string, sources ...MCPConfigSource) MCPConfigPort {
	return MCPConfigPort{
		Sources:  func() ([]MCPConfigSource, error) { return sources, nil },
		ReadFile: fakeFiles(files),
		HomeDir:  func() (string, error) { return "/home/op", nil },
	}
}

var (
	agySource    = MCPConfigSource{Binary: "agy", Files: []string{"~/.gemini/config/mcp_config.json", "~/.gemini/antigravity/mcp_config.json"}}
	claudeSource = MCPConfigSource{Binary: "claude", Files: []string{".mcp.json"}}
)

func TestRun_MCPConfig_AConfiguredServerWarnsAndNeverHalts(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.ProjectRoot = "/repo"
	opts.MCPConfig = mcpPort(map[string]string{
		"/home/op/.gemini/config/mcp_config.json":      agyTrailConfig,
		"/home/op/.gemini/antigravity/mcp_config.json": `{"mcpServers": {"trail": {}}}`,
		"/repo/.mcp.json": `{"mcpServers": {"github": {}}}`,
	}, agySource, claudeSource)

	r, err := Run(opts)

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "mcp-config")
	want := []string{
		"WARN: agy has MCP servers configured (docs, trail); the repo rule is CLI only (docs/conventions/cli-only-no-mcp.md)",
		"WARN: claude has MCP servers configured (github); the repo rule is CLI only (docs/conventions/cli-only-no-mcp.md)",
	}
	if c.Level != LevelWarn || c.Detail != strings.Join(want, "\n") {
		t.Fatalf("mcp-config = %s %q, want warn %q", c.Level, c.Detail, want)
	}
	if r.Halted() {
		t.Fatalf("an MCP server must only warn; OverallLevel=%s", r.OverallLevel)
	}
}

func TestRun_MCPConfig_NoServerAndNoFileIsAPass(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.ProjectRoot = "/repo"
	opts.MCPConfig = mcpPort(map[string]string{
		"/home/op/.gemini/antigravity/mcp_config.json": `{"mcpServers": {}}`,
	}, agySource, claudeSource)

	r, err := Run(opts)

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if c := findCheck(t, r, "mcp-config"); c.Level != LevelPass {
		t.Fatalf("mcp-config = %s %q, want pass", c.Level, c.Detail)
	}
}

func TestMCPConfigWarnings_AnUnreadableConfigIsAWarningNotASilentPass(t *testing.T) {
	t.Parallel()
	port := mcpPort(map[string]string{"/repo/.mcp.json": "{not json"}, claudeSource)
	port.HomeDir = func() (string, error) { return "", errors.New("no home") }
	denied := MCPConfigPort{
		Sources:  port.Sources,
		ReadFile: func(string) ([]byte, error) { return nil, fs.ErrPermission },
		HomeDir:  port.HomeDir,
	}

	malformed := mcpConfigWarnings(port, "/repo")
	unresolved := mcpConfigWarnings(MCPConfigPort{
		Sources:  func() ([]MCPConfigSource, error) { return []MCPConfigSource{agySource}, nil },
		ReadFile: port.ReadFile,
		HomeDir:  port.HomeDir,
	}, "/repo")
	unreadable := mcpConfigWarnings(denied, "/repo")

	if len(malformed) != 1 || !strings.HasPrefix(malformed[0], "WARN: cannot read MCP config /repo/.mcp.json: ") {
		t.Errorf("malformed config warnings = %q", malformed)
	}
	if len(unresolved) != 2 || !strings.Contains(unresolved[0], "cannot resolve MCP config ~/.gemini/config/mcp_config.json: no home") {
		t.Errorf("unresolved home warnings = %q", unresolved)
	}
	if len(unreadable) != 1 || !strings.Contains(unreadable[0], "permission denied") {
		t.Errorf("unreadable config warnings = %q", unreadable)
	}
}

func TestMCPConfigWarnings_ASourceErrorIsAWarningBesideTheSourcesThatLoaded(t *testing.T) {
	t.Parallel()
	port := mcpPort(map[string]string{"/repo/.mcp.json": `{"mcpServers": {"x": {}}}`})
	port.Sources = func() ([]MCPConfigSource, error) {
		return []MCPConfigSource{claudeSource}, errors.New("manifest broken")
	}

	got := mcpConfigWarnings(port, "/repo")

	if len(got) != 2 || got[0] != "WARN: cannot list the MCP config files of every CLI: manifest broken" || !strings.Contains(got[1], "claude has MCP servers configured (x)") {
		t.Fatalf("warnings = %q", got)
	}
}

func TestManifestMCPConfigSources_MergesTheFilesOfOneBinaryAndReportsALoadError(t *testing.T) {
	t.Parallel()
	manifests := map[string]bridge.Manifest{
		"agy-tmux":        {Binary: "agy", MCPConfigFiles: []string{"~/a.json", "~/b.json"}},
		"agy-claude-tmux": {Binary: "agy", MCPConfigFiles: []string{"~/b.json"}},
		"claude-tmux":     {Binary: "claude", MCPConfigFiles: []string{".mcp.json"}},
		"codex-tmux":      {Binary: "codex"},
	}
	load := func(name string) (bridge.Manifest, error) {
		m, ok := manifests[name]
		if !ok {
			return bridge.Manifest{}, errors.New("no manifest " + name)
		}
		return m, nil
	}

	got, err := manifestMCPConfigSources([]string{"agy-claude-tmux", "agy-tmux", "claude-tmux", "codex-tmux", "gone"}, load)

	want := []MCPConfigSource{
		{Binary: "agy", Files: []string{"~/b.json", "~/a.json"}},
		{Binary: "claude", Files: []string{".mcp.json"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sources = %+v, want %+v", got, want)
	}
	if err == nil || !strings.Contains(err.Error(), "no manifest gone") {
		t.Errorf("err = %v, want the load error of gone", err)
	}
}

func TestDefaultMCPConfigPort_ReadsTheShippedManifests(t *testing.T) {
	t.Parallel()
	port := defaultMCPConfigPort(MCPConfigPort{})

	sources, err := port.Sources()

	if err != nil {
		t.Fatalf("Sources: %v", err)
	}
	files := map[string][]string{}
	for _, s := range sources {
		files[s.Binary] = s.Files
	}
	if !slices.Contains(files["claude"], ".mcp.json") {
		t.Errorf("claude MCP files = %v, want the project .mcp.json", files["claude"])
	}
	if !slices.Contains(files["agy"], "~/.gemini/antigravity/mcp_config.json") {
		t.Errorf("agy MCP files = %v, want agy's mcp_config.json", files["agy"])
	}
	if home, err := port.HomeDir(); err != nil || home == "" {
		t.Errorf("HomeDir = %q, %v", home, err)
	}
	if _, err := port.ReadFile(filepath.Join(t.TempDir(), "absent")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadFile of an absent file = %v, want fs.ErrNotExist", err)
	}
}
