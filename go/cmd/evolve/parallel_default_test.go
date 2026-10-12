package main

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/paralleltest"
)

var processGlobalSetters = map[string]bool{
	"SetModelCatalogDirFn": true, "RegisterCode": true, "UseProcessEnv": true, "SetHangClassifier": true,
}

func TestParallelDefault_EveryTopLevelTestIsParallelOrListedSerial(t *testing.T) {
	t.Parallel()
	sources, err := paralleltest.Sources(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range paralleltest.Violations(sources, paralleltest.Config{Serial: serialTests, GlobalSetters: processGlobalSetters}) {
		t.Error(v)
	}
}
