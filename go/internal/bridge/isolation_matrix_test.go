package bridge

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/attemptpostmortem"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

const newFamilyManifest = `{"cli":"newfam-tmux","binary":"newfam","transport":"tmux","default_tier":"balanced","prompt_marker":"$ ",
"parsers":{"pane":"claude-repl-v1"},"collectors":{"tokens":"claude-jsonl-v1"},
"transcript":{"format":"claude-jsonl-v1"},"capabilities":["tools","edit"],"home_reads":["~/.newfam"],"context_window":100000,
"controls":{"usage":{"send":"/usage","await":"windows","windows":{"labels":[{"regex":"^\\s*Current week\\s*$","kind":"week","scope":"all models"}],
"value_regex":"(?P<pct>\\d{1,3})%\\s*used","value_direction":"used","scopes":{"all models":{"family":"newfam"}}}}}}`

func newFamilyNeedsAGoChange(dir string) error {
	dec := json.NewDecoder(bytes.NewReader([]byte(newFamilyManifest)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&Manifest{}); err != nil {
		return fmt.Errorf("the manifest cannot declare its parsers: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "newfam-tmux.json"), []byte(newFamilyManifest), 0o644); err != nil {
		return err
	}
	if _, err := LoadManifest("newfam-tmux"); err != nil {
		return fmt.Errorf("the bridge refuses the manifest of the new family: %w", err)
	}
	if spec := mustManifest("newfam-tmux").usageWindowSpec(); spec == nil || len(quotastate.ReadWindows(*spec, "Current week\n40% used\n", time.Now())) != 1 {
		return errors.New("the usage windows of the new family are not read from its manifest")
	}
	if paneProfileFor(tmuxLaunch{name: "newfam-tmux", promptMarker: "$ "}).Name != panestream.FormatClaudeREPL {
		return errors.New("the pane profile of the new family is not the one that its manifest names")
	}
	window := manifestTokenWindow(tokenusage.Window{Driver: "newfam-tmux"})
	if !tokenusage.HasCollector(window.Collector) || window.ContextWindow != 100000 {
		return fmt.Errorf("the token window of the new family is %+v, not the collector and the window of its manifest", window)
	}
	if !attemptpostmortem.KnownTranscriptFormat(mustManifest("newfam-tmux").TranscriptFormat()) || !HasToolUse("newfam-tmux") {
		return errors.New("the transcript format or the tool capability of the new family is not read from its manifest")
	}
	return nil
}

func mustManifest(cli string) Manifest {
	m, _ := LoadManifest(cli)
	return m
}

func TestIsolationMatrix_N14_ANewManifestWithKnownFormatIDsNeedsNoGoChange(t *testing.T) {
	dir := t.TempDir()
	orig := bridgeManifestDirFn
	bridgeManifestDirFn = func() string { return dir }
	t.Cleanup(func() { bridgeManifestDirFn = orig })
	note, err := isolation.Judge("N14", newFamilyNeedsAGoChange(dir))
	if err != nil {
		t.Fatal(err)
	}
	t.Log(note)
}
