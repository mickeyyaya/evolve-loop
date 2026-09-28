package ship

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestLandPrefixes(t *testing.T) {
	lanes := []fleet.LaneCandidate{
		{ID: "lane-good-a", Files: []string{"a.go"}},
		{ID: "lane-toxic", Files: []string{"b.go"}},
		{ID: "lane-good-c", Files: []string{"c.go"}},
	}
	verify := func(ids []string) bool {
		for _, id := range ids {
			if id == "lane-toxic" {
				return false
			}
		}
		return true
	}

	t.Run("prefix-queue mode ejects the culprit and lands survivors", func(t *testing.T) {
		landed, ejected := LandPrefixes(policy.FleetConfig{Landing: "prefix-queue"}, lanes, verify)
		if len(ejected) != 1 || ejected[0] != "lane-toxic" {
			t.Errorf("ejected=%v, want [lane-toxic]", ejected)
		}
		if len(landed) != 2 || !verify(landed) {
			t.Errorf("landed=%v, want both innocent lanes verifying as a set", landed)
		}
	})

	t.Run("per-lane mode stands or falls independently", func(t *testing.T) {
		landed, ejected := LandPrefixes(policy.FleetConfig{Landing: "per-lane"}, lanes, verify)
		if len(landed) != 2 || len(ejected) != 1 || ejected[0] != "lane-toxic" {
			t.Errorf("landed=%v ejected=%v, want 2 landed / [lane-toxic] ejected", landed, ejected)
		}
	})

	t.Run("empty lane set is nil/nil", func(t *testing.T) {
		landed, ejected := LandPrefixes(policy.FleetConfig{Landing: "prefix-queue"}, nil, verify)
		if landed != nil || ejected != nil {
			t.Errorf("empty set must yield nil,nil; got %v,%v", landed, ejected)
		}
	})
}
