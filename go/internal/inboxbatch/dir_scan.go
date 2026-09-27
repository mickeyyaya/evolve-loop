package inboxbatch

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type LoadWarning struct {
	Text       string
	Unreadable bool
}

type DirScan struct {
	Items    []Item
	Warnings []LoadWarning
}

func (s DirScan) HasUnreadable() bool {
	for _, w := range s.Warnings {
		if w.Unreadable {
			return true
		}
	}
	return false
}

func ScanDir(dir string) (DirScan, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return DirScan{}, nil
	}
	if err != nil {
		return DirScan{}, fmt.Errorf("inboxbatch: read dir: %w", err)
	}
	var scan DirScan
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			scan = scan.withFile(dir, e.Name())
		}
	}
	sort.Slice(scan.Items, func(i, j int) bool { return scan.Items[i].ID < scan.Items[j].ID })
	scan.Warnings = append(scan.Warnings, duplicateIDWarnings(scan.Items)...)
	return scan, nil
}

func (s DirScan) withFile(dir, name string) DirScan {
	it, notices, err := LoadFile(filepath.Join(dir, name))
	if err != nil {
		s.Warnings = append(s.Warnings, LoadWarning{Text: name + ": " + err.Error(), Unreadable: true})
		return s
	}
	for _, n := range notices {
		s.Warnings = append(s.Warnings, LoadWarning{Text: n})
	}
	s.Items = append(s.Items, it)
	return s
}

func duplicateIDWarnings(sorted []Item) []LoadWarning {
	var out []LoadWarning
	for i := 1; i < len(sorted); i++ {
		if sorted[i].ID == sorted[i-1].ID {
			out = append(out, LoadWarning{Text: sorted[i].Path + ": duplicate id " + sorted[i].ID + " (also " + sorted[i-1].Path + ") — dep/connects references resolve ambiguously"})
		}
	}
	return out
}
