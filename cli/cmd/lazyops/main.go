package main

import (
	"context"
	"fmt"
	"os"

	"lazyops-cli/internal/app"
)

func main() {
	ctx := context.Background()

	application, err := app.NewFromEnv(os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap error: %v\n", err)
		os.Exit(1)
	}

	if err := application.Run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
