package lifecycle

import (
	"errors"
	"fmt"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

var errNoClassOrder = errors.New("file: no priority class order is wired; the filing verb passes inbox_priority.class_order")

func WithPriorityClasses(order []string) Option {
	return func(m *Mover) {
		if len(order) > 0 {
			m.priorityClasses = slices.Clone(order)
		}
	}
}

func (m *Mover) checkPriorityClass(item inboxbatch.Item) error {
	if len(m.priorityClasses) == 0 {
		return errNoClassOrder
	}
	if err := inboxbatch.CheckPriorityClass(item.PriorityClass, m.priorityClasses); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidItem, err)
	}
	return nil
}
