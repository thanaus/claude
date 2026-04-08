package main

import (
	"context"
	"fmt"
	"os"

	"github.com/nexus/nexus/cmd"
)

func main() {
	if err := cmd.NewRootCmd().ExecuteContext(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
