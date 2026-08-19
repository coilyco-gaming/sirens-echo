// Read-only MCP over this lane's own Temporal namespace, for a roster entry to
// point at. See deploy#698.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"forgejo.coilysiren.me/coilyco-gaming/sirens-echo/internal/temporalmcp"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "sirens-echo-temporal-mcp:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("sirens-echo-temporal-mcp", flag.ContinueOnError)
	addr := flags.String("http", ":8080", "listen `address`")
	if err := flags.Parse(args); err != nil {
		return err
	}
	cfg := temporalmcp.Config{
		HostPort:  os.Getenv("SIRENS_ECHO_TEMPORAL_HOST"),
		Namespace: os.Getenv("SIRENS_ECHO_TEMPORAL_NAMESPACE"),
		APIKey:    os.Getenv("SIRENS_ECHO_TEMPORAL_API_KEY"),
		Instance:  os.Getenv("SIRENS_ECHO_INSTANCE"),
	}
	temporal, err := temporalmcp.Dial(cfg)
	if err != nil {
		return err
	}
	defer temporal.Close()

	mux := http.NewServeMux()
	mux.Handle(temporalmcp.Path, temporalmcp.Handler(temporal, cfg))
	// The wrapper in front of this gates its own readiness on /healthz, so a
	// dial that succeeded is the whole health claim.
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	server := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return server.ListenAndServe()
}
