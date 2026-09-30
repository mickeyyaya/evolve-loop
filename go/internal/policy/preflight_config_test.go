package policy

import "testing"

func TestPreflightConfig_MinFreeGiB(t *testing.T) {
	def := Policy{}.PreflightConfig().MinFreeGiB
	if def <= 0 {
		t.Fatalf("default floor = %v, want a positive documented default", def)
	}
	cases := []struct {
		name string
		p    Policy
		want float64
	}{
		{"absent block takes the default", Policy{}, def},
		{"empty block takes the default", Policy{Preflight: &PreflightPolicy{}}, def},
		{"explicit floor honored", Policy{Preflight: &PreflightPolicy{MinFreeGiB: 5}}, 5},
		{"fractional floor honored", Policy{Preflight: &PreflightPolicy{MinFreeGiB: 0.25}}, 0.25},
		{"negative floor falls back to the default", Policy{Preflight: &PreflightPolicy{MinFreeGiB: -3}}, def},
	}
	for _, c := range cases {
		if got := c.p.PreflightConfig().MinFreeGiB; got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}
