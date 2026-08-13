package community

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The revision arrives through -X, which fails silently when its symbol path is
// wrong. See docs/sirens-echo-build-revision.md.

// xTarget captures the symbol the Dockerfile stamps, so the test compares the
// real string rather than one written twice.
var xTarget = regexp.MustCompile(`-X\s+([^\s=]+)=`)

// The linker does not fail on an unknown symbol. It writes nothing and exits
// zero, so a rename here is invisible until someone reads a log expecting a sha.
func TestDockerfileStampsThisPackagesVariable(t *testing.T) {
	t.Parallel()

	dockerfile, err := os.ReadFile("../../Dockerfile")
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}
	match := xTarget.FindStringSubmatch(string(dockerfile))
	if match == nil {
		t.Fatal("the Dockerfile stamps no -X symbol; the revision cannot reach the binary")
	}
	stamped := match[1]

	want := modulePath(t) + "/internal/community.buildRevision"
	if stamped != want {
		t.Errorf("the Dockerfile stamps %q, but this package's variable is %q. "+
			"A -X path that resolves to nothing is silently ignored", stamped, want)
	}
}

// The publish script has to pass the build arg the Dockerfile declares, or the
// stamp is applied with an empty value and looks the same as no stamp at all.
func TestPublishPassesTheRevisionBuildArg(t *testing.T) {
	t.Parallel()

	dockerfile, err := os.ReadFile("../../Dockerfile")
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}
	declared := regexp.MustCompile(`ARG\s+(SIRENS_ECHO_\w*REVISION\w*)`).FindStringSubmatch(string(dockerfile))
	if declared == nil {
		t.Fatal("the Dockerfile declares no revision ARG")
	}

	script, err := os.ReadFile("../../scripts/publish-image.sh")
	if err != nil {
		t.Fatalf("read publish-image.sh: %v", err)
	}
	if !strings.Contains(string(script), "--build-arg "+declared[1]+"=") {
		t.Errorf("publish-image.sh does not pass --build-arg %s, so the stamp would be empty",
			declared[1])
	}
}

// modulePath reads go.mod rather than hardcoding, so a module rename surfaces
// as one failure here instead of a silently unstamped build.
func modulePath(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	for _, line := range strings.Split(string(body), "\n") {
		if after, found := strings.CutPrefix(line, "module "); found {
			return strings.TrimSpace(after)
		}
	}
	t.Fatal("go.mod declares no module path")
	return ""
}
