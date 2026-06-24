package faillearn

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

// WriteArtifacts persists the deterministic learning artifacts for a
// failure event: retrospective-report.md into runDir and the failure
// lesson into lessonsDir. Existing files are preserved — the floor never
// clobbers a richer LLM-authored artifact. Writes are atomic
// (tmp+rename via internal/atomicwrite).
func WriteArtifacts(ev FailureEvent, runDir, lessonsDir string) error {
	id, lesson := RenderLessonYAML(ev)

	// Loop-scope fatals have no cycle workspace: empty runDir writes the
	// lesson only instead of inventing a report location.
	if runDir != "" {
		if err := writeIfAbsent(filepath.Join(runDir, "retrospective-report.md"), RenderRetrospectiveMarkdown(ev)); err != nil {
			return fmt.Errorf("faillearn: write retrospective: %w", err)
		}
	}
	if err := writeIfAbsent(filepath.Join(lessonsDir, id+".yaml"), lesson); err != nil {
		return fmt.Errorf("faillearn: write lesson %s: %w", id, err)
	}
	return nil
}

// writeIfAbsent atomically writes data to path unless it already exists.
// Single-writer assumption: the stat→write window is not exclusive
// (atomicwrite has no O_EXCL), which is fine for the floor's call graph
// — one orchestrator/reset/loop process per .evolve dir at a time.
func writeIfAbsent(path string, data []byte) error {
	if _, err := os.Stat(path); err == nil {
		return nil // existing artifact wins
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return atomicwrite.Bytes(path, data)
}
