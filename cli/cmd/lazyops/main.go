package main

import (
	"context"
	"fmt"
	"os"

	"lazyops-cli/internal/app"
	"lazyops-cli/internal/redact"
)

func main() {
	ctx := context.Background()

	application, err := app.NewFromEnv(os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap error: %s\n", redact.Text(err.Error()))
		os.Exit(1)
	}

	if err := application.Run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", redact.Text(err.Error()))
		os.Exit(1)
	}
}
