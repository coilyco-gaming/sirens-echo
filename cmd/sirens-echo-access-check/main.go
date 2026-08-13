// Offline validation of an access policy, for a caller that cannot import Go.
// Reads files and nothing else. See docs/sirens-echo-access-check.md.
package main

import (
	"fmt"
	"os"

	"forgejo.coilysiren.me/coilyco-gaming/sirens-echo/internal/community"
)

// usage names the argument rather than describing the tool, because the caller
// is a CI step that already knows what it invoked.
const usage = "usage: sirens-echo-access-check <access-policy.yaml> [...]"

func main() {
	paths := os.Args[1:]
	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	failed := false
	for _, path := range paths {
		if err := check(path); err != nil {
			// The path is repeated because a CI log shows one line, and which
			// file failed is the first thing the reader needs.
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
			failed = true
			continue
		}
		fmt.Printf("%s: ok\n", path)
	}
	if failed {
		os.Exit(1)
	}
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
