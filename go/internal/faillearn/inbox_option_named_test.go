package faillearn

import "testing"

func TestOption_AppliesInboxDestination(t *testing.T) {
	items := []InboxItem{{ID: "retro-1279-example", Title: "example remediation"}}

	var opt Option = WithInbox("/nonexistent/inbox", items)
	var cfg writeConfig
	opt(&cfg)

	if cfg.inboxDir != "/nonexistent/inbox" {
		t.Errorf("inboxDir = %q, want the destination the option carried", cfg.inboxDir)
	}
	if len(cfg.inboxItems) != len(items) || cfg.inboxItems[0].ID != items[0].ID {
		t.Errorf("inboxItems = %+v, want %+v", cfg.inboxItems, items)
	}
}
