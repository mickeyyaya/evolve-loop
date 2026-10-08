package main

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

func runInboxClaims(args []string, stdout, stderr io.Writer) int {
	root, args, rootOK := takeProjectRoot(args)
	flags, operands, ok := splitInboxFlags(args, "--json")
	if !ok || !rootOK || len(operands) != 0 {
		fmt.Fprintln(stderr, inboxUsage("claims"))
		return 10
	}
	survey, err := inboxmover.SurveyClaims(inboxmover.Options{ProjectRoot: root, Stderr: stderr})
	if err != nil {
		fmt.Fprintf(stderr, "inbox claims: %v\n", err)
		return 2
	}
	if flags["--json"] {
		return encodeInboxJSON("claims", survey, stdout, stderr)
	}
	for _, c := range survey.Claims {
		fmt.Fprintf(stdout, "%s  cycle-%d  %s  %s%s\n", c.ID, c.Holder.Cycle, c.Holder.Verdict, c.Holder.Reason, duplicateNote(c.Duplicate))
	}
	stale := 0
	for _, dir := range survey.EmptyDirs {
		if !dir.Holder.Keeps() {
			stale++
		}
	}
	fmt.Fprintf(stdout, "%d claim(s); %d empty claim dir(s), %d of them stale (evolve gc removes those)\n", len(survey.Claims), len(survey.EmptyDirs), stale)
	return 0
}

func duplicateNote(duplicate bool) string {
	if duplicate {
		return " (the inbox root also holds this id)"
	}
	return ""
}

func runInboxRelease(args []string, stdout, stderr io.Writer) int {
	root, args, rootOK := takeProjectRoot(args)
	flags, operands, ok := splitInboxFlags(args, "--json", "--stale")
	ok = ok && rootOK
	if flags["--stale"] {
		ok = ok && len(operands) == 1
	} else {
		ok = ok && len(operands) == 2 && strings.TrimSpace(operands[0]) != ""
	}
	if !ok || strings.TrimSpace(operands[len(operands)-1]) == "" {
		fmt.Fprintln(stderr, inboxUsage("release"))
		return 10
	}
	opts := inboxmover.Options{ProjectRoot: root, Stderr: stderr}
	if flags["--stale"] {
		return releaseStaleClaims(opts, operands[0], flags["--json"], stdout, stderr)
	}
	res, err := inboxmover.ReleaseClaim(opts, operands[0], operands[1])
	if rc := releaseExitCode(err, stderr); rc != 0 {
		return rc
	}
	if flags["--json"] {
		return encodeInboxJSON("release", res, stdout, stderr)
	}
	printClaimRelease(stdout, res)
	return 0
}

func printClaimRelease(stdout io.Writer, res inboxmover.ClaimRelease) {
	switch res.Outcome {
	case inboxmover.ClaimNotHeld:
		fmt.Fprintf(stdout, "inbox release: %s is not claimed; nothing to release\n", res.ID)
	case inboxmover.ClaimDuplicateRemoved:
		fmt.Fprintf(stdout, "inbox release: %s released from cycle-%d; the root copy stays and the duplicate claim copy is removed\n", res.ID, res.Cycle)
	default:
		fmt.Fprintf(stdout, "inbox release: %s released from cycle-%d to the inbox root\n", res.ID, res.Cycle)
	}
}

func releaseStaleClaims(opts inboxmover.Options, reason string, asJSON bool, stdout, stderr io.Writer) int {
	released, err := inboxmover.ReleaseStaleClaims(opts, reason)
	rc := 0
	if err != nil {
		fmt.Fprintf(stderr, "inbox release: a stale claim stays held: %v\n", err)
		rc = 1
	}
	if asJSON {
		return max(rc, encodeInboxJSON("release", released, stdout, stderr))
	}
	for _, r := range released {
		fmt.Fprintf(stdout, "inbox release: %s released from cycle-%d (%s): %s\n", r.ID, r.Cycle, r.Outcome, r.Holder.Reason)
	}
	fmt.Fprintf(stdout, "inbox release: %d stale claim(s) released\n", len(released))
	return rc
}

func releaseExitCode(err error, stderr io.Writer) int {
	if errors.Is(err, inboxmover.ErrClaimHeld) || errors.Is(err, inboxmover.ErrClaimConflict) {
		fmt.Fprintf(stderr, "inbox release: %v\n", err)
		return 1
	}
	return curationExitCode("release", err, stderr)
}

func takeProjectRoot(args []string) (string, []string, bool) {
	i := slices.Index(args, "--project-root")
	if i < 0 {
		return envOrCwd("EVOLVE_PROJECT_ROOT"), args, true
	}
	if i+1 >= len(args) {
		return "", nil, false
	}
	return args[i+1], slices.Concat(args[:i], args[i+2:]), true
}

func splitInboxFlags(args []string, known ...string) (map[string]bool, []string, bool) {
	flags := map[string]bool{}
	var operands []string
	for _, a := range args {
		switch {
		case slices.Contains(known, a):
			flags[a] = true
		case strings.HasPrefix(a, "-"):
			return nil, nil, false
		default:
			operands = append(operands, a)
		}
	}
	return flags, operands, true
}
