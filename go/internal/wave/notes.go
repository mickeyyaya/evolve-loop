package wave

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	notesDir       = "notes"
	noteSuffix     = ".md"
	noteNameLayout = "20060102T150405.000000000Z"
)

var errBlankNote = errors.New("the note text is blank")

type Note struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

func (s Store) notesPath() string { return filepath.Join(s.dir, notesDir) }

func (s Store) AddNote(text string, now time.Time) (Note, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Note{}, errBlankNote
	}
	if err := os.MkdirAll(s.notesPath(), 0o755); err != nil {
		return Note{}, fmt.Errorf("make the notes dir: %w", err)
	}
	n := Note{Name: now.UTC().Format(noteNameLayout) + noteSuffix, Text: text}
	if err := createExclusive(filepath.Join(s.notesPath(), n.Name), []byte(text+"\n")); err != nil {
		return Note{}, fmt.Errorf("add note %s: %w", n.Name, err)
	}
	return n, nil
}

func createExclusive(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, werr := f.Write(data)
	return errors.Join(werr, f.Close())
}

func (s Store) Notes() ([]Note, error) {
	entries, err := os.ReadDir(s.notesPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	var out []Note
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), noteSuffix) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(s.notesPath(), e.Name()))
		if err != nil {
			return nil, fmt.Errorf("read note %s: %w", e.Name(), err)
		}
		out = append(out, Note{Name: e.Name(), Text: strings.TrimSpace(string(b))})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s Store) RemoveNotes(notes []Note) error {
	for _, n := range notes {
		err := os.Remove(filepath.Join(s.notesPath(), filepath.Base(n.Name)))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove note %s: %w", n.Name, err)
		}
	}
	return nil
}
