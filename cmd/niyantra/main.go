// Package main is the CLI entrypoint for Niyantra.
//
// Commands:
//
//	niyantra snap      Capture current Antigravity account's quota (1 API call)
//	niyantra status    Show all accounts' readiness (0 network calls)
//	niyantra serve     Start the web dashboard
//	niyantra version   Print version information
package main

import (
	"fmt"
	"os"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	switch os.Args[1] {
	case "snap":
		fmt.Println("TODO: snap command")
	case "status":
		fmt.Println("TODO: status command")
	case "serve":
		fmt.Println("TODO: serve command")
	case "version":
		fmt.Printf("niyantra %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Niyantra — Multi-account quota ledger for Antigravity

Usage:
  niyantra <command> [flags]

Commands:
  snap       Capture current account's quota (1 API call)
  status     Show all accounts' readiness (0 network calls)
  serve      Start the web dashboard
  version    Print version information

Flags:
  --port     Dashboard port (default: 9222)
  --db       Database path (default: ~/.niyantra/niyantra.db)
  --debug    Enable verbose logging`)
}
