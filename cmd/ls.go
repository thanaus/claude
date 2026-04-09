package cmd

import (
	"fmt"

	"github.com/nexus/nexus/internal/app"
	"github.com/nexus/nexus/internal/cli/validator"
	"github.com/nexus/nexus/internal/config"
	lsservice "github.com/nexus/nexus/internal/service/ls"
	"github.com/spf13/cobra"
)

func NewLSCmd(svc lsservice.Service) *cobra.Command {
	lsCmd := &cobra.Command{
		Use:     "ls <token>",
		GroupID: groupCore,
		Short: "Scan a source and publish file metadata to a NATS stream",
		Long: `Scan a source and publish file metadata to a NATS stream.

This command reads the sync job configuration using the provided token,
scans the source, and publishes each file as a message to a stream.

Workers can then consume these messages to perform synchronization.`,
		Args: exactArgs(
			fmt.Sprintf("Create a sync job first to obtain a token\nRun '%s sync <source> <destination>'", app.Name),
			"<token>",
		),
	}

	v := validator.New().Add(
		validator.ValidateNATSConfig(
			fmt.Sprintf("Define a valid NATS configuration before running the listing workflow.\nCheck %s and, if set, %s.", app.NATSURLEnv, app.NATSProbeTimeoutEnv),
		),
		validator.ValidateOutputFormat(),
		validator.ValidateLimit(),
	)

	lsCmd.PreRunE = v.PreRunE()
	lsCmd.RunE = newLSRunE(svc)
	lsCmd.Flags().StringP("output", "o", "table", "Output format: table, json")
	lsCmd.Flags().StringP("filter", "f", "", "Filter resources by name (e.g. api, worker)")
	lsCmd.Flags().IntP("limit", "l", 0, "Maximum number of results (0 = no limit)")

	return lsCmd
}

func newLSRunE(svc lsservice.Service) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		token := args[0]
		natsCfg, _ := config.NATSConfigFromContext(cmd.Context())

		result, err := svc.Run(cmd.Context(), lsservice.Input{
			Token: token,
		})
		if err != nil {
			return runtimeError(
				fmt.Sprintf("Failed to initialize ls prerequisites: %s", natsCfg.URL),
				fmt.Sprintf("Ensure the NATS server is running, JetStream is enabled, and the KV bucket %q already exists.\nCreate a job first with '%s sync <source> <destination>'.", "jobs", app.Name),
				err,
			)
		}

		fmt.Println("✔ LS prerequisites ready")
		fmt.Println()
		fmt.Printf("%-14s %s\n", "Token:", token)
		fmt.Printf("%-14s %s\n", "Source:", result.Job.Source)
		fmt.Printf("%-14s %s\n", "State:", result.Job.State)
		fmt.Println()
		fmt.Printf("%-14s %s\n", "NATS:", result.URL)
		fmt.Printf("%-14s %s\n", "JetStream:", jetStreamStatus(result.JetStreamReady))
		fmt.Println()
		fmt.Println("Resources:")
		fmt.Printf("  %-21s %s %s\n", "KV "+result.KeyValue.Name, "✔", resourceDisplayStatus(result.KeyValue.Status))
		fmt.Printf("  %-21s %s %s\n", "DISCOVERY root", "✔", boolStatus(result.DiscoveryPublished))
		fmt.Printf("  %-21s %s %d\n", "Entries discovered", "✔", result.DiscoveredEntries)
		fmt.Printf("  %-21s %s %d\n", "Published to WORK", "✔", result.PublishedWork)
		fmt.Printf("  %-21s %s %d\n", "Errors", "✔", result.Errors)
		if result.Warning != "" {
			fmt.Println()
			fmt.Println("Note:")
			fmt.Printf("  %s\n", result.Warning)
		}

		return nil
	}
}

func boolStatus(ok bool) string {
	if ok {
		return "published"
	}

	return "pending"
}
