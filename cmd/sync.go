package cmd

import (
	"fmt"

	"github.com/nexus/nexus/internal/app"
	"github.com/nexus/nexus/internal/validator"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync <source> <destination>",
	Short: "Create a synchronization job and generate a token",
	Long: fmt.Sprintf(`Synchronize resources from a source service to a destination service.

Required arguments:
  <source>       Source service identifier
  <destination>  Destination service identifier

Examples:
  %s sync service-a service-b
  %s sync service-a service-b --env production --dry-run
  %s sync service-a service-b --env staging --timeout 60s --verbose`, app.Name, app.Name, app.Name),

	RunE: func(cmd *cobra.Command, args []string) error {
		source := args[0]
		destination := args[1]

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		env, _ := cmd.Flags().GetString("env")
		timeout, _ := cmd.Flags().GetString("timeout")
		verbose, _ := cmd.Flags().GetBool("verbose")

		if verbose {
			fmt.Printf("[verbose] source=%s destination=%s env=%s timeout=%s dry-run=%v\n",
				source, destination, env, timeout, dryRun)
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
		validator.RequireArgs(2, []string{"<source>", "<destination>"}),
		validator.ValidateTimeout(),
	)

	syncCmd.PreRunE = v.PreRunE()

	syncCmd.Flags().String("env", "", "Target environment (e.g. production, staging)")
	syncCmd.Flags().String("timeout", "30s", "Sync timeout (e.g. 30s, 2m)")
	syncCmd.Flags().Bool("dry-run", false, "Simulate the sync without applying changes")

	rootCmd.AddCommand(syncCmd)
}
