package shiptest_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/paralleltest"
)

var serialTests = map[string]string{}

func TestEveryTopLevelTestIsParallelOrListedSerial(t *testing.T) {
	t.Parallel()
	sources, err := paralleltest.Sources(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range paralleltest.Violations(sources, paralleltest.Config{Serial: serialTests}) {
		t.Error(v)
	}
}
