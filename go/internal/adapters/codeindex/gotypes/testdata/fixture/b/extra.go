package b

import "errors"

type Resetter interface {
	Reset()
}

type Box[T any] struct{ v T }

func (b Box[T]) Get() T { return b.v }

type Pair[K comparable, V any] struct{}

func (p *Pair[K, V]) Swap() {}

func First[T any](xs []T) T { return xs[0] }

func Both[K comparable, V any](K, V) {}

func Misc(r Resetter) error {
	r.Reset()
	_ = First[int]([]int{int(1)})
	Both[int, string](1, "x")
	_ = len("x")
	func() {}()
	_ = (Describe)(nil)
	_ = Box[int]{}.Get()
	return errors.New("x")
}
