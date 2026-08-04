package faillearn

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteArtifacts persists the deterministic learning artifacts for a
// failure event: retrospective-report.md into runDir and the failure
// lesson into lessonsDir. Existing files are preserved — the floor never
// clobbers a richer LLM-authored artifact. Writes are atomic
// (tmp+rename via internal/atomicwrite).
//
// WithInbox additionally routes the retro's remediation items into
// `.evolve/inbox` (F1(ii)). Those go FIRST, and their failure aborts before the
// retrospective is written: the invariant is that a retrospective on disk can
// never claim remediation that never reached the queue. The reverse order would
// make the exact 1255 state ("filed" in the report, absent from the inbox) a
// reachable outcome of a partial failure. Rollback of already-written items is
// deliberately not attempted — writeIfAbsent makes the retry idempotent, and
// leaving a real remediation item queued is the safe direction to fail in.
func WriteArtifacts(ev FailureEvent, runDir, lessonsDir string, opts ...Option) error {
	var cfg writeConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	if err := cfg.writeInboxItems(); err != nil {
		return err
	}

	id, lesson := RenderLessonYAML(ev)

	// Loop-scope fatals have no cycle workspace: empty runDir writes the
	// lesson only instead of inventing a report location.
	if runDir != "" {
		if _, err := writeIfAbsent(filepath.Join(runDir, "retrospective-report.md"), RenderRetrospectiveMarkdown(ev)); err != nil {
			return fmt.Errorf("faillearn: write retrospective: %w", err)
		}
	}
	if _, err := writeIfAbsent(filepath.Join(lessonsDir, id+".yaml"), lesson); err != nil {
		return fmt.Errorf("faillearn: write lesson %s: %w", id, err)
	}
	return nil
}

// writeIfAbsent atomically writes data to path unless it already exists, and
// REPORTS whether it skipped. Preserving a richer existing artifact is the
// intended behavior for the retrospective and the lesson; for the inbox it is a
// dropped remediation item, so the caller — not this function — decides what a
// skip means (cycle-1282 DEF-4: a silent skip made "the retro was written"
// true while "the remediation was queued" was false, which is precisely the
// 1255 state WithInbox exists to make unreachable).
//
// Exclusive by construction (cycle-1285 F4). The previous shape was
// stat-then-write, so two fleet lanes could both observe "absent" and both
// write, and the DEF-4 content-equality check above only ever observed that
// race after the fact. Publishing with os.Link makes "does not exist" and "I
// created it" ONE atomic step: link fails EEXIST against a concurrent winner,
// which is reported as an ordinary skip and re-enters the DEF-4 check. The
// temp file is written whole before it is linked, so a crash mid-write can
// never leave a partial artifact under the real name — the property
// atomicwrite gave us and rename-based publishing would give away here,
// because rename silently CLOBBERS an existing file.
func writeIfAbsent(path string, data []byte) (skipped bool, err error) {
	if _, err := os.Stat(path); err == nil {
		return true, nil // existing artifact wins — cheap path, no temp file
	} else if !os.IsNotExist(err) {
		return false, err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}
	// Same directory as the destination: os.Link is only defined within one
	// filesystem, and a temp dir elsewhere would fail EXDEV.
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return false, err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return false, err
	}
	if err := tmp.Close(); err != nil {
		return false, err
	}
	if err := os.Link(tmp.Name(), path); err != nil {
		if os.IsExist(err) {
			return true, nil // a concurrent writer won the create
		}
		return false, err
	}
	return false, nil
}
