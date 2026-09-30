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

const ArtifactMaxBytes = 2 << 20

var ErrArtifactNotAllowed = errors.New("dashboard: artifact name not allowed")

var ErrArtifactTooLarge = errors.New("dashboard: artifact exceeds size cap")

var artifactName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*\.(md|json|txt|ndjson|log|yaml|yml)$`)

type ArtifactInfo struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

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
