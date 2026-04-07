package cmd

import (
	"fmt"

	"github.com/nexus/nexus/internal/app"
	"github.com/nexus/nexus/internal/cli/validator"
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

		fmt.Printf("Syncing: %s → %s\n", source, destination)
		// TODO: implement business logic
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
