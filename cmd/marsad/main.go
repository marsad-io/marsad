// Command marsad is the vendor-neutral observability MCP gateway.
package main

import (
	"fmt"
	"io"
	"os"
)

// version is set at build time via -ldflags "-X main.version=v0.x.y".
var version = "dev"

func run(args []string, out io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(out, usage)
		return 2
	}
	switch args[0] {
	case "version":
		fmt.Fprintf(out, "marsad %s\n", version)
		return 0
	default:
		fmt.Fprintf(out, "unknown command %q\n\n%s\n", args[0], usage)
		return 2
	}
}

const usage = `usage: marsad <command>

commands:
  version   print the marsad version`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}
