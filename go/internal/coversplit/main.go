package coversplit

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

func Main(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("coversplit", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dir := flags.String("dir", ".", "the Go module directory that holds "+File)
	merge := flags.String("merge", "", "write the merge of the named coverage profiles to this file, and plan no run")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *merge != "" {
		return mergeFiles(*merge, flags.Args(), stderr)
	}
	splits, err := Load(*dir)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	for _, run := range Plan(flags.Args(), splits) {
		fmt.Fprintln(stdout, strings.Join(run.Args(), " "))
	}
	return 0
}

func mergeFiles(out string, inputs []string, stderr io.Writer) int {
	var profiles [][]byte
	for _, in := range inputs {
		data, err := os.ReadFile(in)
		if err != nil {
			fmt.Fprintf(stderr, "coversplit: %v\n", err)
			return 1
		}
		profiles = append(profiles, data)
	}
	merged, err := Merge(profiles)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := os.WriteFile(out, merged, 0o644); err != nil {
		fmt.Fprintf(stderr, "coversplit: %v\n", err)
		return 1
	}
	return 0
}
