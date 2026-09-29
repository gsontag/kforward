package main

import "runtime/debug"

// version is the version go build stamped from git: the tag, a pseudo-version
// after it, with +dirty for uncommitted changes, or (devel) without git.
func version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	return info.Main.Version
}
