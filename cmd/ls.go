package cmd

import (
	"fmt"

	"github.com/nexus/nexus/internal/app"
	"github.com/nexus/nexus/internal/validator"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:     "ls <token>",
	GroupID: groupCore,
	Short: "Scan a source and publish file metadata to a stream",
	Long:    "List services and resources associated with an authentication token.",
	Example: fmt.Sprintf(`  %s ls my-service-token
  %s ls my-service-token --output json
  %s ls my-service-token --filter api --limit 20`, app.Name, app.Name, app.Name),
	Args: exactArgs("<token>"),

	RunE: func(cmd *cobra.Command, args []string) error {
		token := args[0]

		outputFmt, _ := cmd.Flags().GetString("output")
		filter, _ := cmd.Flags().GetString("filter")
		limit, _ := cmd.Flags().GetInt("limit")
		verbose, _ := cmd.Flags().GetCount("verbose")

		if verbose >= 1 {
			fmt.Printf("[verbose:%d] token=%s output=%s filter=%s limit=%d\n",
				verbose, token, outputFmt, filter, limit)
		}

		fmt.Printf("Listing resources for token: %s\n", token)
		if filter != "" {
			fmt.Printf("  Filter: %s\n", filter)
		}
		if limit > 0 {
			fmt.Printf("  Limit:  %d results\n", limit)
		}
		// TODO: implement business logic + output formatting (table/json)
		return nil
	},
}

func init() {
	v := validator.New().Add(
		validator.ValidateOutputFormat(),
		validator.ValidateLimit(),
	)

	lsCmd.PreRunE = v.PreRunE()

	lsCmd.Flags().StringP("output", "o", "table", "Output format: table, json")
	lsCmd.Flags().StringP("filter", "f", "", "Filter resources by name (e.g. api, worker)")
	lsCmd.Flags().IntP("limit", "l", 0, "Maximum number of results (0 = no limit)")

	rootCmd.AddCommand(lsCmd)
}
