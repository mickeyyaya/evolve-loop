package guardcmd

import (
	"flag"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/posteditvalidate"
)

func RunPostEditValidate(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve postedit-validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var bypass bool
	fs.BoolVar(&bypass, "bypass", false, "emergency: bypass validation")
	fs.Usage = func() {
		fmt.Fprintln(stdout, "Usage: evolve postedit-validate [--bypass] < <PostToolUse-payload-json>")
		fmt.Fprintln(stdout, "Validates the just-edited file by extension: .json/.sh/.py")
		fmt.Fprintln(stdout, "Always exits 0. WARN to stderr is surfaced to the LLM.")
	}
	if err := fs.Parse(args); err != nil {
		return 0
	}

	payload, _ := io.ReadAll(stdin)
	projectRoot := cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT")

	posteditvalidate.Run(posteditvalidate.Options{
		Payload:     payload,
		ProjectRoot: projectRoot,
		LLMStderr:   stderr,
		Bypass:      bypass,
	})
	return 0
}
