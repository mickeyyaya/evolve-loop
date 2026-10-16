package recurrence

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/yamlcodec"
)

type lessonEntry struct {
	ID             string `yaml:"id"`
	Pattern        string `yaml:"pattern"`
	FailureContext struct {
		ErrorCategory string `yaml:"errorCategory"`
	} `yaml:"failureContext"`
}

func BackfillFromLessons(lessonsDir string, pol EscalationPolicy) (*Ledger, []string, error) {
	files, err := filepath.Glob(filepath.Join(lessonsDir, "*.yaml"))
	if err != nil {
		return nil, nil, fmt.Errorf("recurrence: glob lessons %s: %w", lessonsDir, err)
	}
	sort.Strings(files)
	led := NewLedger()
	var skipped []string
	for _, f := range files {
		data, readErr := os.ReadFile(f)
		if readErr != nil {
			skipped = append(skipped, filepath.Base(f))
			continue
		}
		var lessons []lessonEntry
		if yamlcodec.Unmarshal(data, &lessons) != nil {
			skipped = append(skipped, filepath.Base(f))
			continue
		}
		recorded := false
		for _, ls := range lessons {
			pat := strings.TrimSpace(ls.Pattern)
			if pat == "" {
				continue
			}
			cyc, ok := cycleFromID(ls.ID)
			if !ok {
				continue
			}
			_ = led.RecordClosure(pat, cyc, nil, nil, pol)
			if e := led.Entries[pat]; e != nil && IsGeneric(pat, ls.FailureContext.ErrorCategory) {
				e.Generic = true
			}
			recorded = true
		}
		if !recorded {
			skipped = append(skipped, filepath.Base(f))
		}
	}
	return led, skipped, nil
}

func cycleFromID(id string) (int, bool) {
	rest := strings.TrimPrefix(id, "cycle-")
	if rest == id {
		return 0, false
	}
	i := strings.IndexByte(rest, '-')
	if i <= 0 {
		return 0, false
	}
	n, err := strconv.Atoi(rest[:i])
	if err != nil {
		return 0, false
	}
	return n, true
}
