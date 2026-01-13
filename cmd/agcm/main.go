package main

import "github.com/green/agcm/cmd/agcm/cmd"

// version is set at build time via ldflags
var version = "dev"

func main() {
	cmd.SetVersion(version)
	cmd.Execute()
}
