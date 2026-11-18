// Package incidentbundle builds and checks the redacted evidence bundle of an incident record.
// See docs/architecture/packages/internal-incidentbundle.md.
package incidentbundle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/inputindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/redact"
)

const (
	Schema       = "incident-bundle/1"
	StateFull    = "full"
	StatePointer = "pointer"
	ManifestName = "manifest.json"
	RootDir      = "knowledge-base/incidents"
)

var idRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-[a-z0-9][a-z0-9-]*$`)

var digitRunRE = regexp.MustCompile(`\d+`)

var errorMarkers = []string{"ERROR", "FATAL", "panic:"}

type Entry struct {
	inputindex.Entry
	CapturedAt    string   `json:"captured_at"`
	SourceAddress string   `json:"source_address"`
	SourceSHA256  string   `json:"source_sha256"`
	Redactions    int      `json:"redactions"`
	State         string   `json:"state"`
	Templates     []string `json:"templates,omitempty"`
}

type Manifest struct {
	Schema  string  `json:"schema"`
	ID      string  `json:"id"`
	Entries []Entry `json:"entries"`
}

func Dir(root, id string) string { return filepath.Join(root, filepath.FromSlash(RootDir), id) }

func ValidID(id string) bool { return idRE.MatchString(id) }

func Add(root, id, source string, budgetBytes int, now time.Time) (Entry, error) {
	if !ValidID(id) {
		return Entry{}, fmt.Errorf("incident id %q must look like yyyy-mm-dd-slug", id)
	}
	raw, err := os.ReadFile(source)
	if err != nil {
		return Entry{}, fmt.Errorf("read source: %w", err)
	}
	dir := Dir(root, id)
	manifest, err := loadOrNew(dir, id)
	if err != nil {
		return Entry{}, err
	}
	sum := digest(raw)
	for _, held := range manifest.Entries {
		if held.SourceSHA256 == sum {
			return held, nil
		}
	}
	entry, body := newEntry(len(manifest.Entries)+1, source, raw, sum, now)
	manifest.Entries = append(manifest.Entries, entry)
	steps := []func() error{
		func() error { return atomicwrite.Bytes(filepath.Join(dir, entry.Path), body) },
		func() error { return evictOverBudget(dir, manifest.Entries, budgetBytes) },
		func() error { return atomicwrite.JSON(filepath.Join(dir, ManifestName), manifest) },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return Entry{}, err
		}
	}
	return manifest.Entries[len(manifest.Entries)-1], nil
}

func Check(root, id string) ([]string, error) {
	if !ValidID(id) {
		return nil, fmt.Errorf("incident id %q must look like yyyy-mm-dd-slug", id)
	}
	dir := Dir(root, id)
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	manifest, err := load(dir)
	if err != nil {
		return nil, err
	}
	var problems []string
	listed := map[string]bool{ManifestName: true}
	for _, entry := range manifest.Entries {
		if entry.State == StateFull {
			listed[entry.Path] = true
			problems = append(problems, checkCopy(dir, entry)...)
		}
	}
	problems = append(problems, unlistedFiles(files, listed)...)
	return problems, nil
}

func CheckAll(root string) (map[string][]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(RootDir)))
	if errors.Is(err, os.ErrNotExist) {
		return map[string][]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := map[string][]string{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		problems, err := Check(root, e.Name())
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		out[e.Name()] = problems
	}
	return out, nil
}

func newEntry(seq int, source string, raw []byte, sum string, now time.Time) (Entry, []byte) {
	redacted := redact.Secrets(string(raw))
	body := []byte(redacted)
	id := fmt.Sprintf("src-%02d", seq)
	copySum := digest(body)
	return Entry{
		Entry: inputindex.Entry{
			ID:      id,
			Class:   "evidence",
			Kind:    "log",
			Path:    id + "-" + filepath.Base(source),
			Summary: fmt.Sprintf("copy of %s, %d bytes", filepath.Base(source), len(body)),
			Bytes:   int64(len(body)),
			SHA256:  copySum,
		},
		CapturedAt:    now.UTC().Format(time.RFC3339),
		SourceAddress: source,
		SourceSHA256:  sum,
		Redactions:    strings.Count(redacted, redact.Token) - strings.Count(string(raw), redact.Token),
		State:         StateFull,
		Templates:     errorTemplates(redacted),
	}, body
}

func errorTemplates(text string) []string {
	seen := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		if template, ok := templateOf(line); ok {
			seen[template] = true
		}
	}
	out := make([]string, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

func templateOf(line string) (string, bool) {
	marked := false
	for _, m := range errorMarkers {
		marked = marked || strings.Contains(line, m)
	}
	if !marked {
		return "", false
	}
	fields := strings.Fields(line)
	if len(fields) > 0 && fields[0][0] >= '0' && fields[0][0] <= '9' {
		fields = fields[1:]
	}
	return digitRunRE.ReplaceAllString(strings.Join(fields, " "), "#"), len(fields) > 0
}

func evictOverBudget(dir string, entries []Entry, budgetBytes int) error {
	for fullBytes(entries) > int64(budgetBytes) {
		idx := oldestRepeat(entries)
		if idx < 0 {
			return nil
		}
		if err := os.Remove(filepath.Join(dir, entries[idx].Path)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("drop %s: %w", entries[idx].Path, err)
		}
		entries[idx].State = StatePointer
		entries[idx].Path = ""
		entries[idx].SHA256 = ""
	}
	return nil
}

func fullBytes(entries []Entry) int64 {
	var total int64
	for _, e := range entries {
		if e.State == StateFull {
			total += e.Bytes
		}
	}
	return total
}

func oldestRepeat(entries []Entry) int {
	for i, e := range entries {
		if e.State == StateFull && templatesHeldElsewhere(entries, i) {
			return i
		}
	}
	return -1
}

func templatesHeldElsewhere(entries []Entry, skip int) bool {
	held := map[string]bool{}
	for i, e := range entries {
		if i == skip || e.State != StateFull {
			continue
		}
		for _, t := range e.Templates {
			held[t] = true
		}
	}
	for _, t := range entries[skip].Templates {
		if !held[t] {
			return false
		}
	}
	return true
}

func checkCopy(dir string, entry Entry) []string {
	body, err := os.ReadFile(filepath.Join(dir, entry.Path))
	if err != nil {
		return []string{fmt.Sprintf("%s: copy is unreadable: %v", entry.ID, err)}
	}
	var problems []string
	if got := digest(body); got != entry.SHA256 {
		problems = append(problems, fmt.Sprintf("%s: %s changed since capture (sha256 %s, manifest %s)", entry.ID, entry.Path, got, entry.SHA256))
	}
	text := string(body)
	if redact.Secrets(text) != text {
		problems = append(problems, fmt.Sprintf("%s: %s holds a secret that redaction would replace", entry.ID, entry.Path))
	}
	for _, run := range redact.EntropyRuns(text) {
		problems = append(problems, fmt.Sprintf("%s: %s holds a high-entropy run of %d characters", entry.ID, entry.Path, len(run)))
	}
	return problems
}

func unlistedFiles(files []os.DirEntry, listed map[string]bool) []string {
	var problems []string
	for _, e := range files {
		if !listed[e.Name()] {
			problems = append(problems, fmt.Sprintf("%s is in the bundle but not in the manifest", e.Name()))
		}
	}
	return problems
}

func loadOrNew(dir, id string) (*Manifest, error) {
	manifest, err := load(dir)
	if errors.Is(err, os.ErrNotExist) {
		return &Manifest{Schema: Schema, ID: id}, nil
	}
	return manifest, err
}

func load(dir string) (*Manifest, error) {
	raw, err := os.ReadFile(filepath.Join(dir, ManifestName))
	if err != nil {
		return nil, err
	}
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, fmt.Errorf("manifest.json: %w", err)
	}
	return &manifest, nil
}

func digest(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
