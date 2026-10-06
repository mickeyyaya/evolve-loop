package inboxbatch

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

var ErrUnknownPriorityClass = errors.New("unknown priority_class")

func PriorityClassPosition(class string, order []string) int {
	return slices.Index(order, class)
}

func CheckPriorityClass(class string, order []string) error {
	if PriorityClassPosition(class, order) >= 0 {
		return nil
	}
	return fmt.Errorf("%w %q: want one of %s", ErrUnknownPriorityClass, class, strings.Join(order, ", "))
}
