package bridge

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/attemptpostmortem"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

func TestEmbeddedManifests_DecodeStrictlyAndNameOnlyKnownFormatIDs(t *testing.T) {
	t.Parallel()
	names := ManifestNames()
	if len(names) < 8 {
		t.Fatalf("ManifestNames() = %v; the schema test lost its corpus", names)
	}
	for _, cli := range names {
		data, err := embeddedManifests.ReadFile("manifests/" + cli + ".json")
		if err != nil {
			t.Fatal(err)
		}
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		var raw struct {
			Manifest
			Base string `json:"base"`
		}
		if err := dec.Decode(&raw); err != nil {
			t.Errorf("manifest %s has a field that the schema does not know: %v", cli, err)
		}
		if _, err := LoadManifest(cli); err != nil {
			t.Errorf("LoadManifest(%s) = %v", cli, err)
		}
	}
}

func TestParseManifest_RefusesAnUnknownFormatIDKindCapabilityOrHomeRead(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		`"parsers":{"pane":"newfam-repl-v9"}`:      `parsers.pane for cli=x names "newfam-repl-v9", which is not a known format id`,
		`"parsers":{"usage":"claude-usage-v1"}`:    `parsers.usage for cli=x is not a known kind; the kinds are pane`,
		`"parsers":{"pane":"claude"}`:              `parsers.pane for cli=x names "claude"`,
		`"home_reads":["~/."]`:                     `home_reads entry "~/." for cli=x must start with ~/`,
		`"home_reads":["~/.SSH/id_rsa"]`:           `overlaps the secret path ~/.ssh`,
		`"collectors":{"tokens":"claude-tmux"}`:    `collectors.tokens for cli=x names "claude-tmux"`,
		`"transcript":{"format":"codex-jsonl-v1"}`: `transcript.format "codex-jsonl-v1" for cli=x is not a known format id`,
		`"transcript":{}`:                          `transcript.format "" for cli=x`,
		`"capabilities":["tools","fly"]`:           `capabilities entry "fly" for cli=x is not a known capability; the capabilities are tools, edit`,
		`"home_reads":["~/.ssh/known_hosts"]`:      `home_reads entry "~/.ssh/known_hosts" for cli=x overlaps the secret path ~/.ssh`,
		`"home_reads":["~/.config"]`:               `overlaps the secret path ~/.config/gh`,
		`"home_reads":["~/.netrc"]`:                `overlaps the secret path ~/.netrc`,
		`"home_reads":["/etc/passwd"]`:             `home_reads entry "/etc/passwd" for cli=x must start with ~/`,
		`"home_reads":["~/"]`:                      `home_reads entry "~/" for cli=x must start with ~/`,
		`"home_reads":["~/a/../.ssh"]`:             `carry no .. segment`,
		`"context_window":-1`:                      `context_window for cli=x is -1`,
	}
	for field, want := range cases {
		_, err := parseManifestWithStderr("x", []byte(`{"cli":"x","binary":"x",`+field+`}`), io.Discard)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("manifest with %s: err = %v, want %q", field, err, want)
		}
	}
	ok := `{"cli":"x","binary":"x","parsers":{"pane":"agy-repl-v1"},"collectors":{"tokens":"agy-conversation-v1"},"transcript":{"format":"claude-jsonl-v1"},"capabilities":["tools","edit"],"home_reads":["~/.x","~/.sshx"],"context_window":1}`
	if _, err := parseManifestWithStderr("x", []byte(ok), io.Discard); err != nil {
		t.Errorf("a manifest of known format ids is refused: %v", err)
	}
}

func TestManifests_CarryTheCollectorsWindowsAndCapabilitiesThatGoHeldBefore(t *testing.T) {
	t.Parallel()
	cases := []struct {
		cli, tokens, transcript string
		window                  int
		tools                   bool
	}{
		{"claude-p", tokenusage.FormatClaudeJSONL, attemptpostmortem.FormatClaudeJSONL, 200_000, true},
		{"claude-tmux", tokenusage.FormatClaudeJSONL, attemptpostmortem.FormatClaudeJSONL, 200_000, true},
		{"agy", tokenusage.FormatAgyConversation, "", 200_000, true},
		{"agy-tmux", tokenusage.FormatAgyConversation, "", 200_000, true},
		{"agy-claude-tmux", tokenusage.FormatAgyConversation, "", 200_000, true},
		{"codex", "", "", 200_000, true},
		{"codex-tmux", "", "", 200_000, true},
		{"ollama-tmux", "", "", 0, false},
	}
	for _, tc := range cases {
		m, err := LoadManifest(tc.cli)
		if err != nil {
			t.Fatal(err)
		}
		window := manifestTokenWindow(tokenusage.Window{Driver: tc.cli})
		if window.Collector != tc.tokens || window.ContextWindow != tc.window || m.TranscriptFormat() != tc.transcript || HasToolUse(tc.cli) != tc.tools {
			t.Errorf("%s: collector %q window %d transcript %q tools %v; want %q %d %q %v", tc.cli, window.Collector, window.ContextWindow, m.TranscriptFormat(), HasToolUse(tc.cli), tc.tokens, tc.window, tc.transcript, tc.tools)
		}
	}
	if w := manifestTokenWindow(tokenusage.Window{Driver: "no-such-cli", TokenLineRegex: "x"}); w.Collector != "" || w.ContextWindow != 0 || w.TokenLineRegex != "x" {
		t.Errorf("a driver with no manifest = %+v, want the window unchanged", w)
	}
}

func TestParseManifest_AManifestWithNoCapabilitiesSaysSoOnStderr(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	m, err := parseManifestWithStderr("old", []byte(`{"cli":"old","binary":"old"}`), &stderr)
	if err != nil || m.HasCapability(CapabilityTools) || !strings.Contains(stderr.String(), "cli=old declares no capabilities: it has no tool use") {
		t.Fatalf("a manifest written before X9 = %+v, %v, stderr %q; want it loaded, toolless and named on stderr", m, err, stderr.String())
	}
	stderr.Reset()
	if _, err := parseManifestWithStderr("none", []byte(`{"cli":"none","binary":"none","capabilities":[]}`), &stderr); err != nil || stderr.Len() != 0 {
		t.Fatalf("a manifest that declares no capability on purpose: err %v, stderr %q; want silence", err, stderr.String())
	}
}
