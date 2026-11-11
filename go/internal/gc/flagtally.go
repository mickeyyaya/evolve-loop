package gc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxfile"
)

const (
	FlagTallyFile       = "gc-flag-tally.json"
	FlagRepeatThreshold = 2
	flagItemWeight      = 0.3
	flagIDHashLen       = 8
)

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func IsFlag(it WorktreeItem) bool {
	return it.Action == WorktreeActionFlagDirty || it.Action == WorktreeActionFlagUnmerged
}

func FlagKey(it WorktreeItem) string {
	return strings.Join([]string{string(it.Action), it.Branch, it.Path}, "|")
}

func CountFlags(evolveDir string, items []WorktreeItem) (map[string]int, error) {
	path := filepath.Join(evolveDir, FlagTallyFile)
	prior, err := readFlagTally(path)
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, it := range items {
		if IsFlag(it) {
			key := FlagKey(it)
			counts[key] = prior[key] + 1
		}
	}
	if err := atomicwrite.JSON(path, counts); err != nil {
		return nil, fmt.Errorf("gc: flag tally write: %w", err)
	}
	return counts, nil
}

func readFlagTally(path string) (map[string]int, error) {
	body, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]int{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("gc: flag tally read: %w", err)
	}
	var tally map[string]int
	if err := json.Unmarshal(body, &tally); err != nil {
		return nil, fmt.Errorf("gc: flag tally %s is not a count map: %w", path, err)
	}
	return tally, nil
}

func FileRepeatedFlags(inboxDir string, items []WorktreeItem, counts map[string]int, now func() time.Time) ([]string, error) {
	var filed []string
	for _, it := range items {
		if !IsFlag(it) || counts[FlagKey(it)] != FlagRepeatThreshold {
			continue
		}
		res, err := inboxfile.Mint(inboxDir, flagItemJSON(it, counts[FlagKey(it)]), now)
		if err != nil {
			return filed, fmt.Errorf("gc: file flag %s: %w", FlagKey(it), err)
		}
		if !res.Skipped {
			filed = append(filed, res.Path)
		}
	}
	return filed, nil
}

func flagItemID(it WorktreeItem) string {
	sum := sha256.Sum256([]byte(FlagKey(it)))
	name := it.Branch
	if name == "" {
		name = filepath.Base(it.Path)
	}
	slug := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(name), "-"), "-")
	return strings.Join([]string{"gc", string(it.Action), slug, hex.EncodeToString(sum[:])[:flagIDHashLen]}, "-")
}

func flagItemJSON(it WorktreeItem, count int) []byte {
	subject := it.Branch
	if subject == "" {
		subject = it.Path
	}
	raw, _ := json.Marshal(map[string]any{
		"id":             flagItemID(it),
		"title":          fmt.Sprintf("gc flag %s: %s", it.Action, subject),
		"kind":           "task",
		"priority_class": inboxbatch.ClassHygiene,
		"weight":         flagItemWeight,
		"summary":        fmt.Sprintf("evolve gc flagged %s (%s) on %d runs and keeps it: gc never removes a flagged item.", subject, it.Reason, count),
		"fix":            "Land the work through evolve ship, or retire the branch or tree on purpose. Then the flag leaves the gc report.",
		"acceptance":     []string{"The next evolve gc run no longer flags " + subject + "."},
	})
	return raw
}
