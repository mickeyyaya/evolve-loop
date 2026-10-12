package bridge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDoctorAuthBackfillF1_OllamaCloudCredentialsAreFoundByEnvThenKeyFile(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		env          map[string]string
		keyFile      *string
		wantSource   string
		wantOptional bool
	}{
		{name: "api key in env wins", env: map[string]string{"OLLAMA_API_KEY": "k"}, keyFile: strPtrF1("key"), wantSource: "env:OLLAMA_API_KEY"},
		{name: "empty env value falls through to the signin key", env: map[string]string{"OLLAMA_API_KEY": ""}, keyFile: strPtrF1("key"), wantSource: "file:~/.ollama/id_ed25519"},
		{name: "signin key alone", keyFile: strPtrF1("key"), wantSource: "file:~/.ollama/id_ed25519"},
		{name: "empty signin key is no credential", keyFile: strPtrF1(""), wantOptional: true},
		{name: "nothing configured means local-only and optional", wantOptional: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			home := t.TempDir()
			if tc.keyFile != nil {
				mkfile(t, filepath.Join(home, ".ollama", "id_ed25519"), *tc.keyFile)
			}
			env := map[string]string{"HOME": home}
			for k, v := range tc.env {
				env[k] = v
			}
			a := doctorEngine(env, nil, 0, nil).doctorAuth("ollama-tmux")
			if tc.wantOptional {
				if a.Configured || !a.AuthOptional || a.Hint == "" {
					t.Fatalf("auth = %+v, want unconfigured, optional, with a hint", a)
				}
				return
			}
			if !a.Configured || a.Source != tc.wantSource || a.SubscriptionType != "ollama-cloud" || a.AuthOptional {
				t.Fatalf("auth = %+v, want configured from %s as ollama-cloud", a, tc.wantSource)
			}
		})
	}
}

func TestDoctorAuthBackfillF1_AgyIsConfiguredByAnyOfItsThreeDirectories(t *testing.T) {
	t.Parallel()
	for _, rel := range []string{
		filepath.Join(".config", "agy"),
		".agy",
		filepath.Join("Library", "Application Support", "Antigravity"),
	} {
		home := t.TempDir()
		if err := os.MkdirAll(filepath.Join(home, rel), 0o755); err != nil {
			t.Fatal(err)
		}
		a := doctorEngine(map[string]string{"HOME": home}, nil, 0, nil).doctorAuth("agy-tmux")
		if !a.Configured || a.Source != "file:"+filepath.Join(home, rel) || a.SubscriptionType != "google-ai" {
			t.Errorf("%s: auth = %+v", rel, a)
		}
	}
}

func strPtrF1(s string) *string { return &s }
