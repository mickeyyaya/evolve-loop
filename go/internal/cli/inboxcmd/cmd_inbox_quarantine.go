package inboxcmd

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

func runInboxQuarantine(root string, args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, inboxUsage("quarantine"))
		return 10
	}
	switch args[0] {
	case "list":
		return runQuarantineList(filepath.Join(root, ".evolve", "inbox", "quarantine"), args[1:], stdout, stderr)
	case "release":
		return runQuarantineRelease(root, args, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "inbox quarantine: unknown subcommand %q (want list|release)\n", args[0])
		return 10
	}
}

func runQuarantineList(qDir string, args []string, stdout, stderr io.Writer) int {
	asJSON := false
	for _, a := range args {
		if a != "--json" {
			fmt.Fprintf(stderr, "inbox quarantine list: unknown arg %q\n", a)
			return 10
		}
		asJSON = true
	}
	scan, err := inboxbatch.ScanDir(qDir)
	if err != nil {
		fmt.Fprintf(stderr, "inbox quarantine list: %v\n", err)
		return 1
	}
	printLoadWarnings(stderr, "inbox quarantine list", scan.Warnings)
	items := scan.Items
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", " ")
		if err := enc.Encode(items); err != nil {
			fmt.Fprintf(stderr, "inbox quarantine list: encode: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "%d quarantined item(s)\n", len(items))
	for _, it := range items {
		fmt.Fprintf(stdout, "  %s\t%s\n", it.ID, it.Title)
	}
	return 0
}

func runQuarantineRelease(root string, args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 || args[1] == "" {
		fmt.Fprintln(stderr, inboxUsage("quarantine"))
		return 10
	}
	id := args[1]
	res, err := inboxmover.ReleaseFromQuarantine(inboxmover.Options{ProjectRoot: root, Stderr: stderr}, id)
	if err != nil {
		fmt.Fprintf(stderr, "inbox quarantine release: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "released %s → %s\n", id, res.DestPath)
	return 0
}
