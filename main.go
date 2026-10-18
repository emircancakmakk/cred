package main

import (
	"fmt"
	"os"

	"github.com/emircancakmakk/cred/cmd"
	"github.com/emircancakmakk/cred/internal/clipboard"
)

func main() {
	if handled, err := clipboard.HandleClearArgs(os.Args[1:], os.Stdin); handled {
		if err != nil {
			os.Exit(1)
		}
		return
	}
	app := cmd.NewApp(os.Stdin, os.Stdout, os.Stderr)
	if err := app.Execute(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "cred: %s\n", err)
		os.Exit(1)
	}
}
