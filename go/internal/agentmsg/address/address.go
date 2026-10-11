package address

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type Selector string

const (
	SelectRole     Selector = "role"
	SelectPhase    Selector = "phase"
	SelectDispatch Selector = "dispatch"
	SelectAll      Selector = "*"
)

const MaxValueBytes = 256

const cyclePrefix = "cycle:"

var ErrSyntax = errors.New("address: syntax")

var (
	cyclePattern    = regexp.MustCompile(`^[1-9][0-9]*$`)
	namePattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
	dispatchPattern = regexp.MustCompile(`^[A-Za-z0-9._:/-]+$`)
)

var valuePatterns = map[Selector]*regexp.Regexp{
	SelectRole:     namePattern,
	SelectPhase:    namePattern,
	SelectDispatch: dispatchPattern,
}

type Address struct {
	Cycle    int
	Selector Selector
	Value    string
}

func Parse(s string) (Address, error) {
	cycle, selector, err := splitCycle(s)
	if err != nil {
		return Address{}, err
	}
	a, err := parseSelector(selector)
	if err != nil {
		return Address{}, err
	}
	a.Cycle = cycle
	return a, nil
}

func splitCycle(s string) (int, string, error) {
	rest, qualified := strings.CutPrefix(s, cyclePrefix)
	if !qualified {
		return 0, s, nil
	}
	digits, selector, found := strings.Cut(rest, "/")
	if !found || !cyclePattern.MatchString(digits) {
		return 0, "", fmt.Errorf("%w: %q: a cycle is a positive number with no lead zero, then /", ErrSyntax, s)
	}
	cycle, err := strconv.Atoi(digits)
	if err != nil {
		return 0, "", fmt.Errorf("%w: %q: %v", ErrSyntax, s, err)
	}
	return cycle, selector, nil
}

func parseSelector(s string) (Address, error) {
	if s == string(SelectAll) {
		return Address{Selector: SelectAll}, nil
	}
	kind, value, found := strings.Cut(s, ":")
	a := Address{Selector: Selector(kind), Value: value}
	if !found {
		return Address{}, fmt.Errorf("%w: %q: a selector is role:, phase:, dispatch: or *", ErrSyntax, s)
	}
	if err := a.checkValue(); err != nil {
		return Address{}, err
	}
	return a, nil
}

func (a Address) checkValue() error {
	if a.Selector == SelectAll && a.Value == "" {
		return nil
	}
	pattern, known := valuePatterns[a.Selector]
	if !known {
		return fmt.Errorf("%w: unknown selector %q", ErrSyntax, a.Selector)
	}
	if len(a.Value) > MaxValueBytes || !pattern.MatchString(a.Value) {
		return fmt.Errorf("%w: %s value %q", ErrSyntax, a.Selector, a.Value)
	}
	return nil
}

func (a Address) String() string {
	selector := string(SelectAll)
	if a.Selector != SelectAll {
		selector = string(a.Selector) + ":" + a.Value
	}
	if a.Relative() {
		return selector
	}
	return cyclePrefix + strconv.Itoa(a.Cycle) + "/" + selector
}

func (a Address) Relative() bool { return a.Cycle == 0 }

func (a Address) Resolve(senderCycle int) Address {
	if a.Relative() {
		a.Cycle = senderCycle
	}
	return a
}

func (a Address) MarshalText() ([]byte, error) {
	if err := a.checkValue(); err != nil {
		return nil, err
	}
	return []byte(a.String()), nil
}

func (a *Address) UnmarshalText(b []byte) error {
	parsed, err := Parse(string(b))
	if err != nil {
		return err
	}
	*a = parsed
	return nil
}
