package policy

const defaultPreflightMinFreeGiB = 1.0

type PreflightPolicy struct {
	MinFreeGiB float64 `json:"min_free_gib"`
}

type PreflightConfig struct {
	MinFreeGiB float64
}

func (p Policy) PreflightConfig() PreflightConfig {
	c := PreflightConfig{MinFreeGiB: defaultPreflightMinFreeGiB}
	if p.Preflight != nil && p.Preflight.MinFreeGiB > 0 {
		c.MinFreeGiB = p.Preflight.MinFreeGiB
	}
	return c
}

func (c PreflightConfig) MinFreeBytes() uint64 {
	return uint64(c.MinFreeGiB * (1 << 30))
}
