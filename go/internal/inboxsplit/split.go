package inboxsplit

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
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
)

const (
	consoleSuffix = "-console"
	laneSuffix    = "-lane"
	splitFromKey  = "split_from"
)

type Item = map[string]json.RawMessage

type Proposal struct {
	OriginalID string
	ConsoleID  string
	LaneID     string
	Console    Item
	Lane       Item
}

var routeStamps = []string{
	inboxbatch.RoutedReasonField, inboxbatch.RoutedCycleField, inboxbatch.RoutedAtField,
}

var routeKeys = append([]string{inboxbatch.RouteField}, routeStamps...)

var textFields = []string{"title", "fix", "acceptance", "summary", "description", "source", "connects_to"}

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
	if path, named := protectedTextPath(item, isProtected); named {
		return Proposal{}, fmt.Errorf("%w: %s", ErrFixNamesProtected, path)
	}
	consoleID, laneID := id+consoleSuffix, id+laneSuffix
	laneDeps := slices.Clone(deps)
	if laneAfterConsole {
		laneDeps = append(laneDeps, consoleID)
	}
	return Proposal{
		OriginalID: id, ConsoleID: consoleID, LaneID: laneID,
		Console: consolePart(item, consoleID, id, protected),
		Lane:    lanePart(item, laneID, id, free, laneDeps),
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

func partition(files []string, isProtected func(string) bool) (protected, free []string) {
	for _, f := range files {
		if isProtected(f) {
			protected = append(protected, f)
		} else {
			free = append(free, f)
		}
	}
	return protected, free
}

func protectedTextPath(item Item, isProtected func(string) bool) (string, bool) {
	for _, field := range textFields {
		for _, text := range textsOf(item[field]) {
			for _, tok := range pathToken.FindAllString(text, -1) {
				if tok = strings.TrimRight(tok, "."); isProtected(tok) {
					return tok, true
				}
			}
		}
	}
	return "", false
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

func consolePart(item Item, id, from string, protected []string) Item {
	part := basePart(item, id, from, protected)
	part["title"] = encode(titleOf(item, from) + " (console part)")
	if !routesToConsole(item) {
		delete(part, inboxbatch.RouteField)
	}
	return part
}

func lanePart(item Item, id, from string, free, deps []string) Item {
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
	part["deps"] = encode(deps)
	part["acceptance"] = encode([]string{
		fmt.Sprintf("The change that %s describes is made in %s only.", from, strings.Join(free, ", ")),
	})
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
