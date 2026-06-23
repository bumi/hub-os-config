// Command hub-os-config is the Alby Hub appliance configuration service.
//
// Subcommands:
//
//	run         (default) run the configuration service
//	update      download and replace the executable from a hard-coded URL
//	version     print the version
package main

import (
	"fmt"
	"os"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	cmd := "run"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	var err error
	switch cmd {
	case "run":
		err = runService()
	case "update":
		err = update()
	case "version", "--version", "-v":
		fmt.Println(version)
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", cmd)
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`hub-os-config - Alby Hub appliance configuration service

Usage:
  hub-os-config [command]

Commands:
  run         Run the configuration service (default)
  update      Download and replace this executable from a hard-coded URL
  version     Print the version
  help        Show this help
`)
}
