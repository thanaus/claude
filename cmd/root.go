package cmd

import (
	lsservice "github.com/nexus/nexus/internal/service/ls"
	statusservice "github.com/nexus/nexus/internal/service/status"
	syncservice "github.com/nexus/nexus/internal/service/sync"
	workerservice "github.com/nexus/nexus/internal/service/worker"

	"github.com/nexus/nexus/internal/app"
	"github.com/spf13/cobra"
)

// NewRootCmd builds the CLI and wires shared infrastructure into subcommands.
func NewRootCmd() *cobra.Command {
	root := buildRootCmd()

	root.AddCommand(NewSyncCmd(syncservice.New()))
	root.AddCommand(NewLSCmd(lsservice.New()))
	root.AddCommand(NewWorkerCmd(workerservice.New()))
	root.AddCommand(NewStatusCmd(statusservice.New()))
	root.AddCommand(NewVersionCmd())

	return root
}

func buildRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   app.Name,
		Short: app.DisplayName + " — Distributed file synchronization powered by NATS.",
		Long: app.DisplayName + ` — Distributed file synchronization powered by NATS.

Define sync jobs, stream file metadata, and scale workers independently
to synchronize data across any infrastructure.`,
	}

	root.Version = app.Version
	root.SuggestionsMinimumDistance = 2
	root.SetFlagErrorFunc(flagErrorFunc)

	// Global flags (available on all subcommands).
	root.PersistentFlags().CountP("verbose", "v", "Increase verbosity (-v, -vv, -vvv)")

	// SilenceErrors: we handle error display ourselves in main().
	root.SilenceErrors = true
	// SilenceUsage: never let Cobra print usage automatically on error.
	// Users must explicitly run `<app> help <cmd>` or `<app> <cmd> --help`.
	// See docs/adr/ADR-001-silence-usage.md
	root.SilenceUsage = true
	root.CompletionOptions.DisableDefaultCmd = true

	root.AddGroup(&cobra.Group{
		ID:    groupCore,
		Title: "Core Commands:",
	})
	root.AddGroup(&cobra.Group{
		ID:    groupMonitoring,
		Title: "Monitoring:",
	})
	root.AddGroup(&cobra.Group{
		ID:    groupOther,
		Title: "Other Commands:",
	})

	root.SetHelpCommandGroupID(groupOther)

	return root
}
