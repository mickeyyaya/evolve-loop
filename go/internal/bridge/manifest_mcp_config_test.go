package bridge

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestParseManifest_RejectsAnUnsafeMCPConfigEntry(t *testing.T) {
	t.Parallel()
	for _, entry := range []string{"/etc/mcp.json", "../mcp.json", "a/../../mcp.json", "~/../x.json"} {
		data := fmt.Sprintf(`{"cli":"x-tmux","binary":"x","mcp_config_files":[%q]}`, entry)

		_, err := parseManifestWithStderr("x-tmux", []byte(data), &bytes.Buffer{})

		if err == nil || !strings.Contains(err.Error(), "x-tmux") || !strings.Contains(err.Error(), entry) || !strings.Contains(err.Error(), "mcp_config_files") {
			t.Errorf("entry %q: err = %v, want a refusal naming the manifest and the entry", entry, err)
		}
	}
}

func TestParseManifest_AcceptsRelativeAndHomeMCPConfigEntries(t *testing.T) {
	t.Parallel()
	data := `{"cli":"x-tmux","binary":"x","mcp_config_files":[".mcp.json","~/.gemini/mcp_config.json"]}`

	m, err := parseManifestWithStderr("x-tmux", []byte(data), &bytes.Buffer{})

	if err != nil || len(m.MCPConfigFiles) != 2 {
		t.Fatalf("m=%v err=%v, want both entries accepted", m.MCPConfigFiles, err)
	}
}

func TestEmbeddedManifests_ListTheirMCPConfigFiles(t *testing.T) {
	t.Parallel()
	for cli, want := range map[string][]string{
		"agy-tmux":    {"~/.gemini/config/mcp_config.json", "~/.gemini/antigravity/mcp_config.json"},
		"claude-tmux": {".mcp.json"},
	} {
		m, err := loadManifestRaw(cli)
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprint(m.MCPConfigFiles) != fmt.Sprint(want) {
			t.Errorf("%s mcp_config_files = %v, want %v", cli, m.MCPConfigFiles, want)
		}
	}
}
