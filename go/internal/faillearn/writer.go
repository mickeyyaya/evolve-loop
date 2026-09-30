package faillearn

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const publishedFileMode fs.FileMode = 0o644

const unqueuedRetroName = "retrospective-unqueued.md"

func WriteArtifacts(ev FailureEvent, runDir, lessonsDir string, opts ...Option) error {
	var cfg writeConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	if err := cfg.writeInboxItems(); err != nil {
		return cfg.preserveDiagnosis(ev, runDir, err)
	}

	id, lesson := RenderLessonYAML(ev)

	hasCycleWorkspace := runDir != ""
	if hasCycleWorkspace {
		if _, err := writeIfAbsent(filepath.Join(runDir, "retrospective-report.md"), RenderRetrospectiveMarkdown(ev)); err != nil {
			return fmt.Errorf("faillearn: write retrospective: %w", err)
		}
	}
	if isNearDuplicate(lessonsDir, lesson, cfg.resolvedNoveltyThreshold()) {
		return nil
	}
	if _, err := writeIfAbsent(filepath.Join(lessonsDir, id+".yaml"), lesson); err != nil {
		return fmt.Errorf("faillearn: write lesson %s: %w", id, err)
	}
	return nil
}

func (c writeConfig) preserveDiagnosis(ev FailureEvent, runDir string, cause error) error {
	hasCycleWorkspace := runDir != ""
	if !hasCycleWorkspace {
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

func writeIfAbsent(path string, data []byte) (skipped bool, err error) {
	if _, err := os.Stat(path); err == nil {
		return true, nil
	} else if !os.IsNotExist(err) {
		return false, err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}
	// The temp file lives beside path because os.Link cannot cross filesystems.
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return false, err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return false, err
	}
	if err := tmp.Chmod(publishedFileMode); err != nil {
		_ = tmp.Close()
		return false, err
	}
	if err := tmp.Close(); err != nil {
		return false, err
	}
	// Link, not rename: it fails EEXIST against a concurrent winner instead of clobbering it.
	if err := os.Link(tmp.Name(), path); err != nil {
		if os.IsExist(err) {
			return true, nil
		}
		return false, err
	}
	return false, nil
}
