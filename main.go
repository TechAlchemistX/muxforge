package main

import "github.com/TechAlchemistX/muxforge/cmd"

// Build-time variables injected by goreleaser ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cmd.Execute(version, commit, date)
}
