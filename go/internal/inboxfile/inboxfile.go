// Package inboxfile validates, locks and publishes one new inbox item.
package inboxfile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/verifylock"
)

const (
	LockName = ".bump.lock"
	LockWait = 30 * time.Second
)

var (
	ErrInvalidItem  = errors.New("inboxmover: invalid inbox item")
	ErrAlreadyFiled = errors.New("already filed in the inbox")

	errNoClassOrder = errors.New("file: no priority class order is wired; the filing verb passes inbox_priority.class_order")
)

var kebabItemID = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

var requiredTextFields = []string{"id", "title", "kind", "summary", "fix"}

var mintedClasses = []string{
	inboxbatch.ClassCorrectness,
	inboxbatch.ClassStability,
	inboxbatch.ClassPerformance,
	inboxbatch.ClassDebuggability,
	inboxbatch.ClassFeature,
	inboxbatch.ClassMaintainability,
	inboxbatch.ClassHygiene,
	inboxbatch.ClassSecurity,
}

type Result struct {
	Path    string
	Skipped bool
}

func Mint(inboxDir string, raw []byte, now func() time.Time) (Result, error) {
	if now == nil {
		now = time.Now
	}
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("file: create %s: %w", inboxDir, err)
	}
	path, err := Publish(inboxDir, raw, mintedClasses, now().UTC())
	if errors.Is(err, ErrAlreadyFiled) {
		return Result{Skipped: true}, nil
	}
	if err != nil {
		return Result{}, err
	}
	return Result{Path: path}, nil
}

func Publish(inboxDir string, raw []byte, classes []string, now time.Time) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), LockWait)
	defer cancel()
	release, err := verifylock.AcquireAt(ctx, filepath.Join(inboxDir, LockName), io.Discard)
	if err != nil {
		return "", fmt.Errorf("file: lock %s: %w", inboxDir, err)
	}
	defer release()
	return File(inboxDir, raw, classes, now)
}

func File(inboxDir string, raw []byte, classes []string, now time.Time) (string, error) {
	fields, item, err := decodeNewItem(raw)
	if err != nil {
		return "", err
	}
	if err := checkPriorityClass(item, classes); err != nil {
		return "", err
	}
	pending, filed, err := scanIDs(inboxDir)
	if err != nil {
		return "", fmt.Errorf("file: scan %s: %w", inboxDir, err)
	}
	if err := checkNewIdentity(item, pending, filed); err != nil {
		return "", err
	}
	if _, authored := fields["created_at"]; !authored {
		fields["created_at"] = jsonString(now.Format(time.RFC3339))
	}
	path := filepath.Join(inboxDir, now.Format(inboxbatch.FilenameStampLayout)+"-"+item.ID+".json")
	if err := PublishNewItem(path, fields); err != nil {
		return "", err
	}
	return path, nil
}

func checkPriorityClass(item inboxbatch.Item, classes []string) error {
	if len(classes) == 0 {
		return errNoClassOrder
	}
	if err := inboxbatch.CheckPriorityClass(item.PriorityClass, classes); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidItem, err)
	}
	return nil
}

func jsonString(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}

func IsLifecycleOwned(key string) bool {
	return key == "status" || (key != inboxbatch.RouteField && inboxbatch.RoleOf(key).IsStamp())
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
	return CheckFieldRules(fields, item, func(string) bool { return true })
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

func CheckFieldRules(fields map[string]json.RawMessage, item inboxbatch.Item, judged func(field string) bool) error {
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
		if IsLifecycleOwned(key) {
			return fmt.Errorf("%w: %q is written by the lifecycle verbs, never at filing", ErrInvalidItem, key)
		}
	}
	if _, authored := fields[inboxbatch.RouteField]; authored && !inboxbatch.IsConsoleRoute(item.Route) {
		return fmt.Errorf("%w: an authored route may only send the item to the console; route %q is `evolve inbox route-lane`'s", ErrInvalidItem, item.Route)
	}
	return nil
}

func ReadID(path string) (string, bool) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	var doc struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", false
	}
	return inboxbatch.ItemID(doc.ID, path), true
}

func PendingIDs(inboxDir string) (map[string]bool, error) {
	pending, _, err := scanIDs(inboxDir)
	return pending, err
}

func FiledIDs(inboxDir string) (map[string]bool, error) {
	_, filed, err := scanIDs(inboxDir)
	return filed, err
}

func scanIDs(inboxDir string) (pending, filed map[string]bool, err error) {
	pending, filed = map[string]bool{}, map[string]bool{}
	consumed := filepath.Join(inboxDir, inboxbatch.ConsumedDirName) + string(filepath.Separator)
	err = filepath.WalkDir(inboxDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".json") {
			return err
		}
		if id, ok := ReadID(path); ok && id != "" {
			filed[id] = true
			if !strings.HasPrefix(path, consumed) {
				pending[id] = true
			}
		}
		return nil
	})
	return pending, filed, err
}

func checkNewIdentity(item inboxbatch.Item, pending, filed map[string]bool) error {
	if pending[item.ID] {
		return fmt.Errorf("%w: id %q is %w", ErrInvalidItem, item.ID, ErrAlreadyFiled)
	}
	for _, dep := range item.Deps {
		if !filed[dep] {
			return fmt.Errorf("%w: deps names no other inbox item: %q", ErrInvalidItem, dep)
		}
	}
	return nil
}

func PublishNewItem(path string, fields map[string]json.RawMessage) error {
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
