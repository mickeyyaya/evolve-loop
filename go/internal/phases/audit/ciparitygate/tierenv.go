package ciparitygate

import (
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
)

func TierEnv() []string { return ciparity.CIEnv(os.Environ()) }
