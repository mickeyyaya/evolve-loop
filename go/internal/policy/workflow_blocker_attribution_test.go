package policy

import (
	"errors"
	"strconv"
	"testing"
)

func TestParse_BlockerAttributionRange(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"19", "96"} {
		_, err := Parse([]byte(`{"workflow": {"blocker_attribution": {"wait_share_pct": ` + v + `}}}`))
		if !errors.Is(err, ErrOutOfRange) {
			t.Errorf("wait_share_pct = %s: err = %v; want ErrOutOfRange", v, err)
		}
	}
	for _, v := range []string{"20", "95"} {
		p, err := Parse([]byte(`{"workflow": {"blocker_attribution": {"wait_share_pct": ` + v + `}}}`))
		if err != nil || v != strconv.Itoa(p.WorkflowConfig().BlockerAttribution.WaitSharePct) {
			t.Errorf("wait_share_pct = %s: config %+v, err %v; want it accepted as set", v, p.WorkflowConfig().BlockerAttribution, err)
		}
	}
	if _, err := Parse([]byte(`{"workflow": {"blocker_attribution": {"waitshare": 50}}}`)); !errors.Is(err, ErrUnknownKey) {
		t.Errorf("unknown key: err = %v; want ErrUnknownKey", err)
	}
}

func TestParse_BlockerAttributionDefaultsToFiftyPercent(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`{}`, `{"workflow": {}}`, `{"workflow": {"blocker_attribution": {}}}`} {
		p, err := Parse([]byte(body))
		if got := p.WorkflowConfig().BlockerAttribution; err != nil || got != (BlockerAttributionConfig{WaitSharePct: DefaultBlockerWaitSharePct}) || DefaultBlockerWaitSharePct != 50 {
			t.Errorf("%s: config = %+v, %v; want 50", body, got, err)
		}
	}
}

func TestBlockerAttributionPolicy_ReadsTheSetValueOverTheDefault(t *testing.T) {
	t.Parallel()
	set := 70
	p := Policy{Workflow: &WorkflowPolicy{BlockerAttribution: &BlockerAttributionPolicy{WaitSharePct: &set}}}
	if got := p.WorkflowConfig().BlockerAttribution.WaitSharePct; got != 70 {
		t.Fatalf("WaitSharePct = %d; want the set 70", got)
	}
}
