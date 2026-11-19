package lifecycle

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeItemFiledConsole signalcenter.Code = "INBOX_ITEM_FILED_CONSOLE"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleInbox, CodeItemFiledConsole, "an item was filed as console-owned (protected fix surface, console kind, or authored console route) and no lane claims it; fields.class = protected | kind | route, task_id, kind, path, reason, step=intake")
}

var ErrInvalidItem = errors.New("inboxmover: invalid inbox item")

var ErrAlreadyFiled = errors.New("already filed in the inbox")

type FileResult struct {
	Path          string
	ConsoleReason string
}

var kebabItemID = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

var requiredTextFields = []string{"id", "title", "kind", "summary", "fix"}

const RouteField = inboxbatch.RouteField

func IsMoverWritten(key string) bool {
	return inboxbatch.RoleOf(key).Owner == inboxbatch.LoopStampOwned
}

func rootStampCovers(key string, claim, root any) bool {
	if inboxbatch.RoleOf(key).IsCounter() {
		claimCount, claimOK := claim.(float64)
		rootCount, rootOK := root.(float64)
		return claimOK && rootOK && rootCount >= claimCount
	}
	return IsMoverWritten(key) || reflect.DeepEqual(claim, root)
}

func isLifecycleOwned(key string) bool {
	return key != RouteField && inboxbatch.RoleOf(key).IsStamp()
}

func (m *Mover) File(raw []byte) (FileResult, error) {
	fields, item, err := decodeNewItem(raw)
	if err != nil {
		return FileResult{}, err
	}
	if err := m.checkPriorityClass(item); err != nil {
		return FileResult{}, err
	}
	if err := m.stampIntake(fields, item); err != nil {
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
	reason := consoleRoutedReason(path, m.isProtected)
	m.emitFiledConsole(item, path, reason)
	return FileResult{Path: path, ConsoleReason: reason}, nil
}

func consoleReasonClass(reason string) string {
	switch {
	case strings.HasPrefix(reason, "protected fix surface:"), strings.HasPrefix(reason, "mentions protected path:"):
		return "protected"
	case strings.HasPrefix(reason, "kind:"):
		return "kind"
	default:
		return "route"
	}
}

func (m *Mover) emitFiledConsole(item inboxbatch.Item, path, reason string) {
	if reason == "" {
		return
	}
	c := m.center()
	if c == nil {
		return
	}
	class := consoleReasonClass(reason)
	c.Emit(signalcenter.Event{
		Module:   signalcenter.ModuleInbox,
		Origin:   "Mover.File",
		Kind:     signalcenter.KindInboxWarning,
		Code:     CodeItemFiledConsole,
		Severity: signalcenter.SeverityInfo,
		Reason:   fmt.Sprintf("filed console-owned %s (%s): %s", item.ID, class, reason),
		Fields: map[string]string{
			"task_id": item.ID,
			"kind":    item.Kind,
			"class":   class,
			"path":    path,
			"reason":  reason,
			"step":    "intake",
		},
	})
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
	if err := checkAuthoredFields(fields, item); err != nil {
		return err
	}
	return checkFieldRules(fields, item, func(string) bool { return true })
}

type fieldRule struct {
	field string
	check func(fields map[string]json.RawMessage, item inboxbatch.Item) error
}

var itemFieldRules = append(requiredTextRules(),
	fieldRule{"id", checkKebabID},
	fieldRule{"weight", checkWeight},
	fieldRule{"acceptance", checkAcceptance},
)

func requiredTextRules() []fieldRule {
	rules := make([]fieldRule, 0, len(requiredTextFields))
	for _, key := range requiredTextFields {
		rules = append(rules, fieldRule{key, func(fields map[string]json.RawMessage, _ inboxbatch.Item) error {
			var text string
			if json.Unmarshal(fields[key], &text) != nil || isBlank(text) {
				return fmt.Errorf("%w: %q must be a non-empty string", ErrInvalidItem, key)
			}
			return nil
		}})
	}
	return rules
}

func isBlank(s string) bool { return strings.TrimSpace(s) == "" }

func checkKebabID(_ map[string]json.RawMessage, item inboxbatch.Item) error {
	if !kebabItemID.MatchString(item.ID) {
		return fmt.Errorf("%w: id %q must be kebab-case", ErrInvalidItem, item.ID)
	}
	return nil
}

func checkWeight(_ map[string]json.RawMessage, item inboxbatch.Item) error {
	if item.Weight <= 0 || item.Weight > 1 {
		return fmt.Errorf("%w: weight must be in (0, 1], got %v", ErrInvalidItem, item.Weight)
	}
	return nil
}

func checkAcceptance(_ map[string]json.RawMessage, item inboxbatch.Item) error {
	if len(item.Acceptance) == 0 || slices.ContainsFunc(item.Acceptance, isBlank) {
		return fmt.Errorf("%w: acceptance must list at least one non-empty criterion", ErrInvalidItem)
	}
	return nil
}

func checkFieldRules(fields map[string]json.RawMessage, item inboxbatch.Item, judged func(field string) bool) error {
	for _, rule := range itemFieldRules {
		if !judged(rule.field) {
			continue
		}
		if err := rule.check(fields, item); err != nil {
			return err
		}
	}
	unjudged := func(field string) bool { return !judged(field) }
	if dirty := slices.DeleteFunc(inboxbatch.SanitizedFields(item), unjudged); len(dirty) > 0 {
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
		return fmt.Errorf("%w: id %q is %w", ErrInvalidItem, item.ID, ErrAlreadyFiled)
	}
	for _, dep := range item.Deps {
		if !filed[dep] {
			return fmt.Errorf("%w: deps names no other inbox item: %q", ErrInvalidItem, dep)
		}
	}
	return nil
}

func publishNewItem(path string, fields map[string]json.RawMessage) error {
	body, err := marshalFiled(fields)
	if err != nil {
		return fmt.Errorf("file: encode %s: %w", path, err)
	}
	tmp := fmt.Sprintf("%s.tmp.%d", path, os.Getpid())
	defer func() { _ = os.Remove(tmp) }()
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return fmt.Errorf("file: write %s: %w", tmp, err)
	}
	if err := os.Link(tmp, path); err != nil {
		return fmt.Errorf("file: publish %s: %w", path, err)
	}
	return nil
}
