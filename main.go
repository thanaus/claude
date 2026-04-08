package main

import (
	"context"
	"fmt"
	"os"

	"github.com/nexus/nexus/cmd"
	natsclient "github.com/nexus/nexus/internal/nats"
)

func main() {
	deps := cmd.Deps{
		NATSClient: natsclient.Client{},
	}

	if err := cmd.NewRootCmd(deps).ExecuteContext(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
