//go:build acs

package cycle2230_test

import (
	"os"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/tmuxtest"
)

func TestMain(m *testing.M) {
	os.Exit(tmuxtest.Main(m))
}
