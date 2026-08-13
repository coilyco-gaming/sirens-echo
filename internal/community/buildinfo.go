package community

// buildRevision is set at build time with -X. An unset value means the binary
// was built without one. See docs/sirens-echo-build-revision.md.
var buildRevision = ""

// BuildRevision reports the commit this binary was built from, or an empty
// string when the build did not carry one.
func BuildRevision() string { return buildRevision }
