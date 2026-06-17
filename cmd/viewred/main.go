// Package main is the entry point for the ViewRed binary.
//
// ViewRed is a network detection & response tool with local AI.
package main

import (
	"fmt"
	"os"
)

// Version is the semantic version of this build, set via -ldflags at build time.
var Version = "0.1.0-dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("viewred %s\n", Version)
		return
	}
	fmt.Printf("ViewRed %s — bootstrap stub. Real implementation comes in plan 2.\n", Version)
}
