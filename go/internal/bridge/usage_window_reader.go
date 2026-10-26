package bridge

import (
	"fmt"
	"reflect"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

func UsageWindows(family, pane string, now time.Time) []quotastate.UsageWindow {
	r, err := Resolve(DefaultDriverSet(), family)
	if err != nil {
		return nil
	}
	spec := r.Manifest.usageWindowSpec()
	if spec == nil {
		return nil
	}
	return quotastate.ReadWindows(*spec, pane, now)
}

func (m Manifest) usageWindowSpec() *quotastate.WindowSpec {
	if usage, ok := m.Control("usage"); ok {
		return usage.Windows
	}
	return nil
}

func validateUsageWindows(cli string, m Manifest) error {
	spec := m.usageWindowSpec()
	if spec == nil {
		return nil
	}
	if err := spec.Validate(); err != nil {
		return fmt.Errorf("bridge:manifest: controls.usage.windows.%w (cli=%s)", err, cli)
	}
	return nil
}

func UsageWindowsDeclared(family string) bool {
	r, err := Resolve(DefaultDriverSet(), family)
	return err == nil && r.Manifest.usageWindowSpec() != nil
}

func windowsSettled(spec quotastate.WindowSpec, now time.Time) paneSettled {
	var prev []quotastate.UsageWindow
	return func(pane, _ string) bool {
		read := quotastate.ReadWindows(spec, pane, now)
		done := len(read) > 0 && reflect.DeepEqual(read, prev)
		prev = read
		return done
	}
}

func validateControlAwaits(cli string, m Manifest) error {
	for event, spec := range m.Controls {
		if spec.Await == controlAwaitWindows && spec.Windows == nil {
			return fmt.Errorf("bridge:manifest: controls.%s.await is %q, but the control declares no windows (cli=%s)", event, controlAwaitWindows, cli)
		}
	}
	return nil
}
