// Offline validation of an access policy, for a caller that cannot import Go.
// Reads files and nothing else. See docs/sirens-echo-access-check.md.
package main

import (
	"fmt"
	"io"
	"os"

	"forgejo.coilysiren.me/coilyco-gaming/sirens-echo/internal/community"
)

// usage names the argument rather than describing the tool, because the caller
// is a CI step that already knows what it invoked.
const usage = "usage: sirens-echo-access-check <access-policy.yaml> [...]"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run returns the code rather than calling os.Exit, because the codes are what
// deploy's CI keys on and an untestable main cannot prove them.
func run(paths []string, stdout, stderr io.Writer) int {
	if len(paths) == 0 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	failed := false
	for _, path := range paths {
		if err := check(path); err != nil {
			// The path is repeated because a CI log shows one line, and which
			// file failed is the first thing the reader needs.
			fmt.Fprintf(stderr, "%s: %v\n", path, err)
			failed = true
			continue
		}
		fmt.Fprintf(stdout, "%s: ok\n", path)
	}
	if failed {
		return 1
	}
	return 0
}

// check runs the same loader the runtime does, so this cannot drift from what
// the pod will accept. A second implementation would be a worse gate than none.
func check(path string) error {
	policy, err := community.LoadAccessPolicy(path)
	if err != nil {
		return err
	}
	if policy == nil {
		return fmt.Errorf("loaded no policy")
	}
	return nil
}
