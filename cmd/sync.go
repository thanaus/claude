package cmd

import (
	"fmt"
	"time"

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
		"Define a source directory and a destination (local path, cloud, etc.)",
		"<source>",
		"<destination>",
	),

	RunE: func(cmd *cobra.Command, args []string) error {
		source := args[0]
		destination := args[1]
		env, _ := cmd.Flags().GetString("env")
		timeout, _ := cmd.Flags().GetDuration("timeout")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		verbose, _ := cmd.Flags().GetCount("verbose")

		if verbose >= 1 {
			fmt.Printf("[verbose:%d] source=%s destination=%s env=%s timeout=%s dry-run=%v\n",
				verbose, source, destination, env, timeout, dryRun)
		}

		if dryRun {
			fmt.Printf("[dry-run] Simulating sync: %s → %s\n", source, destination)
			return nil
		}

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
	)

	syncCmd.PreRunE = v.PreRunE()

	syncCmd.Flags().String("env", "", "Target environment (e.g. production, staging)")
	syncCmd.Flags().Duration("timeout", 30*time.Second, "Sync timeout (e.g. 30s, 2m)")
	syncCmd.Flags().Bool("dry-run", false, "Simulate the sync without applying changes")

	rootCmd.AddCommand(syncCmd)
}
