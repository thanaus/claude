package cmd

import (
	"fmt"
	"time"

	"github.com/nexus/nexus/internal/app"
	"github.com/spf13/cobra"
)

var (
	syncEnv     string
	syncTimeout time.Duration
	syncDryRun  bool
)

var syncCmd = &cobra.Command{
	Use:     "sync <source> <destination>",
	GroupID: groupOperations,
	Short: "Create a synchronization job and generate a token",
	Long:    "Synchronize resources from a source service to a destination service.",
	Example: fmt.Sprintf(`  %s sync service-a service-b
  %s sync service-a service-b --env production --dry-run
  %s sync service-a service-b --env staging --timeout 60s --verbose`, app.Name, app.Name, app.Name),
	Args: exactArgs("<source>", "<destination>"),

	RunE: func(cmd *cobra.Command, args []string) error {
		source := args[0]
		destination := args[1]
		verbose, _ := cmd.Flags().GetBool("verbose")

		if verbose {
			fmt.Printf("[verbose] source=%s destination=%s env=%s timeout=%s dry-run=%v\n",
				source, destination, syncEnv, syncTimeout, syncDryRun)
		}

		if syncDryRun {
			fmt.Printf("[dry-run] Simulating sync: %s → %s\n", source, destination)
			return nil
		}

		fmt.Printf("Syncing: %s → %s\n", source, destination)
		// TODO: implement business logic
		return nil
	},
}

func init() {
	syncCmd.Flags().StringVar(&syncEnv, "env", "", "Target environment (e.g. production, staging)")
	syncCmd.Flags().DurationVar(&syncTimeout, "timeout", 30*time.Second, "Sync timeout (e.g. 30s, 2m)")
	syncCmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "Simulate the sync without applying changes")

	rootCmd.AddCommand(syncCmd)
}
