// amp-cli is a standalone experiment; it is not registered in cmd/kepub.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cli := flag.String("cli", "amp", "Amp CLI executable (or a fake for offline tests)")
	bindingPath := flag.String("bindings", "", "trusted host bindings JSON for explicit continuation")
	timeout := flag.Duration("timeout", time.Minute, "task deadline")
	flag.Parse()
	if flag.NArg() != 0 || *timeout <= 0 {
		fmt.Fprintln(os.Stderr, "invalid arguments")
		os.Exit(2)
	}
	var bindings struct {
		SchemaVersion int       `json:"schemaVersion"`
		Threads       []Binding `json:"threads"`
	}
	if *bindingPath != "" {
		data, err := os.ReadFile(*bindingPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "cannot read trusted binding")
			os.Exit(2)
		}
		if err := json.Unmarshal(data, &bindings); err != nil || bindings.SchemaVersion != 1 {
			fmt.Fprintln(os.Stderr, "invalid trusted binding")
			os.Exit(2)
		}
	}
	// A closed consumer pipe must return EPIPE so run can clean up the group,
	// not terminate the Go host immediately while its separate group survives.
	signal.Ignore(syscall.SIGPIPE)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	os.Exit(run(ctx, config{cli: *cli, bindings: bindings.Threads, timeout: *timeout, grace: 200 * time.Millisecond, cleanupTimeout: 2 * time.Second}, os.Stdin, os.Stdout, os.Stderr))
}
