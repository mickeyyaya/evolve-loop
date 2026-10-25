package loopchain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func LoadChainConfig(evolveDir string) policy.ChainConfig {
	return policy.LoadOrDefaults(paths.PolicyPath(evolveDir), os.Stderr).ChainConfig()
}

func InboxPendingCount(evolveDir string) (int, []string, error) {
	dir := filepath.Join(evolveDir, "inbox")
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil, nil
		}
		return 0, nil, fmt.Errorf("read inbox: %w", err)
	}
	n := 0
	var skipped []string
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if isInboxItemFile(filepath.Join(dir, e.Name())) {
			n++
			continue
		}
		skipped = append(skipped, e.Name())
	}
	return n, skipped, nil
}

func isInboxItemFile(path string) bool {
	buf, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var doc map[string]any
	if err := json.Unmarshal(buf, &doc); err != nil {
		return false
	}
	id, _ := doc["id"].(string)
	return strings.TrimSpace(id) != ""
}

func BrakeEngaged(evolveDir string) bool {
	_, err := os.Stat(paths.LoopStopPath(evolveDir))
	return err == nil
}
