package inboxbatch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const consumedDirName = "consumed"

const ConsumedDirName = consumedDirName

const unknownConsumedField = "unknown"

type ConsumedRecord struct {
	ID        string
	At        string
	Via       string
	CreatedAt string
	File      string
}

func (r ConsumedRecord) Describe() string {
	return fmt.Sprintf("consumed at %s via %s, record %s", r.At, r.Via, r.File)
}

func CopyOf(records []ConsumedRecord, createdAt, file string) (ConsumedRecord, bool) {
	for _, rec := range records {
		if createdAt != "" && rec.CreatedAt != "" {
			if rec.CreatedAt == createdAt {
				return rec, true
			}
			continue
		}
		if rec.File == file {
			return rec, true
		}
	}
	return ConsumedRecord{}, false
}

func ConsumedRecords(inboxDir string) map[string][]ConsumedRecord {
	root := filepath.Join(inboxDir, consumedDirName)
	dirs := append([]string{root}, CycleDirs(root)...)
	var paths []string
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
				paths = append(paths, filepath.Join(dir, e.Name()))
			}
		}
	}
	sort.Strings(paths)
	out := map[string][]ConsumedRecord{}
	for _, p := range paths {
		rec, ok := readConsumedRecord(p)
		if !ok {
			continue
		}
		out[rec.ID] = append(out[rec.ID], rec)
	}
	return out
}

func readConsumedRecord(path string) (ConsumedRecord, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ConsumedRecord{}, false
	}
	var doc struct {
		ID        string `json:"id"`
		CreatedAt string `json:"created_at"`
		Consumed  struct {
			At  string `json:"at"`
			Via string `json:"via"`
		} `json:"consumed"`
	}
	if json.Unmarshal(raw, &doc) != nil {
		return ConsumedRecord{}, false
	}
	rec := ConsumedRecord{ID: ItemID(doc.ID, path), At: doc.Consumed.At, Via: doc.Consumed.Via, CreatedAt: doc.CreatedAt, File: filepath.Base(path)}
	if rec.At == "" {
		rec.At = unknownConsumedField
	}
	if rec.Via == "" {
		rec.Via = unknownConsumedField
	}
	return rec, true
}

func resurrectedWarning(file string, rec ConsumedRecord) LoadWarning {
	return LoadWarning{
		Text: fmt.Sprintf("%s: skipped resurrected item %s, a copy of a retired item (%s) — remove the file", file, rec.ID, rec.Describe()),
		file: file,
	}
}
