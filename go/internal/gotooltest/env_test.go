package gotooltest

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestEnv_MergesTheLimitsWithTheCallerEnvironment(t *testing.T) {
	cases := []struct {
		name      string
		base      []string
		wantFlags string
	}{
		{"an empty environment", nil, "GOFLAGS=-p=2"},
		{"flags of the caller kept", []string{"GOFLAGS=-race -tags=integration"}, "GOFLAGS=-race -tags=integration -p=2"},
		{"a duplicate -p= replaced", []string{"GOFLAGS=-p=16 -race"}, "GOFLAGS=-race -p=2"},
		{"a separate -p value replaced", []string{"GOFLAGS=-p 16 -count=1"}, "GOFLAGS=-count=1 -p=2"},
		{"a trailing -p kept as one flag", []string{"GOFLAGS=-p"}, "GOFLAGS=-p -p=2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Env(tc.base)

			if !slices.Contains(got, tc.wantFlags) {
				t.Errorf("Env(%v) = %v, want the entry %q", tc.base, got, tc.wantFlags)
			}
			if !slices.Contains(got, "GOMAXPROCS=2") {
				t.Errorf("Env(%v) = %v, want GOMAXPROCS=2", tc.base, got)
			}
		})
	}
}

func TestEnv_KeepsOtherEntriesAndOneEntryForEachLimit(t *testing.T) {
	base := []string{"GOCACHE=/host/cache", "GOMAXPROCS=64", "PATH=/bin", "GOFLAGS=-p=8"}

	got := Env(base)

	flags, procs := 0, 0
	for _, entry := range got {
		if strings.HasPrefix(entry, "GOFLAGS=") {
			flags++
		}
		if strings.HasPrefix(entry, "GOMAXPROCS=") {
			procs++
		}
	}
	if flags != 1 || procs != 1 {
		t.Errorf("Env = %v, want one GOFLAGS and one GOMAXPROCS entry", got)
	}
	if !reflect.DeepEqual(got[:2], []string{"GOCACHE=/host/cache", "PATH=/bin"}) {
		t.Errorf("Env = %v, want GOCACHE and PATH kept first and unchanged", got)
	}
}

func TestEnv_DoesNotChangeTheCallerSlice(t *testing.T) {
	base := []string{"GOFLAGS=-p=8", "A=1"}

	Env(base)

	if !reflect.DeepEqual(base, []string{"GOFLAGS=-p=8", "A=1"}) {
		t.Errorf("Env changed its argument: %v", base)
	}
}
