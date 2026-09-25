// Package version holds the version of this build of MAYAK. Task's build
// tasks set it from git (`git describe --tags`) with
// -ldflags "-X github.com/local/mayak/internal/version.Version=...";
// a plain `go build` leaves the development value.
package version

import "strings"

// Version is this build's version, as the release tag names it ("0.1.0",
// "v0.1.0"), a `git describe` value after a tag ("0.1.0-3-g1a2b3c4"), or
// Development when nothing set it.
var Version = Development

// Development is the version of a build that nothing versioned.
const Development = "dev"

// Current returns the version without a leading "v".
func Current() string { return strings.TrimPrefix(strings.TrimSpace(Version), "v") }

// IsDevelopment reports whether this build carries no release version.
func IsDevelopment() bool { return Current() == "" || Current() == Development }

// UserAgent is the User-Agent this build sends to web services.
func UserAgent() string { return "MAYAK/" + Current() }
