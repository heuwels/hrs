package main

import (
	"fmt"
	"os"
)

var version = "dev"

const usage = `hrs - timesheets for your agent

usage:
  hrs serve [flags]     start the API server
  hrs log [flags]       add an entry
  hrs ls [date]         list entries (default: today)
  hrs goals [action]    manage daily goals
  hrs tui [date]        interactive explorer
  hrs edit <id>         edit an entry
  hrs rm <id>           delete an entry
  hrs export [flags]    export entries (json|csv)
  hrs categories        list all categories
  hrs migrate [flags]   import existing markdown files
  hrs docs [flags]      serve the documentation site
  hrs version           print version
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
	case "goals":
		err = cmdGoals(os.Args[2:])
	case "tui":
		err = cmdTUI(os.Args[2:])
	case "edit":
		err = cmdEdit(os.Args[2:])
	case "rm":
		err = cmdRm(os.Args[2:])
	case "export":
		err = cmdExport(os.Args[2:])
	case "categories":
		err = cmdCategories(os.Args[2:])
	case "migrate":
		err = cmdMigrate(os.Args[2:])
	case "docs":
		err = cmdDocs(os.Args[2:])
	case "version":
		fmt.Println(version)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", os.Args[1], usage)
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
