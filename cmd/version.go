package cmd

import (
	"fmt"

	"github.com/nexus/nexus/internal/app"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: fmt.Sprintf(`Show %s version`, app.DisplayName),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("%s %s\n", app.DisplayName, app.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
