package inboxbatch

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

const (
	ClassCorrectness     = "correctness"
	ClassStability       = "stability"
	ClassPerformance     = "performance"
	ClassDebuggability   = "debuggability"
	ClassFeature         = "feature"
	ClassMaintainability = "maintainability"
	ClassHygiene         = "hygiene"
	ClassSecurity        = "security"
)

var (
	ErrUnknownPriorityClass = errors.New("unknown priority_class")
	ErrNoPriorityClass      = errors.New("no priority_class, so the item would rank below every class")
)

func RequirePriorityClass(class string) error {
	if strings.TrimSpace(class) == "" {
		return ErrNoPriorityClass
	}
	return nil
}

func PriorityClassPosition(class string, order []string) int {
	return slices.Index(order, class)
}

func CheckPriorityClass(class string, order []string) error {
	if PriorityClassPosition(class, order) >= 0 {
		return nil
	}
	return fmt.Errorf("%w %q: want one of %s", ErrUnknownPriorityClass, class, strings.Join(order, ", "))
}
