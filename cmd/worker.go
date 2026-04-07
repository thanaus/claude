package cmd

import (
	"fmt"

	"github.com/nexus/nexus/internal/app"
	"github.com/spf13/cobra"
)

var workerCmd = &cobra.Command{
	Use:     "worker <token>",
	GroupID: groupOperations,
	Short: "Run workers to process and synchronize files",
	Long:    "Start, stop, or inspect workers associated with a service token.",
	Example: fmt.Sprintf(`  %s worker my-service-token
  %s worker my-service-token --env production
  %s worker my-service-token --namespace payments --verbose`, app.Name, app.Name, app.Name),
	Args: exactArgs("<token>"),

	RunE: func(cmd *cobra.Command, args []string) error {
		token := args[0]

		env, _ := cmd.Flags().GetString("env")
		namespace, _ := cmd.Flags().GetString("namespace")
		verbose, _ := cmd.Flags().GetBool("verbose")

		if verbose {
			fmt.Printf("[verbose] token=%s env=%s namespace=%s\n",
				token, env, namespace)
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

func init() {
	workerCmd.Flags().String("env", "", "Target environment (e.g. production, staging)")
	workerCmd.Flags().String("namespace", "", "Worker namespace (e.g. payments, auth)")

	rootCmd.AddCommand(workerCmd)
}
