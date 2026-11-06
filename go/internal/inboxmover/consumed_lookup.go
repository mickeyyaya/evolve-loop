package inboxmover

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

func FindConsumedRecord(inboxDir, id string) (string, error) {
	var matches []string
	_ = filepath.WalkDir(filepath.Join(inboxDir, "consumed"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}
		if itemIDAt(path) == id {
			matches = append(matches, path)
		}
		return nil
	})
	switch len(matches) {
	case 0:
		return "", ErrNotFound
	case 1:
		return matches[0], nil
	}
	return "", fmt.Errorf("inboxmover: %d consumed records share id %q: %s", len(matches), id, strings.Join(matches, ", "))
}
