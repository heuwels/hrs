package main

import (
	"fmt"
	"os"
)

const usage = `hrs - timesheets for your agent

usage:
  hrs serve [flags]     start the API server
  hrs log [flags]       add an entry
  hrs ls [date]         list entries (default: today)
  hrs tui [date]        interactive explorer
  hrs migrate [flags]   import existing markdown files
  hrs docs [flags]      serve the documentation site
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "serve":
		err = cmdServe(os.Args[2:])
	case "log":
		err = cmdLog(os.Args[2:])
	case "ls":
		err = cmdLs(os.Args[2:])
	case "tui":
		err = cmdTUI(os.Args[2:])
	case "migrate":
		err = cmdMigrate(os.Args[2:])
	case "docs":
		err = cmdDocs(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", os.Args[1], usage)
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
