package main

import "runtime/debug"

// version is stamped in by release builds:
// go build -ldflags "-X main.version=v1.2.3".
var version string

// release returns this build's version: the stamped one, else the one Go
// records, which is the tag for "go install ...@v1.2.3" and a commit-based
// version for a local build, else "dev".
func release() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
