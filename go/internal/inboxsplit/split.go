package inboxsplit

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

var (
	ErrNoFiles       = errors.New("inboxsplit: the item declares no files")
	ErrAllProtected  = errors.New("inboxsplit: every declared file is protected, so the item stays on the console")
	ErrNoneProtected = errors.New("inboxsplit: no declared file is protected, so the item needs a route, not a split")
	ErrStamped       = errors.New("inboxsplit: the item carries a stamp that a lifecycle verb wrote")
	ErrMalformed     = errors.New("inboxsplit: the item is not well formed")

	ErrFixNamesProtected = errors.New("inboxsplit: the item text names a protected path, so the lane part would carry work it must not do")
	ErrNoCriterion       = errors.New("inboxsplit: a part would carry no acceptance criterion, so no grader could judge it")
)

const (
	consoleSuffix = "-console"
	laneSuffix    = "-lane"
	splitFromKey  = "split_from"
	testSuffix    = "_test.go"
)

var ratchetRegistries = []string{".cover-strict", ".apicover-enforce"}

type Item = map[string]json.RawMessage

type Proposal struct {
	OriginalID string
	ConsoleID  string
	LaneID     string
	Console    Item
	Lane       Item
	LaneWaits  bool
}

var routeStamps = []string{
	inboxbatch.RoutedReasonField, inboxbatch.RoutedCycleField, inboxbatch.RoutedAtField,
}

var routeKeys = append([]string{inboxbatch.RouteField}, routeStamps...)

var textFields = []string{"title", "fix", "summary", "description", "source", "connects_to"}

var laneFields = []string{
	"kind", "class", "priority", "priority_class", "weight", "campaign", "source", "created_at",
	"summary", "fix", "description", "connects_to", "injected_by", "deliverable_kind",
}

var pathToken = regexp.MustCompile(`[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)+/?`)

func Propose(item Item, isProtected func(path string) bool, laneAfterConsole bool) (Proposal, error) {
	id, files, deps, err := readSplittable(item)
	if err != nil {
		return Proposal{}, err
	}
	criteria, err := readCriteria(item)
	if err != nil {
		return Proposal{}, err
	}
	protected, free := partition(files, isProtected)
	switch {
	case len(files) == 0:
		return Proposal{}, ErrNoFiles
	case len(free) == 0:
		return Proposal{}, ErrAllProtected
	case len(protected) == 0:
		return Proposal{}, ErrNoneProtected
	}
	if key, stamped := firstStamp(item); stamped {
		return Proposal{}, fmt.Errorf("%w: %s", ErrStamped, key)
	}
	consoleCriteria, laneCriteria, err := newSurface(isProtected, protected, free).divide(item, criteria)
	if err != nil {
		return Proposal{}, err
	}
	consoleID, laneID := id+consoleSuffix, id+laneSuffix
	laneWaits := laneAfterConsole || verifiesOnly(free)
	laneDeps := slices.Clone(deps)
	if laneWaits {
		laneDeps = append(laneDeps, consoleID)
	}
	return Proposal{
		OriginalID: id, ConsoleID: consoleID, LaneID: laneID, LaneWaits: laneWaits,
		Console: consolePart(item, consoleID, id, protected, consoleCriteria),
		Lane:    lanePart(item, laneID, id, free, laneDeps, laneCriteria),
	}, nil
}

func Proposals(items []Item, admissible, window int, isProtected func(path string) bool) []Proposal {
	if window <= 0 || admissible >= 2*window {
		return nil
	}
	var out []Proposal
	for _, item := range items {
		if p, err := Propose(item, isProtected, false); err == nil {
			out = append(out, p)
		}
	}
	return out
}

func readSplittable(item Item) (id string, files, deps []string, err error) {
	for key, dst := range map[string]any{"id": &id, "files": &files, "deps": &deps} {
		if raw, present := item[key]; present {
			if jerr := json.Unmarshal(raw, dst); jerr != nil {
				return "", nil, nil, fmt.Errorf("%w: %s: %v", ErrMalformed, key, jerr)
			}
		}
	}
	for _, key := range []string{"title", inboxbatch.RouteField} {
		var s string
		if raw, present := item[key]; present && json.Unmarshal(raw, &s) != nil {
			return "", nil, nil, fmt.Errorf("%w: %s must be text", ErrMalformed, key)
		}
	}
	if strings.TrimSpace(id) == "" {
		return "", nil, nil, fmt.Errorf("%w: id is missing", ErrMalformed)
	}
	return id, files, deps, nil
}

func readCriteria(item Item) ([]string, error) {
	var criteria []string
	if raw, present := item["acceptance"]; present {
		if err := json.Unmarshal(raw, &criteria); err != nil {
			return nil, fmt.Errorf("%w: acceptance must be a list of text: %v", ErrMalformed, err)
		}
	}
	return criteria, nil
}

func partition(files []string, isProtected func(string) bool) (protected, free []string) {
	for _, f := range files {
		if isProtected(f) || slices.ContainsFunc(declaredPaths(f), isProtected) {
			protected = append(protected, f)
		} else {
			free = append(free, f)
		}
	}
	return protected, free
}

func declaredPaths(entry string) []string {
	return inboxbatch.Item{Files: []string{entry}}.DeclaredPaths()
}

func verifiesOnly(entries []string) bool {
	return !slices.ContainsFunc(entries, func(entry string) bool {
		paths := declaredPaths(entry)
		return len(paths) == 0 || slices.ContainsFunc(paths, func(p string) bool { return !isVerification(p) })
	})
}

func isVerification(p string) bool {
	base := path.Base(p)
	return strings.HasSuffix(base, testSuffix) || slices.Contains(ratchetRegistries, base)
}

func textsOf(raw json.RawMessage) []string {
	var one string
	if json.Unmarshal(raw, &one) == nil {
		return []string{one}
	}
	var many []string
	_ = json.Unmarshal(raw, &many)
	return many
}

func firstStamp(item Item) (string, bool) {
	for _, key := range slices.Sorted(maps.Keys(item)) {
		if inboxbatch.RoleOf(key).IsStamp() && !slices.Contains(routeKeys, key) {
			return key, true
		}
	}
	return "", false
}

func basePart(item Item, id, from string, files []string) Item {
	part := maps.Clone(item)
	for _, key := range routeStamps {
		delete(part, key)
	}
	part["id"] = encode(id)
	part["files"] = encode(files)
	part[splitFromKey] = encode(from)
	return part
}

func consolePart(item Item, id, from string, protected, criteria []string) Item {
	part := basePart(item, id, from, protected)
	part["title"] = encode(titleOf(item, from) + " (console part)")
	part["acceptance"] = encode(criteria)
	if !routesToConsole(item) {
		delete(part, inboxbatch.RouteField)
	}
	return part
}

func lanePart(item Item, id, from string, free, deps, criteria []string) Item {
	part := Item{}
	for _, key := range laneFields {
		if raw, present := item[key]; present {
			part[key] = raw
		}
	}
	part["id"] = encode(id)
	part["files"] = encode(free)
	part[splitFromKey] = encode(from)
	part["title"] = encode(titleOf(item, from) + " (lane part)")
	part["acceptance"] = encode(criteria)
	if len(deps) > 0 {
		part["deps"] = encode(deps)
	}
	return part
}

func routesToConsole(item Item) bool {
	var route string
	_ = json.Unmarshal(item[inboxbatch.RouteField], &route)
	return inboxbatch.IsConsoleRoute(route)
}

func titleOf(item Item, fallback string) string {
	var title string
	_ = json.Unmarshal(item["title"], &title)
	if strings.TrimSpace(title) == "" {
		return fallback
	}
	return title
}

func encode(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
