package cmd

import (
	"fmt"

	"github.com/nexus/nexus/internal/app"
	"github.com/nexus/nexus/internal/cli/validator"
	"github.com/spf13/cobra"
)

func NewStatusCmd() *cobra.Command {
	statusCmd := &cobra.Command{
		Use:     "status <token>",
		GroupID: groupMonitoring,
		Short: "Show job and worker status",
		Long: "Display the health and operational status of services associated " +
			"with an authentication token.",
		Example: fmt.Sprintf(`  %s status my-service-token
  %s status my-service-token --output json
  %s status my-service-token --watch --verbose`, app.Name, app.Name, app.Name),
		Args: exactArgs("", "<token>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			token := args[0]

			outputFmt, _ := cmd.Flags().GetString("output")
			watch, _ := cmd.Flags().GetBool("watch")
			verbose, _ := cmd.Flags().GetCount("verbose")

			if verbose >= 1 {
				fmt.Printf("[verbose:%d] token=%s output=%s watch=%v\n",
					verbose, token, outputFmt, watch)
			}

			if watch {
				fmt.Printf("Watching services for token: %s\n", token)
				// TODO: implement polling / watch loop
				return nil
			}

			fmt.Printf("Status of services for token: %s\n", token)
			// TODO: implement business logic + output formatting (table/json)
			return nil
		},
	}

	v := validator.New().Add(
		validator.ValidateOutputFormat(),
	)

	statusCmd.PreRunE = v.PreRunE()
	statusCmd.Flags().StringP("output", "o", "table", "Output format: table, json")
	statusCmd.Flags().Bool("watch", false, "Continuously refresh status")

	return statusCmd
}
