package bridge

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBillingTokenHash(t *testing.T) {
	h := billingTokenHash([]byte(`{"accessToken":"abc123def456ghijklmnopqrstuvwxyz0123456789"}`))
	if len(h) != 64 {
		t.Fatalf("token hash len = %d, want 64 (sha256 hex)", len(h))
	}
	if billingTokenHash([]byte(`{"other":"x"}`)) != "present-but-no-token-field" {
		t.Fatal("no accessToken → sentinel")
	}
}

func TestBillingSnapshot(t *testing.T) {
	home := t.TempDir()
	mkfile(t, filepath.Join(home, ".claude", ".credentials.json"), `{"accessToken":"tok-secret-aaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`)
	fixed := time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC)
	eng := NewEngine(Deps{LookupEnv: mapLookup(map[string]string{"HOME": home, "ANTHROPIC_API_KEY": "k", "ANTHROPIC_BASE_URL": "http://x"}), Now: func() time.Time { return fixed }})

	dir := t.TempDir()
	p, err := eng.BillingSnapshot(dir, "pre")
	if err != nil {
		t.Fatalf("snapshot err: %v", err)
	}
	b, _ := os.ReadFile(p)
	if strings.Contains(string(b), "tok-secret") {
		t.Fatal("snapshot must NOT store the raw token")
	}
	if !strings.Contains(string(b), `"anthropic_api_key_in_env": "yes"`) {
		t.Fatalf("snapshot should record the api-key canary; got %s", b)
	}

	// missing args
	if _, err := eng.BillingSnapshot("", "x"); err == nil {
		t.Fatal("missing dir should error")
	}
	if _, err := eng.BillingSnapshot(dir, ""); err == nil {
		t.Fatal("missing label should error")
	}
	// mkdir error: dir under a regular file
	blocker := writeJSON(t, filepath.Join(t.TempDir(), "blk"), "x")
	if _, err := eng.BillingSnapshot(filepath.Join(blocker, "sub"), "x"); err == nil {
		t.Fatal("mkdir error should propagate")
	}
	// write error: the deterministic output path is pre-created as a directory
	wdir := t.TempDir()
	if err := os.Mkdir(filepath.Join(wdir, "snap-pre-"+itoa(fixed.Unix())+".json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := eng.BillingSnapshot(wdir, "pre"); err == nil {
		t.Fatal("write error (path is a dir) should propagate")
	}
	// marshal error via the shared seam
	orig := marshalIndent
	marshalIndent = func(any, string, string) ([]byte, error) { return nil, errors.New("boom") }
	defer func() { marshalIndent = orig }()
	if _, err := eng.BillingSnapshot(t.TempDir(), "pre"); err == nil {
		t.Fatal("marshal error should propagate")
	}
}

func TestBillingCompare(t *testing.T) {
	mk := func(body string) string {
		p := filepath.Join(t.TempDir(), "s.json")
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	cases := []struct {
		name, before, after string
		wantCode            int
	}{
		{"fail-apikey", `{}`, `{"anthropic_api_key_in_env":"yes"}`, 1},
		{"fail-baseurl", `{}`, `{"anthropic_base_url_in_env":"http://x"}`, 1},
		{"pass-rotated", `{"cred_token_hash":"h1"}`, `{"cred_token_hash":"h2"}`, 0},
		{"pass-present", `{"cred_token_hash":"h1"}`, `{"cred_token_hash":"h1"}`, 0},
		{"inconclusive-absent", `{"cred_token_hash":"absent"}`, `{"cred_token_hash":"absent"}`, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, code := BillingCompare(mk(tc.before), mk(tc.after))
			if code != tc.wantCode {
				t.Fatalf("compare = %d, want %d", code, tc.wantCode)
			}
		})
	}
	// read error → INCONCLUSIVE
	if _, code := BillingCompare("/no/before.json", "/no/after.json"); code != 2 {
		t.Fatalf("missing files → code %d, want 2", code)
	}
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
