package main

import "github.com/mandeep/muxforge/cmd"

// Build-time variables injected by goreleaser ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cmd.Execute(version, commit, date)
}
