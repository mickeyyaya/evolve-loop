//go:build !unix

package main

import "os"

func usageOf(*os.ProcessState, string) (usage, bool) {
	return usage{}, false
}
