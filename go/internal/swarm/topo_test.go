package swarm

import (
	"reflect"
	"strings"
	"testing"
)

func w(id string, deps ...string) WorkerSpec { return WorkerSpec{WorkerID: id, DependsOn: deps} }

func TestTopoOrder(t *testing.T) {
	tests := []struct {
		name    string
		workers []WorkerSpec
		want    []string
		wantErr string
	}{
		{
			name:    "no deps sorts by worker_id (deterministic tie-break)",
			workers: []WorkerSpec{w("w2"), w("w0"), w("w1")},
			want:    []string{"w0", "w1", "w2"},
		},
		{
			name:    "linear chain",
			workers: []WorkerSpec{w("w2", "w1"), w("w1", "w0"), w("w0")},
			want:    []string{"w0", "w1", "w2"},
		},
		{
			name:    "diamond: dependents after the shared root, tie broken by id",
			workers: []WorkerSpec{w("w0"), w("w1", "w0"), w("w2", "w0"), w("w3", "w1", "w2")},
			want:    []string{"w0", "w1", "w2", "w3"},
		},
		{
			name:    "single worker",
			workers: []WorkerSpec{w("w0")},
			want:    []string{"w0"},
		},
		{
			name:    "cycle is rejected",
			workers: []WorkerSpec{w("w0", "w1"), w("w1", "w0")},
			wantErr: "cycle",
		},
		{
			name:    "self-dependency is rejected",
			workers: []WorkerSpec{w("w0", "w0")},
			wantErr: "depends_on itself",
		},
		{
			name:    "dangling dependency is rejected",
			workers: []WorkerSpec{w("w0", "ghost")},
			wantErr: "unknown worker",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := TopoOrder(tc.workers)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("order = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestTopoOrder_Deterministic guards the tie-break: the same input always yields
// the same order, regardless of input slice order.
func TestTopoOrder_Deterministic(t *testing.T) {
	a := []WorkerSpec{w("w0"), w("w1", "w0"), w("w2", "w0")}
	b := []WorkerSpec{w("w2", "w0"), w("w1", "w0"), w("w0")}
	oa, err := TopoOrder(a)
	if err != nil {
		t.Fatal(err)
	}
	ob, err := TopoOrder(b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(oa, ob) {
		t.Errorf("non-deterministic: %v vs %v", oa, ob)
	}
}
