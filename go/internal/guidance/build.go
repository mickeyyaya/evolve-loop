package guidance

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/skillcheck"
)

const AgentsPath = "AGENTS.md"

var ErrNoL0Region = errors.New("AGENTS.md has no GENERATED:guidance-l0 region; add the BEGIN and END markers inside the instructions digest")

type Output struct {
	Path    string
	Content string
}

func Outputs(root string, rules []Rule) ([]Output, error) {
	agents, err := os.ReadFile(filepath.Join(root, AgentsPath))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", AgentsPath, err)
	}
	if !strings.Contains(string(agents), L0Begin) {
		return nil, ErrNoL0Region
	}
	spliced, err := skillcheck.SpliceMarkedRegion(string(agents), L0Block(rules), L0Begin, L0End, "")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", AgentsPath, err)
	}
	return []Output{
		{Path: IndexMarkdownPath, Content: IndexMarkdown(rules)},
		{Path: indexJSONPath, Content: IndexJSON(rules)},
		{Path: AgentsPath, Content: spliced},
	}, nil
}

func stale(root string, outs []Output) ([]Output, error) {
	var drifted []Output
	for _, o := range outs {
		current, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(o.Path)))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read %s: %w", o.Path, err)
		}
		if string(current) != o.Content {
			drifted = append(drifted, o)
		}
	}
	return drifted, nil
}

func Write(root string, outs []Output) ([]string, error) {
	drifted, err := stale(root, outs)
	if err != nil {
		return nil, err
	}
	var written []string
	for _, o := range drifted {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(o.Path)), []byte(o.Content), 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", o.Path, err)
		}
		written = append(written, o.Path)
	}
	return written, nil
}
