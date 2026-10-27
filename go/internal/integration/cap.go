package integration

const (
	MinCap               = 1
	MaxCap               = 4
	DefaultCap           = 4
	DefaultMaxAgeMinutes = 45
)

func HalveCap(cap int) int {
	if cap > MaxCap {
		return MaxCap
	}
	if cap <= MinCap {
		return MinCap
	}
	return cap / 2
}

func GrowCap(cap int) int {
	if cap < MinCap {
		return MinCap
	}
	if cap >= MaxCap {
		return MaxCap
	}
	return cap + 1
}
