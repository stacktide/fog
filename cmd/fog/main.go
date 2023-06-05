package main

import (
	"context"
	"os"
)

func main() {
	ctx := context.Background()

	err := rootCmd.ExecuteContext(ctx)

	if err != nil {
		os.Exit(1)
	}
}
