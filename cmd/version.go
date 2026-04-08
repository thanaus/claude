package cmd

import (
	"fmt"

	"github.com/nexus/nexus/internal/app"
	"github.com/spf13/cobra"
)

func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		GroupID: groupOther,
		Short:   fmt.Sprintf(`Show %s version`, app.DisplayName),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("%s %s\n", app.DisplayName, app.Version)
		},
	}
}
