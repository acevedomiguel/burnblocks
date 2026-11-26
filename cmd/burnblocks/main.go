// Command burnblocks reports Kimi Code and OpenRouter usage, credits, and
// estimated runway from the terminal.
//
// The command is read-only. It never sends inference requests, purchases
// credit, or creates, rotates, or revokes a credential.
//
// This scaffold wires version reporting so the release pipeline can inject and
// verify it. The provider subcommands are added with the shared engines.
package main

import (
	"fmt"
	"io"
	"os"
)

// version is replaced at build time with
// -ldflags "-X main.version=<version>".
var version = "dev"

const usage = `burnblocks reports Kimi Code and OpenRouter usage.

Usage:
  burnblocks --version   Print the version and exit.
  burnblocks --help      Print this help and exit.

Provider subcommands are not implemented yet.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	switch args[0] {
	case "--version", "-v", "version":
		fmt.Fprintf(stdout, "burnblocks version %s\n", version)
		return 0
	case "--help", "-h", "help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "burnblocks: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}
