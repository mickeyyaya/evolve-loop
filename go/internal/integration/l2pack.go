package integration

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/acsrunner"
	"github.com/mickeyyaya/evolve-loop/go/internal/lanebudget"
	"github.com/mickeyyaya/evolve-loop/go/internal/repocontract"
)

type PackStep struct {
	Name string
	Run  PackageTest
}

type Checkout func(ctx context.Context, tip string) (dir string, release func(), err error)

type Command func(ctx context.Context, dir string, argv ...string) (out string, exit int, err error)

type packSpec struct {
	name    string
	argv    []string
	listing bool
}

func l2Specs() []packSpec {
	return []packSpec{
		{name: "repo-contract", argv: append([]string{"go", "test", "-count=1"}, repocontract.Packages()...)},
		{name: "cover-changed", argv: []string{"make", "test-integration", "cover-changed", "COVER_CHANGED_BASE=" + refMain}},
		{name: "cover-strict", argv: []string{"make", "test-integration", "cover-strict"}},
		{name: "build", argv: []string{"go", "build", "./..."}},
		{name: "vet", argv: []string{"go", "vet", "./..."}},
		{name: "gofmt", argv: []string{"gofmt", "-l", "."}, listing: true},
	}
}

func PackageTestArgv(pkg string) []string {
	return append([]string{"go"}, lanebudget.Args([]string{"test", "-count=1", pkg})...)
}

func L2Pack(checkout Checkout, run Command) []PackStep {
	specs := l2Specs()
	steps := make([]PackStep, len(specs))
	for i, spec := range specs {
		steps[i] = PackStep{Name: spec.name, Run: spec.test(checkout, run)}
	}
	return steps
}

func (s packSpec) test(checkout Checkout, run Command) PackageTest {
	return func(ctx context.Context, tip, _ string) (TestOutcome, error) {
		dir, release, err := checkout(ctx, tip)
		if err != nil {
			return TestOutcome{}, fmt.Errorf("integration: check out %s: %w", tip, err)
		}
		defer release()
		out, exit, err := run(ctx, filepath.Join(dir, "go"), s.argv...)
		if err != nil {
			return TestOutcome{}, acsrunner.HostFailure(err)
		}
		red := exit != 0 || (s.listing && strings.TrimSpace(out) != "")
		return TestOutcome{Green: !red, Output: out}, nil
	}
}
