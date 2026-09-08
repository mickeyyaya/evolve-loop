package ship

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckEGPSGate_IncompleteEvidenceFailsClosed(t *testing.T) {
	for _, body := range []string{"", "{}", "null", "{broken", `{"red_count":0,"ship_eligible":false}`} {
		t.Run(body, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "acs-verdict.json")
			if body != "" {
				if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := checkEGPSGate(path, &RunResult{}); err == nil {
				t.Fatalf("incomplete predicate evidence %q must refuse ship", body)
			}
		})
	}
}
