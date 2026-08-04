package faillearn

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// publishedMode is the mode every published runtime artifact lands at —
// the same literal internal/atomicwrite.Bytes documents and enforces
// (atomicwrite.go:61-63). The floor's own artifacts are read by the OTHER
// fleet lanes and by the operator, so os.CreateTemp's 0600 (which is not
// umask-derived and therefore cannot be configured away) is a defect, not a
// stricter default.
const publishedFileMode fs.FileMode = 0o644

// unqueuedRetroName is the DEGRADED retrospective: the diagnosis published when
// the remediation queue write failed. Deliberately NOT retrospective-report.md —
// that name means "the remediation is queued", and the 1255 invariant is that it
// may never appear otherwise. A distinct name keeps the invariant exact while
// letting the failure analysis outlive the queue write.
const unqueuedRetroName = "retrospective-unqueued.md"

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
		return cfg.preserveDiagnosis(ev, runDir, err)
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

// preserveDiagnosis publishes the failure analysis under unqueuedRetroName after
// the queue write failed, and returns cause unchanged so the caller still fails
// loudly. Preserving the diagnosis is an ADDITION to the abort, never a
// replacement for it.
//
// The residual the cycle-1287 landing named rather than closed: the abort
// ordering is correct — a retrospective may not claim remediation that reached no
// queue — but an unwritable inbox left ZERO artifacts on disk, so the analysis of
// the failure died together with the failure it described, and the next
// continuation had to re-derive it. The degraded artifact carries the diagnosis
// plus the ids of every item that did NOT reach the queue, which is the work that
// would otherwise be lost.
//
// A failure to publish the degraded artifact is joined onto cause rather than
// replacing it: the queue failure is the diagnosis the caller acts on.
func (c writeConfig) preserveDiagnosis(ev FailureEvent, runDir string, cause error) error {
	// Loop-scope fatals have no cycle workspace. An empty runDir means there is
	// no place to publish into, not that the process cwd is one.
	if runDir == "" {
		return cause
	}
	var b bytes.Buffer
	b.WriteString("<!-- faillearn: degraded retrospective — remediation UNQUEUED -->\n\n")
	b.WriteString("# UNQUEUED — this diagnosis reached no remediation queue\n\n")
	fmt.Fprintf(&b, "The remediation queue write failed (`%v`), so `retrospective-report.md` was\n", cause)
	b.WriteString("deliberately NOT written: on disk, that name asserts the remediation was queued.\n")
	b.WriteString("This degraded artifact preserves the analysis so it can be requeued by hand or\n")
	b.WriteString("reconciled by the next continuation.\n\n")
	b.WriteString("## Remediation items that are still UNQUEUED\n\n")
	unqueued := c.unqueuedItems()
	if len(unqueued) == 0 {
		b.WriteString("- (none recorded)\n")
	}
	for _, it := range unqueued {
		fmt.Fprintf(&b, "- `%s` — %s\n", it.ID, it.Title)
	}
	b.WriteString("\n")
	b.Write(RenderRetrospectiveMarkdown(ev))

	if _, err := writeIfAbsent(filepath.Join(runDir, unqueuedRetroName), b.Bytes()); err != nil {
		return errors.Join(cause, fmt.Errorf("faillearn: write %s: %w", unqueuedRetroName, err))
	}
	return cause
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
	// Chmod the TEMP, before it is linked — never the destination. os.CreateTemp
	// creates at 0600, os.Link preserves it, and the link publishes the inode
	// itself, so the mode must be right before publication or the artifact is
	// briefly (and then permanently) 0600. Chmod-ing path instead would also
	// rewrite the mode of a PRESERVED pre-existing artifact on the skip path,
	// which writeIfAbsent's contract says must be left entirely alone.
	if err := tmp.Chmod(publishedFileMode); err != nil {
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
