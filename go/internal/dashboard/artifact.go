package dashboard

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
)

// ArtifactMaxBytes caps a single artifact read. Audit reports are budgeted at
// 32 KiB; logs and interaction ledgers can run to megabytes — the page shows
// the first 2 MiB and says so rather than streaming an unbounded file.
const ArtifactMaxBytes = 2 << 20

// ErrArtifactNotAllowed is returned for a name outside the allowlist: path
// separators, dot-segments, hidden files, or an unknown extension.
var ErrArtifactNotAllowed = errors.New("dashboard: artifact name not allowed")

// ErrArtifactTooLarge is returned when the file exceeds ArtifactMaxBytes.
var ErrArtifactTooLarge = errors.New("dashboard: artifact exceeds size cap")

// artifactName is the allowlist: a plain file name with a known extension.
var artifactName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*\.(md|json|txt|ndjson|log|yaml|yml)$`)

// ArtifactInfo describes one file in a cycle workspace.
type ArtifactInfo struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

// ListArtifacts returns the readable files in cycle's workspace, sorted by
// name. Sub-directories, lock sidecars and disallowed names are omitted.
func ListArtifacts(root string, cycle int) ([]ArtifactInfo, error) {
	entries, err := os.ReadDir(core.RunWorkspacePath(root, cycle))
	if err != nil {
		return nil, err
	}
	out := make([]ArtifactInfo, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !artifactName.MatchString(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		out = append(out, ArtifactInfo{Name: e.Name(), Size: info.Size(), ModTime: info.ModTime()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ReadArtifact returns the bytes of one allowlisted, regular (never symlinked)
// file from cycle's workspace, bounded by ArtifactMaxBytes. The content is
// LLM-authored; callers must treat it as text, never as markup.
func ReadArtifact(root string, cycle int, name string) ([]byte, error) {
	if !artifactName.MatchString(name) || strings.Contains(name, "..") {
		return nil, ErrArtifactNotAllowed
	}
	path := filepath.Join(core.RunWorkspacePath(root, cycle), name)
	lst, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !lst.Mode().IsRegular() {
		return nil, ErrArtifactNotAllowed
	}
	if lst.Size() > ArtifactMaxBytes {
		return nil, fmt.Errorf("%w: %s is %d bytes", ErrArtifactTooLarge, name, lst.Size())
	}
	f, err := reportdoc.OpenRegularNoFollow(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(lst, st) {
		return nil, ErrArtifactNotAllowed
	}
	return io.ReadAll(io.LimitReader(f, ArtifactMaxBytes))
}
