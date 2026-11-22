package gc

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type tempCheck struct {
	dir       string
	now       func() time.Time
	held      func(context.Context) func(string) (bool, error)
	removeAll func(string) error
}

func (tempCheck) Name() string { return "temp" }

func (t tempCheck) tempDir() string {
	if t.dir != "" {
		return t.dir
	}
	return os.TempDir()
}

func (t tempCheck) nowFunc() func() time.Time {
	if t.now != nil {
		return t.now
	}
	return time.Now
}

func (t tempCheck) holderProbe(ctx context.Context) func(string) (bool, error) {
	if t.held != nil {
		return t.held(ctx)
	}
	return LiveHolderProbe(ctx, sysexec.DefaultRunner)
}

func (t tempCheck) Scan(ctx context.Context, cc CheckContext) ([]CheckItem, error) {
	if cc.Policy.TempTTLHours <= 0 {
		return []CheckItem{}, nil
	}
	cutoff := t.nowFunc()().Add(-time.Duration(cc.Policy.TempTTLHours) * time.Hour)
	classified, err := classifyTempDir(t.tempDir(), cutoff, tempConfig{held: t.holderProbe(ctx)})
	if err != nil {
		return nil, err
	}
	items := make([]CheckItem, 0, len(classified))
	for _, it := range classified {
		items = append(items, CheckItem{
			Subject:  it.path,
			Action:   string(it.action),
			Rule:     it.rule,
			Evidence: it.evidence,
		})
	}
	return items, nil
}

func (t tempCheck) Apply(ctx context.Context, cc CheckContext) error {
	if cc.Policy.TempTTLHours <= 0 {
		return nil
	}
	removeAll := t.removeAll
	if removeAll == nil {
		removeAll = os.RemoveAll
	}
	cutoff := t.nowFunc()().Add(-time.Duration(cc.Policy.TempTTLHours) * time.Hour)
	rep := ReapPipelineTemp(t.tempDir(), cutoff, removeAll, SparingHeld(t.holderProbe(ctx)))
	if len(rep.Errors) > 0 {
		return errors.New(strings.Join(rep.Errors, "; "))
	}
	return nil
}
