package swarm

import "testing"

func TestSwarmResult_TotalCostUSD(t *testing.T) {
	cases := []struct {
		name    string
		workers []WorkerResult
		want    float64
	}{
		{"empty → 0", nil, 0},
		{"single worker", []WorkerResult{{CostUSD: 0.5}}, 0.5},
		{"multiple workers sum", []WorkerResult{{CostUSD: 0.25}, {CostUSD: 0.75}}, 1.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sr := SwarmResult{Workers: tc.workers}
			if got := sr.TotalCostUSD(); got != tc.want {
				t.Errorf("TotalCostUSD = %v, want %v", got, tc.want)
			}
		})
	}
}
