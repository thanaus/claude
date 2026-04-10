package cmd

import (
	"fmt"

	"github.com/nexus/nexus/internal/app"
	"github.com/nexus/nexus/internal/cli/validator"
	"github.com/nexus/nexus/internal/config"
	workerservice "github.com/nexus/nexus/internal/service/worker"
	"github.com/spf13/cobra"
)

func NewWorkerCmd(svc workerservice.Service) *cobra.Command {
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
	}

	v := validator.New().Add(
		validator.ValidateNATSConfig(
			fmt.Sprintf("Define a valid NATS configuration before running the worker.\nCheck %s and, if set, %s.", app.NATSURLEnv, app.NATSProbeTimeoutEnv),
		),
	)

	workerCmd.PreRunE = v.PreRunE()
	workerCmd.RunE = newWorkerRunE(svc)
	workerCmd.Flags().String("env", "", "Target environment (e.g. production, staging)")
	workerCmd.Flags().String("namespace", "", "Worker namespace (e.g. payments, auth)")
	workerCmd.Flags().IntP("workers", "w", 4, "Number of worker comparison goroutines")

	return workerCmd
}

func newWorkerRunE(svc workerservice.Service) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		token := args[0]
		env, _ := cmd.Flags().GetString("env")
		namespace, _ := cmd.Flags().GetString("namespace")
		workers, _ := cmd.Flags().GetInt("workers")
		verbose, _ := cmd.Flags().GetCount("verbose")
		natsCfg, _ := config.NATSConfigFromContext(cmd.Context())

		if verbose >= 1 {
			fmt.Printf("[verbose:%d] token=%s env=%s namespace=%s\n", verbose, token, env, namespace)
		}

		result, err := svc.Run(cmd.Context(), workerservice.Input{
			Token:   token,
			Workers: workers,
		})
		if err != nil {
			return runtimeError(
				fmt.Sprintf("Failed to run worker workflow: %s", natsCfg.URL),
				fmt.Sprintf("Ensure the NATS server is running, JetStream is enabled, and the job token %q exists in the KV bucket %q.\nCreate a job first with '%s sync <source> <destination>' then publish work with '%s ls <token>'.", token, "jobs", app.Name, app.Name),
				err,
			)
		}

		fmt.Println("✔ Worker completed")
		fmt.Println()
		fmt.Printf("%-14s %s\n", "Token:", result.Job.Token)
		fmt.Printf("%-14s %s\n", "Destination:", result.Job.Destination)
		fmt.Println()
		fmt.Println("Summary:")
		fmt.Printf("  %-21s %d\n", "Processed", result.WorkerProcessed)
		fmt.Printf("  %-21s %d\n", "To copy", result.WorkerToCopy)
		fmt.Printf("  %-21s %d\n", "Already OK", result.WorkerOK)
		fmt.Printf("  %-21s %d\n", "Errors", result.WorkerErrors)

		return nil
	}
}
