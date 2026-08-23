package main

import (
	"fmt"
	"os"

	"github.com/dzaneyo/relay/internal/app"
	"github.com/dzaneyo/relay/internal/cli"
	"github.com/dzaneyo/relay/internal/storage"
)

func main() {
	db, err := storage.Open()
	if err != nil {
		fmt.Fprintln(os.Stderr, "open database:", err)
		os.Exit(1)
	}

	defer db.Close()

	application := app.New(db)

	if err := cli.NewRootCommand(application).Execute(); err != nil {
		os.Exit(1)
	}
}
