package lifecycle

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

var ErrInvalidItem = errors.New("inboxmover: invalid inbox item")

type FileResult struct {
	Path          string
	ConsoleReason string
}

var kebabItemID = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

var requiredTextFields = []string{"id", "title", "kind", "summary", "fix"}

var lifecycleOwnedFields = []string{
	"consumed", "failure_count", "last_failure_reason", "continuation", "released_continuations", "unbacked", "git_sha",
}

var lifecycleOwnedPrefixes = []string{"routed_", "retired_"}

const RouteField = "route"

func IsMoverWritten(key string) bool {
	return key == RouteField || isLifecycleOwned(key)
}

func isLifecycleOwned(key string) bool {
	hasOwnedPrefix := func(prefix string) bool { return strings.HasPrefix(key, prefix) }
	return slices.Contains(lifecycleOwnedFields, key) || slices.ContainsFunc(lifecycleOwnedPrefixes, hasOwnedPrefix)
}

func (m *Mover) File(raw []byte) (FileResult, error) {
	fields, item, err := decodeNewItem(raw)
	if err != nil {
		return FileResult{}, err
	}
	filed, err := filedItemIDs(m.inboxDir)
	if err != nil {
		return FileResult{}, fmt.Errorf("file: scan %s: %w", m.inboxDir, err)
	}
	if err := checkNewIdentity(item, filed); err != nil {
		return FileResult{}, err
	}
	now := m.now().UTC()
	if _, authored := fields["created_at"]; !authored {
		fields["created_at"] = jsonString(now.Format(time.RFC3339))
	}
	path := filepath.Join(m.inboxDir, now.Format(inboxbatch.FilenameStampLayout)+"-"+item.ID+".json")
	if err := publishNewItem(path, fields); err != nil {
		return FileResult{}, err
	}
	m.linef("filed %s: %s", item.ID, filepath.Base(path))
	m.ledgerLine(ledgerEntry{Action: "file", TaskID: item.ID, To: ".evolve/inbox/" + filepath.Base(path), Reason: "inbox add"})
	return FileResult{Path: path, ConsoleReason: consoleRoutedReason(path, m.isProtected)}, nil
}

func decodeNewItem(raw []byte) (map[string]json.RawMessage, inboxbatch.Item, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, inboxbatch.Item{}, fmt.Errorf("%w: the item must be one JSON object", ErrInvalidItem)
	}
	var item inboxbatch.Item
	if err := json.Unmarshal(raw, &item); err != nil {
		return nil, inboxbatch.Item{}, fmt.Errorf("%w: %v", ErrInvalidItem, err)
	}
	if err := checkNewFields(fields, item); err != nil {
		return nil, inboxbatch.Item{}, err
	}
	return fields, item, nil
}

func checkNewFields(fields map[string]json.RawMessage, item inboxbatch.Item) error {
	isBlank := func(s string) bool { return strings.TrimSpace(s) == "" }
	if err := checkAuthoredFields(fields, item); err != nil {
		return err
	}
	for _, key := range requiredTextFields {
		var text string
		if json.Unmarshal(fields[key], &text) != nil || isBlank(text) {
			return fmt.Errorf("%w: %q must be a non-empty string", ErrInvalidItem, key)
		}
	}
	switch {
	case !kebabItemID.MatchString(item.ID):
		return fmt.Errorf("%w: id %q must be kebab-case", ErrInvalidItem, item.ID)
	case item.Weight <= 0 || item.Weight > 1:
		return fmt.Errorf("%w: weight must be in (0, 1], got %v", ErrInvalidItem, item.Weight)
	case len(item.Acceptance) == 0 || slices.ContainsFunc(item.Acceptance, isBlank):
		return fmt.Errorf("%w: acceptance must list at least one non-empty criterion", ErrInvalidItem)
	}
	if dirty := inboxbatch.SanitizedFields(item); len(dirty) > 0 {
		return fmt.Errorf("%w: %s holds control characters or exceeds the loader's length bound", ErrInvalidItem, strings.Join(dirty, ", "))
	}
	return nil
}

func checkAuthoredFields(fields map[string]json.RawMessage, item inboxbatch.Item) error {
	for _, key := range slices.Sorted(maps.Keys(fields)) {
		if isLifecycleOwned(key) {
			return fmt.Errorf("%w: %q is written by the lifecycle verbs, never at filing", ErrInvalidItem, key)
		}
	}
	if _, authored := fields[RouteField]; authored && !inboxbatch.IsConsoleRoute(item.Route) {
		return fmt.Errorf("%w: an authored route may only send the item to the console; route %q is `evolve inbox route-lane`'s", ErrInvalidItem, item.Route)
	}
	return nil
}

func filedItemIDs(inboxDir string) (map[string]bool, error) {
	filed := map[string]bool{}
	err := filepath.WalkDir(inboxDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".json") {
			return err
		}
		if id, ok := readID(path); ok && id != "" {
			filed[id] = true
		}
		return nil
	})
	return filed, err
}

func checkNewIdentity(item inboxbatch.Item, filed map[string]bool) error {
	if filed[item.ID] {
		return fmt.Errorf("%w: id %q is already filed in the inbox", ErrInvalidItem, item.ID)
	}
	for _, dep := range item.Deps {
		if !filed[dep] {
			return fmt.Errorf("%w: deps names no other inbox item: %q", ErrInvalidItem, dep)
		}
	}
	return nil
}

func publishNewItem(path string, fields map[string]json.RawMessage) error {
	var body bytes.Buffer
	enc := json.NewEncoder(&body)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(fields); err != nil {
		return fmt.Errorf("file: encode %s: %w", path, err)
	}
	tmp := fmt.Sprintf("%s.tmp.%d", path, os.Getpid())
	defer func() { _ = os.Remove(tmp) }()
	if err := os.WriteFile(tmp, body.Bytes(), 0o644); err != nil {
		return fmt.Errorf("file: write %s: %w", tmp, err)
	}
	if err := os.Link(tmp, path); err != nil {
		return fmt.Errorf("file: publish %s: %w", path, err)
	}
	return nil
}
