package cmd

import (
	"fmt"

	"github.com/nexus/nexus/internal/app"
	"github.com/spf13/cobra"
)

func NewWorkerCmd() *cobra.Command {
	workerCmd := &cobra.Command{
		Use:     "worker <token>",
		GroupID: groupCore,
		Short: "Process file events from a NATS stream and synchronize data to the destination",
		Long: `Process file events from a NATS stream and synchronize data to the destination.

Workers consume messages published by the list command, compare files
with the destination, and transfer them if needed.

Multiple workers can run in parallel to scale processing.`,
		Args: exactArgs(
			fmt.Sprintf("Create a sync job first to obtain a token\nRun '%s sync <source> <destination>'", app.Name),
			"<token>",
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			token := args[0]

			env, _ := cmd.Flags().GetString("env")
			namespace, _ := cmd.Flags().GetString("namespace")
			verbose, _ := cmd.Flags().GetCount("verbose")

			if verbose >= 1 {
				fmt.Printf("[verbose:%d] token=%s env=%s namespace=%s\n",
					verbose, token, env, namespace)
			}

			fmt.Printf("Managing workers for token: %s\n", token)
			if env != "" {
				fmt.Printf("  Environment: %s\n", env)
			}
			if namespace != "" {
				fmt.Printf("  Namespace:   %s\n", namespace)
			}
			// TODO: implement business logic
			return nil
		},
	}

	workerCmd.Flags().String("env", "", "Target environment (e.g. production, staging)")
	workerCmd.Flags().String("namespace", "", "Worker namespace (e.g. payments, auth)")

	return workerCmd
}
