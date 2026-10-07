package bridge

import (
	"fmt"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

func UsageWindows(family, pane string, now time.Time) []quotastate.UsageWindow {
	m, err := LoadManifest(family + "-tmux")
	if err != nil {
		return nil
	}
	spec := m.usageWindowSpec()
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
