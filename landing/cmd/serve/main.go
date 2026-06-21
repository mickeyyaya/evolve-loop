// Command serve is a tiny static file server for previewing the built site.
// Usage: go run ./cmd/serve [dir] [addr]   (defaults: dist 127.0.0.1:8077)
package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	dir := "dist"
	addr := "127.0.0.1:8077"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	if len(os.Args) > 2 {
		addr = os.Args[2]
	}
	log.Printf("serving %s on http://%s", dir, addr)
	log.Fatal(http.ListenAndServe(addr, http.FileServer(http.Dir(dir))))
}
