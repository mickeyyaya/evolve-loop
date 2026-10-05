package lifecycle

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

type EditOp string

const (
	EditSet    EditOp = "set"
	EditAdd    EditOp = "add"
	EditRemove EditOp = "remove"
)

type FieldEdit struct {
	Op    EditOp
	Field string
	Value string
}

func (m *Mover) Edit(target string, edits []FieldEdit) (string, error) {
	edits = trimmedEdits(edits)
	if err := checkEditRequest(edits); err != nil {
		return "", err
	}
	path, err := m.locateEditable(target)
	if err != nil {
		return "", err
	}
	filed, err := m.identityIndex(edits)
	if err != nil {
		return "", err
	}
	var id string
	if err := rewriteItemJSON(path, admitEvery, func(item map[string]json.RawMessage) error {
		consoleReason := m.consoleReason(item)
		var cerr error
		if id, cerr = curate(item, edits, filed); cerr == nil && consoleReason != "" && m.consoleReason(item) == "" {
			cerr = fmt.Errorf("%w: the edit would open %s to lanes (it is console-owned: %s); a lane route is `evolve inbox route-lane`'s", ErrConsoleRouted, cmp.Or(id, "the item"), consoleReason)
		}
		return cerr
	}); err != nil {
		return "", fmt.Errorf("edit %s: %w", filepath.Base(path), err)
	}
	summary := describeEdits(edits)
	m.linef("edited %s: %s", filepath.Base(path), summary)
	m.ledgerLine(ledgerEntry{Action: "edit", TaskID: cmp.Or(id, "unknown"), Reason: summary})
	return path, nil
}

func trimmedEdits(edits []FieldEdit) []FieldEdit {
	out := make([]FieldEdit, len(edits))
	for i, e := range edits {
		out[i] = FieldEdit{Op: e.Op, Field: strings.TrimSpace(e.Field), Value: strings.TrimSpace(e.Value)}
	}
	return out
}

func checkEditRequest(edits []FieldEdit) error {
	if len(edits) == 0 {
		return fmt.Errorf("%w: edit needs at least one field edit", ErrBadArgs)
	}
	for _, e := range edits {
		role := inboxbatch.RoleOf(e.Field)
		switch {
		case role.Owner != inboxbatch.CuratorOwned:
			return fmt.Errorf("%w: %q is %s-owned, not a curation field (%s); routes and stamps have their own verbs",
				ErrBadArgs, e.Field, role.Owner, strings.Join(inboxbatch.FieldsOwnedBy(inboxbatch.CuratorOwned), ", "))
		case e.Op == EditSet:
		case e.Op != EditAdd && e.Op != EditRemove:
			return fmt.Errorf("%w: unknown edit %q", ErrBadArgs, e.Op)
		case role.Shape != inboxbatch.ListValue:
			return fmt.Errorf("%w: %s %s: only a list field takes add or remove", ErrBadArgs, e.Op, e.Field)
		}
	}
	return nil
}

func (m *Mover) consoleReason(item map[string]json.RawMessage) string {
	raw, _ := json.Marshal(item)
	it, ok := routingItem(raw)
	if !ok {
		return ""
	}
	_, reason := inboxbatch.ConsoleRouted(it, m.isProtected)
	return reason
}

func (m *Mover) locateEditable(target string) (string, error) {
	if !strings.HasSuffix(target, ".json") {
		loc, err := m.locatePending("edit", target)
		return loc.Path, err
	}
	file, err := os.Stat(target)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return "", fmt.Errorf("%w: %s", ErrNotFound, target)
	case err != nil:
		return "", fmt.Errorf("edit: %w", err)
	}
	dir, derr := os.Stat(filepath.Dir(target))
	root, rerr := os.Stat(m.inboxDir)
	if derr != nil || rerr != nil || file.IsDir() || !os.SameFile(dir, root) {
		return "", fmt.Errorf("%w: %s is not an item in the inbox root %s", ErrNotFound, target, m.inboxDir)
	}
	return target, nil
}

func (m *Mover) identityIndex(edits []FieldEdit) (map[string]bool, error) {
	touchesIdentity := func(e FieldEdit) bool { return e.Field == "id" || e.Field == "deps" }
	if !slices.ContainsFunc(edits, touchesIdentity) {
		return nil, nil
	}
	filed, err := filedItemIDs(m.inboxDir)
	if err != nil {
		return nil, fmt.Errorf("edit: scan %s: %w", m.inboxDir, err)
	}
	return filed, nil
}

func curate(item map[string]json.RawMessage, edits []FieldEdit, filed map[string]bool) (string, error) {
	filedID := carriedID(item)
	for _, e := range edits {
		if e.Field == "id" && filedID != "" {
			return "", fmt.Errorf("%w: ids are never renamed; this item is %q", ErrInvalidItem, filedID)
		}
		if err := applyEdit(item, e); err != nil {
			return "", err
		}
	}
	return judgeCurated(item, edits, filed)
}

func carriedID(item map[string]json.RawMessage) string {
	var id string
	if err := json.Unmarshal(item["id"], &id); err != nil {
		return string(item["id"])
	}
	return strings.TrimSpace(id)
}

func applyEdit(item map[string]json.RawMessage, e FieldEdit) error {
	switch inboxbatch.RoleOf(e.Field).Shape {
	case inboxbatch.NumberValue:
		w, err := strconv.ParseFloat(e.Value, 64)
		if err != nil || math.IsNaN(w) || math.IsInf(w, 0) {
			return fmt.Errorf("%w: weight must be a number, got %q", ErrInvalidItem, e.Value)
		}
		item[e.Field] = json.RawMessage(strconv.FormatFloat(w, 'f', -1, 64))
	case inboxbatch.ListValue:
		return editList(item, e)
	default:
		item[e.Field] = jsonString(e.Value)
	}
	return nil
}

func editList(item map[string]json.RawMessage, e FieldEdit) error {
	var list []string
	if e.Op == EditSet {
		if err := json.Unmarshal([]byte(e.Value), &list); err != nil || list == nil {
			return fmt.Errorf("%w: set %s takes a JSON array of strings, got %q", ErrInvalidItem, e.Field, e.Value)
		}
		item[e.Field] = jsonStrings(list)
		return nil
	}
	if raw, ok := item[e.Field]; ok {
		if err := json.Unmarshal(raw, &list); err != nil {
			return fmt.Errorf("%w: %s is not a list of strings: %v", ErrInvalidItem, e.Field, err)
		}
	}
	listed := slices.Contains(list, e.Value)
	switch {
	case e.Value == "":
		return fmt.Errorf("%w: %s %s names a blank entry", ErrInvalidItem, e.Op, e.Field)
	case e.Op == EditAdd && listed:
		return fmt.Errorf("%w: %s already lists %q", ErrInvalidItem, e.Field, e.Value)
	case e.Op == EditAdd:
		list = append(list, e.Value)
	case !listed:
		return fmt.Errorf("%w: %s does not list %q", ErrInvalidItem, e.Field, e.Value)
	default:
		list = slices.DeleteFunc(list, func(entry string) bool { return entry == e.Value })
	}
	item[e.Field] = jsonStrings(list)
	return nil
}

func jsonStrings(list []string) json.RawMessage {
	b, _ := json.Marshal(append([]string{}, list...))
	return b
}

func judgeCurated(fields map[string]json.RawMessage, edits []FieldEdit, filed map[string]bool) (string, error) {
	raw, _ := json.Marshal(fields)
	var item inboxbatch.Item
	if err := json.Unmarshal(raw, &item); err != nil {
		return "", fmt.Errorf("%w: the edited item does not load: %v", ErrInvalidItem, err)
	}
	edited := map[string]bool{}
	for _, e := range edits {
		edited[e.Field] = true
	}
	if err := checkFieldRules(fields, item, func(field string) bool { return edited[field] }); err != nil {
		return "", err
	}
	return item.ID, checkCuratedIdentity(item, edited, filed)
}

func checkCuratedIdentity(item inboxbatch.Item, edited, filed map[string]bool) error {
	if edited["id"] && filed[item.ID] {
		return fmt.Errorf("%w: id %q is already filed in the inbox", ErrInvalidItem, item.ID)
	}
	if !edited["deps"] {
		return nil
	}
	for _, dep := range item.Deps {
		switch {
		case dep == item.ID:
			return fmt.Errorf("%w: deps names the item itself: %q", ErrInvalidItem, dep)
		case !filed[dep]:
			return fmt.Errorf("%w: deps names no other inbox item: %q", ErrInvalidItem, dep)
		}
	}
	return nil
}

func describeEdits(edits []FieldEdit) string {
	parts := make([]string, len(edits))
	for i, e := range edits {
		parts[i] = fmt.Sprintf("%s %s=%s", e.Op, e.Field, e.Value)
	}
	return strings.Join(parts, "; ")
}
