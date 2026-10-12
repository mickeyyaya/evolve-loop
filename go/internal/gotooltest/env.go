package gotooltest

import (
	"strings"
)

const (
	nestedParallelism = "2"
	goflagsKey        = "GOFLAGS="
	gomaxprocsKey     = "GOMAXPROCS="
)

func Env(base []string) []string {
	goflags := ""
	out := make([]string, 0, len(base)+2)
	for _, entry := range base {
		switch {
		case strings.HasPrefix(entry, goflagsKey):
			goflags = strings.TrimPrefix(entry, goflagsKey)
		case strings.HasPrefix(entry, gomaxprocsKey):
		default:
			out = append(out, entry)
		}
	}
	return append(out, goflagsKey+limitedFlags(goflags), gomaxprocsKey+nestedParallelism)
}

func limitedFlags(goflags string) string {
	kept := make([]string, 0)
	fields := strings.Fields(goflags)
	for i := 0; i < len(fields); i++ {
		switch {
		case strings.HasPrefix(fields[i], "-p="):
		case fields[i] == "-p" && i+1 < len(fields):
			i++
		default:
			kept = append(kept, fields[i])
		}
	}
	return strings.Join(append(kept, "-p="+nestedParallelism), " ")
}
