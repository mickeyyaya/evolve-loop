package ship

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepoContractGate_AFlakeClearedRunSaysSoInItsSummaryLine(t *testing.T) {
	t.Parallel()
	repo, goDir := importerFixture(t)
	mustWrite(t, filepath.Join(goDir, "internal", "base", "base.go"), "package base\n\nconst Stop = \"new-stop\"\n")
	mustWrite(t, filepath.Join(goDir, "internal", "user", "user_test.go"), "package user\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestUserContract(t *testing.T) {\n\tif _, err := os.Stat(\"first-run.marker\"); err != nil {\n\t\t_ = os.WriteFile(\"first-run.marker\", nil, 0o644)\n\t\tt.Fatal(\"red on the first run only\")\n\t}\n}\n")
	ws := t.TempDir()
	var out strings.Builder
	var heard []string

	if err := runRepoContractGateAt(context.Background(), "enforce", repo, "HEAD", ws, &out, func(names []string) { heard = names }); err != nil {
		t.Fatalf("a red that is green alone must not be the lane's RED: %v\n%s", err, out.String())
	}

	if strings.Join(heard, ",") != "example.com/lane/internal/user.TestUserContract" {
		t.Fatalf("the Signal Center listener still hears the cleared names with a scan log: %v", heard)
	}
	want := "[ship] repo-contract gate: green with SHIP_BACKSTOP_FLAKE for example.com/lane/internal/user.TestUserContract; full output: " + filepath.Join(ws, scanLogName) + "\n"
	if out.String() != want {
		t.Fatalf("the loop log must say the ship proceeded on flake evidence:\n got %q\nwant %q", out.String(), want)
	}
	scan, err := os.ReadFile(filepath.Join(ws, scanLogName))
	if err != nil || !strings.Contains(string(scan), "SHIP_BACKSTOP_FLAKE") {
		t.Fatalf("the scan log keeps the flake WARN note: %v\n%s", err, scan)
	}
}

func TestRepoContractGate_AnImporterRedThatIsGreenAloneShipsAsFlakeEvidence(t *testing.T) {
	t.Parallel()
	repo, goDir := importerFixture(t)
	mustWrite(t, filepath.Join(goDir, "internal", "base", "base.go"), "package base\n\nconst Stop = \"new-stop\"\n")
	mustWrite(t, filepath.Join(goDir, "internal", "user", "user_test.go"), "package user\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestUserContract(t *testing.T) {\n\tif _, err := os.Stat(\"first-run.marker\"); err != nil {\n\t\t_ = os.WriteFile(\"first-run.marker\", nil, 0o644)\n\t\tt.Fatal(\"red on the first run only\")\n\t}\n}\n")
	var out strings.Builder
	var cleared []string
	if err := runRepoContractGateAt(context.Background(), "enforce", repo, "HEAD", "", &out, func(names []string) { cleared = names }); err != nil {
		t.Fatalf("a red that is green alone must not be the lane's RED: %v\n%s", err, out.String())
	}
	if strings.Join(cleared, ",") != "example.com/lane/internal/user.TestUserContract" {
		t.Fatalf("the Signal Center listener hears the cleared names through the production gate: %v", cleared)
	}
	if !strings.Contains(out.String(), "SHIP_BACKSTOP_FLAKE") || !strings.Contains(out.String(), "example.com/lane/internal/user.TestUserContract green when their packages ran by themselves") {
		t.Fatalf("the flake evidence names the test:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "alone re-run: go test -json -count=1 -timeout "+repoContractTestTimeout+" example.com/lane/internal/user\n") {
		t.Fatalf("the alone re-run runs the one package by itself:\n%s", out.String())
	}
}
