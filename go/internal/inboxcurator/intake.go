package inboxcurator

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

var ErrIntake = errors.New("inbox intake refused the item")

func CheckIntake(it inboxbatch.Item, kinds []string) error {
	if err := inboxbatch.CheckOrigin(it.Origin); err != nil {
		return fmt.Errorf("%w: %w", ErrIntake, err)
	}
	if !slices.Contains(kinds, it.Kind) {
		return fmt.Errorf("%w: kind %q is not one of the policy's inbox.curator.kinds: %s", ErrIntake, it.Kind, strings.Join(kinds, ", "))
	}
	return checkSurface(it)
}

func checkSurface(it inboxbatch.Item) error {
	for _, f := range it.Files {
		if strings.HasPrefix(strings.TrimSpace(f), "/") {
			return fmt.Errorf("%w: files entry %q is an absolute path; name a path relative to the repository", ErrIntake, f)
		}
	}
	paths := it.DeclaredPaths()
	if len(paths) == 0 {
		return fmt.Errorf("%w: files declares no repository path, so the item has no fix surface", ErrIntake)
	}
	for _, p := range paths {
		if slices.Contains(strings.Split(p, "/"), "..") {
			return fmt.Errorf("%w: files entry %q leaves the repository", ErrIntake, p)
		}
	}
	return nil
}

func UnderivedConsoleRoute(it inboxbatch.Item, isProtected func(string) bool) bool {
	if !inboxbatch.IsConsoleRoute(it.Route) {
		return false
	}
	it.Route = ""
	derived, _ := inboxbatch.ConsoleRouted(it, isProtected)
	return !derived
}

var ErrUnexplainedRoute = errors.New("no protected fix surface and no pipeline kind explain the console route; give the reason (--reason), or drop the route so a lane can take the item")

type Rules struct {
	Kinds       []string
	Thresholds  Thresholds
	IsProtected func(string) bool
}

type Override struct {
	Reason       string
	AllowOverlap bool
}

type Record struct {
	Reason       string   `json:"reason"`
	Overlaps     []string `json:"overlaps,omitempty"`
	ConsoleRoute bool     `json:"console_route,omitempty"`
}

type Admission struct {
	Warned []Overlap
	Record *Record
}

type OverlapError struct {
	Refused   []Overlap
	Threshold float64
}

func (e *OverlapError) Error() string {
	return fmt.Sprintf("%v: the item overlaps open item(s) %s at or above similarity %.2f", ErrIntake, strings.Join(e.IDs(), ", "), e.Threshold)
}

func (e *OverlapError) Unwrap() error { return ErrIntake }

func (e *OverlapError) IDs() []string {
	ids := make([]string, len(e.Refused))
	for i, o := range e.Refused {
		ids[i] = o.ID
	}
	return ids
}

func Admit(it inboxbatch.Item, open []inboxbatch.Item, r Rules, o Override) (Admission, error) {
	if err := CheckIntake(it, r.Kinds); err != nil {
		return Admission{}, err
	}
	record := Record{Reason: o.Reason}
	if UnderivedConsoleRoute(it, r.IsProtected) {
		if o.Reason == "" {
			return Admission{}, fmt.Errorf("%w: route %q: %w", ErrIntake, it.Route, ErrUnexplainedRoute)
		}
		record.ConsoleRoute = true
	}
	v := JudgeOverlap(it, open, r.Thresholds)
	adm := Admission{Warned: v.Warned}
	if len(v.Refused) > 0 {
		refusal := &OverlapError{Refused: v.Refused, Threshold: r.Thresholds.Refuse}
		if !o.AllowOverlap || o.Reason == "" {
			return adm, refusal
		}
		record.Overlaps = refusal.IDs()
	}
	if record.ConsoleRoute || len(record.Overlaps) > 0 {
		adm.Record = &record
	}
	return adm, nil
}
