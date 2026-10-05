package policy

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBootBinaryRefresh_Vocabulary(t *testing.T) {
	cases := []struct {
		name string
		p    Policy
		want string
	}{
		{"absent block defaults auto", Policy{}, "auto"},
		{"empty word defaults auto", Policy{Boot: &BootPolicy{}}, "auto"},
		{"off honored", Policy{Boot: &BootPolicy{BinaryRefresh: "off"}}, "off"},
		{"auto explicit", Policy{Boot: &BootPolicy{BinaryRefresh: "auto"}}, "auto"},
		{"unknown word fails safe to auto", Policy{Boot: &BootPolicy{BinaryRefresh: "shadwo"}}, "auto"},
	}
	for _, c := range cases {
		if got := c.p.BootBinaryRefresh(); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestBootDetachWait_DefaultsAndOverride(t *testing.T) {
	cases := []struct {
		name string
		p    Policy
		want time.Duration
	}{
		{"absent block defaults", Policy{}, DefaultBootDetachWait},
		{"zero defaults", Policy{Boot: &BootPolicy{DetachWaitS: 0}}, DefaultBootDetachWait},
		{"negative defaults", Policy{Boot: &BootPolicy{DetachWaitS: -5}}, DefaultBootDetachWait},
		{"seconds honored", Policy{Boot: &BootPolicy{DetachWaitS: 30}}, 30 * time.Second},
	}
	for _, c := range cases {
		if got := c.p.BootDetachWait(); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
	if DefaultBootDetachWait != 10*time.Minute {
		t.Errorf("DefaultBootDetachWait = %v, want 10m", DefaultBootDetachWait)
	}
}

func TestBootDetachWait_LoadsFromPolicyJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(`{"boot":{"detach_wait_s":7}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Boot == nil || p.Boot.DetachWaitS != 7 || p.BootDetachWait() != 7*time.Second {
		t.Errorf("boot.detach_wait_s=7 must load as 7s, got %+v", p.Boot)
	}
}
