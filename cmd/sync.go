package cmd

import (
	"fmt"

	"github.com/nexus/nexus/internal/app"
	"github.com/nexus/nexus/internal/cli/validator"
	"github.com/nexus/nexus/internal/config"
	syncservice "github.com/nexus/nexus/internal/service/sync"
	"github.com/spf13/cobra"
)

func NewSyncCmd(svc syncservice.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "sync <source> <destination>",
		GroupID: groupCore,
		Short: "Create a synchronization job between a source and a destination",
		Long: `Create a synchronization job between a source and a destination.

The job configuration is stored in NATS and identified by a unique token.
This token must be used with the list and workers commands to
scan and process files.

No files are transferred during this step.`,
		Args: exactArgs(
			"Provide both source and destination directories.",
			"<source>",
			"<destination>",
		),
	}

	v := validator.New().Add(
		validator.ValidateNATSConfig(
			fmt.Sprintf("Define a valid NATS configuration before creating a synchronization job.\nCheck %s and, if set, %s.", app.NATSURLEnv, app.NATSProbeTimeoutEnv),
		),
		validator.ValidateSyncPaths(0, 1),
	)

	cmd.PreRunE = v.PreRunE()
	cmd.RunE = newSyncRunE(svc)

	return cmd
}

func newSyncRunE(svc syncservice.Service) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		source := args[0]
		destination := args[1]
		natsCfg, _ := config.NATSConfigFromContext(cmd.Context())

		result, err := svc.Provision(cmd.Context(), syncservice.Input{
			Source:      source,
			Destination: destination,
		})
		if err != nil {
			return runtimeError(
				fmt.Sprintf("Failed to initialize NATS resources: %s", natsCfg.URL),
				fmt.Sprintf("Ensure the NATS server is running, JetStream is enabled, and reachable.\nCheck the %s environment variable.", app.NATSURLEnv),
				err,
			)
		}

		fmt.Println("✔ Sync job created")
		fmt.Println()
		fmt.Printf("%-14s %s\n", "Source:", source)
		fmt.Printf("%-14s %s\n", "Destination:", destination)
		fmt.Printf("%-14s %s\n", "Token:", result.NATS.Token)
		fmt.Println()
		fmt.Printf("%-14s %s\n", "NATS:", result.NATS.URL)
		fmt.Printf("%-14s %s\n", "JetStream:", jetStreamStatus(result.NATS.JetStreamReady))
		fmt.Println()
		fmt.Println("Resources:")
		for _, stream := range result.NATS.Streams {
			fmt.Printf("  %-21s %s %s\n", "Stream "+stream.Name, "✔", resourceDisplayStatus(stream.Status))
		}
		fmt.Printf("  %-21s %s %s\n", "KV "+result.NATS.KeyValue.Name, "✔", resourceDisplayStatus(result.NATS.KeyValue.Status))
		fmt.Println()
		fmt.Println("Next steps:")
		fmt.Printf("  %s ls %s\n", app.Name, result.NATS.Token)
		fmt.Printf("  %s worker %s\n", app.Name, result.NATS.Token)

		return nil
	}
}

func jetStreamStatus(ready bool) string {
	if ready {
		return "enabled"
	}

	return "disabled"
}

func resourceDisplayStatus(status string) string {
	switch status {
	case "", "created":
		return "ready"
	default:
		return status
	}
}
