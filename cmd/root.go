package cmd

import (
	"fmt"
	"os"

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
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Version = app.Version

	// Global flags (available on all subcommands).
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose logging")

	// SilenceErrors: we handle error display ourselves in Execute().
	rootCmd.SilenceErrors = true
	// SilenceUsage: never let Cobra print usage automatically on error.
	// Users must explicitly run `nexus help <cmd>` or `nexus <cmd> --help`.
	// See docs/adr/ADR-001-silence-usage.md
	rootCmd.SilenceUsage = true
}
