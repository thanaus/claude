package cmd

import (
	"fmt"

	"github.com/nexus/nexus/internal/app"
	"github.com/nexus/nexus/internal/cli/validator"
	"github.com/nexus/nexus/internal/config"
	lsservice "github.com/nexus/nexus/internal/service/ls"
	"github.com/spf13/cobra"
)

func NewLSCmd(svc lsservice.Service) *cobra.Command {
	lsCmd := &cobra.Command{
		Use:     "ls <token>",
		GroupID: groupCore,
		Short: "Scan a source and publish file metadata to a NATS stream",
		Long: `Scan a source and publish file metadata to a NATS stream.

This command reads the sync job configuration using the provided token,
scans the source, and publishes each file as a message to a stream.

Workers can then consume these messages to perform synchronization.`,
		Args: exactArgs(
			fmt.Sprintf("Create a sync job first to obtain a token\nRun '%s sync <source> <destination>'", app.Name),
			"<token>",
		),
	}

	v := validator.New().Add(
		validator.ValidateNATSConfig(
			fmt.Sprintf("Define a valid NATS configuration before running the listing workflow.\nCheck %s and, if set, %s.", app.NATSURLEnv, app.NATSProbeTimeoutEnv),
		),
		validator.ValidateOutputFormat(),
		validator.ValidateLimit(),
	)

	lsCmd.PreRunE = v.PreRunE()
	lsCmd.RunE = newLSRunE(svc)
	lsCmd.Flags().StringP("output", "o", "table", "Output format: table, json")
	lsCmd.Flags().StringP("filter", "f", "", "Filter resources by name (e.g. api, worker)")
	lsCmd.Flags().IntP("limit", "l", 0, "Maximum number of results (0 = no limit)")
	lsCmd.Flags().IntP("workers", "w", 4, "Number of scan workers")

	return lsCmd
}

func newLSRunE(svc lsservice.Service) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		token := args[0]
		natsCfg, _ := config.NATSConfigFromContext(cmd.Context())
		reporter := &lsCLIReporter{}
		workers, _ := cmd.Flags().GetInt("workers")

		result, err := svc.Run(cmd.Context(), lsservice.Input{
			Token:    token,
			Reporter: reporter,
			Workers:  workers,
		})
		reporter.EndProgress()
		if err != nil {
			return runtimeError(
				fmt.Sprintf("Failed to run ls workflow: %s", natsCfg.URL),
				fmt.Sprintf("Ensure the NATS server is running, JetStream is enabled, and the job token %q exists in the KV bucket %q.\nCreate a job first with '%s sync <source> <destination>'.", token, "jobs", app.Name),
				err,
			)
		}

		printLSCompleted(result)

		return nil
	}
}

type lsCLIReporter struct {
	progressActive bool
}

func (r *lsCLIReporter) OnPrepared(prepared lsservice.Prepared) {
	printLSPrepared(prepared)
}

func (r *lsCLIReporter) OnProgress(progress lsservice.Progress) {
	if r.progressActive {
		clearProgressBlock()
	}

	fmt.Println("Progress:")
	fmt.Printf("  %-19s %d\n", "Discovered", progress.DiscoveredEntries)
	fmt.Printf("  %-19s %d\n", "Published", progress.PublishedWork)
	fmt.Printf("  %-19s %d\n", "Errors", progress.Errors)
	r.progressActive = true
}

func (r *lsCLIReporter) EndProgress() {
	if !r.progressActive {
		return
	}

	clearProgressBlock()
	r.progressActive = false
}

func clearProgressBlock() {
	fmt.Print("\033[4A")
	for range 4 {
		fmt.Print("\033[2K")
		fmt.Print("\033[1B")
	}
	fmt.Print("\033[4A")
}

func boolStatus(ok bool) string {
	if ok {
		return "published"
	}

	return "pending"
}

func printLSPrepared(prepared lsservice.Prepared) {
	fmt.Println("✔ Scan started")
	fmt.Println()
	fmt.Printf("%-14s %s\n", "Token:", prepared.Job.Token)
	fmt.Printf("%-14s %s\n", "Source:", prepared.Job.Source)
	fmt.Printf("%-14s %s\n", "State:", prepared.Job.State)
	fmt.Println()
	fmt.Printf("%-14s %s\n", "NATS:", prepared.URL)
	fmt.Printf("%-14s %s\n", "JetStream:", jetStreamStatus(prepared.JetStreamReady))
	fmt.Println()
	fmt.Println("Resources:")
	fmt.Printf("  %-21s %s %s\n", "KV "+prepared.KeyValue.Name, "✔", resourceDisplayStatus(prepared.KeyValue.Status))
	fmt.Printf("  %-21s %s %s\n", "DISCOVERY", "✔", boolStatus(prepared.DiscoveryPublished))
	fmt.Println()
}

func printLSCompleted(result lsservice.Result) {
	fmt.Println("✔ Scan completed")
	fmt.Println()
	fmt.Printf("%-14s %s\n", "Token:", result.Job.Token)
	fmt.Printf("%-14s %s\n", "Source:", result.Job.Source)
	fmt.Printf("%-14s %s\n", "State:", result.Job.State)
	fmt.Println()
	fmt.Println("Summary:")
	fmt.Printf("  %-21s %s %d\n", "Entries discovered", "", result.DiscoveredEntries)
	fmt.Printf("  %-21s %s %d\n", "Published to WORK", "", result.PublishedWork)
	fmt.Printf("  %-21s %s %d\n", "Errors", "", result.Errors)
}
