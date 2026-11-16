package ship

import (
	"context"
	"io"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
)

const gateStepDir = "go"

func GateStepRunner(run CmdRunner) StepRunner {
	return func(ctx context.Context, tree string, steps []baseattr.Key) ([]baseattr.Key, error) {
		var red []baseattr.Key
		for _, step := range steps {
			row, known := ciparity.GateStepOfKey(step.Step)
			if !known {
				red = append(red, step)
				continue
			}
			words := strings.Fields(row.Gate)
			code, err := run(ctx, words[0], filepath.Join(tree, gateStepDir), words[1:], nil, nil, io.Discard, io.Discard)
			if err != nil {
				return nil, err
			}
			if code != 0 {
				red = append(red, step)
			}
		}
		return red, nil
	}
}

func provable(k baseattr.Key) bool {
	if !k.IsStep() {
		return isKey(k)
	}
	_, known := ciparity.GateStepOfKey(k.Step)
	return known
}
