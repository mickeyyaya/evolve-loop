package practicewatch

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
)

func commandSource(locator string) Source {
	return Source{ID: "agy-log", Family: "agy", Kind: KindCommand, Locator: locator, Enabled: true}
}

func installFakeAgy(t *testing.T, script string) string {
	t.Helper()
	bin := t.TempDir()
	fakeclitest.Install(t, filepath.Join(bin, "agy"), script)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return bin
}

func TestWatch_ACommandSourceRunsTheCLIAndFilesItsChangedOutput(t *testing.T) {
	root, data := t.TempDir(), t.TempDir()
	out := filepath.Join(data, "out")
	installFakeAgy(t, "#!/bin/sh\n[ \"$1\" = changelog ] || { echo \"bad args: $*\" >&2; exit 9; }\ncat "+out+"\n")
	write := func(text string) {
		if err := os.WriteFile(out, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("v1\nold line\n")
	opts := optionsFor(root, commandSource("agy changelog"))
	if res := only(t, Watch(context.Background(), opts)); res.Status != StatusRecorded {
		t.Fatalf("first run: %s (%s)", res.Status, res.Detail)
	}
	write("v2\nnew practice\nv1\nold line\n")

	res := only(t, Watch(context.Background(), opts))

	if res.Status != StatusFiled {
		t.Fatalf("status = %s (%s), want filed", res.Status, res.Detail)
	}
	raw, _ := os.ReadFile(itemFile(root, inboxItems(t, root)[0]))
	if !strings.Contains(string(raw), "new practice") || !strings.Contains(string(raw), "agy changelog") {
		t.Errorf("the item must carry the added line and name the command:\n%s", raw)
	}
}

func TestWatch_AMissingBinaryIsASkipWithAReasonNotAnError(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	opts := optionsFor(t.TempDir(), commandSource("agy changelog"))

	report := Watch(context.Background(), opts)

	res := only(t, report)
	if res.Status != StatusSkipped || !strings.Contains(res.Detail, "agy") || report.Errors() != 0 {
		t.Errorf("result = %+v, errors = %d, want skipped naming the binary", res, report.Errors())
	}
}

func TestWatch_ACommandThatExitsNonZeroIsAnError(t *testing.T) {
	installFakeAgy(t, "#!/bin/sh\necho boom >&2\nexit 3\n")

	res := only(t, Watch(context.Background(), optionsFor(t.TempDir(), commandSource("agy changelog"))))

	if res.Status != StatusError || !strings.Contains(res.Detail, "boom") || !strings.Contains(res.Detail, "3") {
		t.Errorf("result = %+v, want an error with the exit code and stderr", res)
	}
}

func TestWatch_ACommandThatRunsPastTheTimeoutIsAnError(t *testing.T) {
	installFakeAgy(t, "#!/bin/sh\nsleep 5\n")
	opts := optionsFor(t.TempDir(), commandSource("agy changelog"))
	opts.Timeout = 200 * time.Millisecond

	res := only(t, Watch(context.Background(), opts))

	if res.Status != StatusError {
		t.Errorf("result = %+v, want an error", res)
	}
}

func TestWatch_ARunnerFailureThatIsNotAMissingBinaryIsAnError(t *testing.T) {
	opts := optionsFor(t.TempDir(), commandSource("agy changelog"))
	opts.Run = func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		return -1, errors.New("runner broke")
	}

	res := only(t, Watch(context.Background(), opts))

	if res.Status != StatusError || !strings.Contains(res.Detail, "runner broke") {
		t.Errorf("result = %+v", res)
	}
}
