package testverdict

import (
	"os"
	"path/filepath"
	"strings"
)

func (s *Store) Stats() (Stats, error) {
	files, _, err := listStore(s.root)
	st := Stats{ByGate: map[string]int{}}
	for _, f := range files {
		st.Files++
		st.Bytes += f.size
		if filepath.Base(filepath.Dir(f.path)) == indexDir {
			continue
		}
		st.Entries++
		_, gate, _ := strings.Cut(strings.TrimSuffix(filepath.Base(f.path), entryFile), ".")
		st.ByGate[gate]++
	}
	return st, err
}

func (s *Store) Root() string { return s.root }

func (s *Store) Wipe() (Stats, error) {
	st, err := s.Stats()
	if err != nil {
		return Stats{}, err
	}
	return st, os.RemoveAll(s.root)
}
