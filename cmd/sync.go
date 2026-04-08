package cmd

import (
	"fmt"

	"github.com/nexus/nexus/internal/app"
	"github.com/nexus/nexus/internal/cli/validator"
	natsclient "github.com/nexus/nexus/internal/nats"
	syncservice "github.com/nexus/nexus/internal/service/sync"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
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

	RunE: func(cmd *cobra.Command, args []string) error {
		source := args[0]
		destination := args[1]

		service := syncservice.New(natsclient.Client{})
		result, err := service.CheckNATS(cmd.Context(), syncservice.Input{
			Source:      source,
			Destination: destination,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Syncing: %s → %s\n", source, destination)
		fmt.Printf("NATS connection OK: %s\n", result.NATS.URL)
		if result.NATS.JetStreamReady {
			fmt.Println("JetStream is available.")
		}

		return nil
	},
}

func init() {
	v := validator.New().Add(
		validator.ValidateEnvRequired(
			app.NATSURLEnv,
			"Define the NATS server URL before creating a synchronization job",
		),
		validator.ValidateSyncPaths(0, 1),
	)

	syncCmd.PreRunE = v.PreRunE()

	rootCmd.AddCommand(syncCmd)
}
