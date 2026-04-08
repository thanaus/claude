package cmd

import (
	"fmt"

	"github.com/nexus/nexus/internal/app"
	"github.com/nexus/nexus/internal/cli/validator"
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
		validator.ValidateEnvRequired(
			app.NATSURLEnv,
			"Define the NATS server URL before creating a synchronization job",
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

		result, err := svc.CheckNATS(cmd.Context(), syncservice.Input{
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
	}
}
