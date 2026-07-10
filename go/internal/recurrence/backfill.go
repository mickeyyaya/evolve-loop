package recurrence

// backfill.go — cycle-662 gap G2 (HISTORICAL BACKFILL). Cycle 661 landed the
// ledger core but it starts EMPTY, so Count()==0 for every historical pattern and
// the recurrence signal is dormant. BackfillFromLessons seeds a fresh ledger from
// the on-disk lesson corpus (.evolve/instincts/lessons/*.yaml), tolerating both
// the deterministic-floor and richer LLM lesson shapes, so the historical
// recurrence chains the advisory channel merely noticed become first-class counts.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// lessonEntry is the minimal read view of a corpus lesson: only the fields the
// backfill scan needs (id → cycle, pattern → ledger key, errorCategory → generic
// echo test). Mirrors the write shape in internal/faillearn.RenderLessonYAML.
type lessonEntry struct {
	ID             string `yaml:"id"`
	Pattern        string `yaml:"pattern"`
	FailureContext struct {
		ErrorCategory string `yaml:"errorCategory"`
	} `yaml:"failureContext"`
}

// BackfillFromLessons scans lessonsDir/*.yaml and records each (pattern, cycle)
// closure into a fresh ledger, tolerating BOTH lesson shapes (deterministic-floor
// where pattern==errorCategory, and the richer LLM shape). The cycle is parsed
// from the `cycle-<N>-` id prefix; recurrence count == distinct cycles. Each
// entry's Generic flag is set via IsGeneric. A malformed or empty file is SKIPPED
// (its basename returned in the second result) and never aborts the scan; the
// error is non-nil only for a fatal I/O failure (dir unreadable), not per-file
// parse failures. Escalator/Autofiler are nil — backfill counts history, it does
// not escalate.
func BackfillFromLessons(lessonsDir string, pol EscalationPolicy) (*Ledger, []string, error) {
	files, err := filepath.Glob(filepath.Join(lessonsDir, "*.yaml"))
	if err != nil {
		return nil, nil, fmt.Errorf("recurrence: glob lessons %s: %w", lessonsDir, err)
	}
	sort.Strings(files) // deterministic scan order
	led := NewLedger()
	var skipped []string
	for _, f := range files {
		data, readErr := os.ReadFile(f)
		if readErr != nil {
			skipped = append(skipped, filepath.Base(f))
			continue
		}
		var lessons []lessonEntry
		if yaml.Unmarshal(data, &lessons) != nil {
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
			// nil esc/af: backfill counts history, escalation apply stays
			// boundary-only. RecordClosure never errors with nil seams.
			_ = led.RecordClosure(pat, cyc, nil, nil, pol)
			if e := led.Entries[pat]; e != nil && IsGeneric(pat, ls.FailureContext.ErrorCategory) {
				e.Generic = true // sticky-true: a pattern generic in any lesson stays generic
			}
			recorded = true
		}
		if !recorded {
			skipped = append(skipped, filepath.Base(f))
		}
	}
	return led, skipped, nil
}

// cycleFromID extracts N from a lesson id of the form "cycle-<N>-<slug>". Returns
// ok=false when the id lacks the prefix or N is not an integer.
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
