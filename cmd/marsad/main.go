// Command marsad is the vendor-neutral observability MCP gateway.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/marsad-io/marsad/internal/config"
	"github.com/marsad-io/marsad/internal/server"
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
	case "serve":
		if err := serve(args[1:], out); err != nil {
			fmt.Fprintln(out, err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(out, "unknown command %q\n\n%s\n", args[0], usage)
		return 2
	}
}

func serve(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(out)
	configPath := fs.String("config", "marsad.yaml", "path to marsad.yaml")
	transport := fs.String("transport", "stdio", "MCP transport: stdio or http")
	listen := fs.String("listen", ":8811", "listen address for --transport http")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *transport != "stdio" && *transport != "http" {
		return fmt.Errorf(`unknown transport %q: use "stdio" or "http"`, *transport)
	}

	cfg, err := config.Load(*configPath, os.Getenv)
	if err != nil {
		return err
	}
	s, err := server.New(cfg, version)
	if err != nil {
		return err
	}

	switch *transport {
	case "stdio":
		return s.RunStdio(context.Background())
	default:
		fmt.Fprintf(out, "marsad %s serving MCP over HTTP on %s\n", version, *listen)
		return http.ListenAndServe(*listen, s.HTTPHandler())
	}
}

const usage = `usage: marsad <command>

commands:
  serve     run the MCP gateway (--config marsad.yaml --transport stdio|http --listen :8811)
  version   print the marsad version`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}
