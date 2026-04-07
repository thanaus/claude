package cmd

import (
	"context"

	"github.com/nexus/nexus/internal/app"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   app.Name,
	Short: app.DisplayName + " — Distributed file synchronization powered by NATS.",
	Long: app.DisplayName + ` — Distributed file synchronization powered by NATS.

Define sync jobs, stream file metadata, and scale workers independently
to synchronize data across any infrastructure.`,
}

// Execute is the entry point called by main.go.
func Execute() error {
	return ExecuteContext(context.Background())
}

// ExecuteContext runs the root command with a caller-provided context.
func ExecuteContext(ctx context.Context) error {
	return rootCmd.ExecuteContext(ctx)
}

func init() {
	rootCmd.Version = app.Version
	rootCmd.SuggestionsMinimumDistance = 2
	rootCmd.SetFlagErrorFunc(flagErrorFunc)

	// Global flags (available on all subcommands).
	rootCmd.PersistentFlags().CountP("verbose", "v", "Increase verbosity (-v, -vv, -vvv)")

	// SilenceErrors: we handle error display ourselves in Execute().
	rootCmd.SilenceErrors = true
	// SilenceUsage: never let Cobra print usage automatically on error.
	// Users must explicitly run `<app> help <cmd>` or `<app> <cmd> --help`.
	// See docs/adr/ADR-001-silence-usage.md
	rootCmd.SilenceUsage = true
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	rootCmd.AddGroup(&cobra.Group{
		ID:    groupCore,
		Title: "Core Commands:",
	})
	rootCmd.AddGroup(&cobra.Group{
		ID:    groupMonitoring,
		Title: "Monitoring:",
	})
	rootCmd.AddGroup(&cobra.Group{
		ID:    groupOther,
		Title: "Other Commands:",
	})

	rootCmd.SetHelpCommandGroupID(groupOther)
}
