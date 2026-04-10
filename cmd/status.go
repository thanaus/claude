package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/nexus/nexus/internal/app"
	"github.com/nexus/nexus/internal/cli/output"
	"github.com/nexus/nexus/internal/cli/validator"
	"github.com/nexus/nexus/internal/config"
	natsclient "github.com/nexus/nexus/internal/nats"
	statusservice "github.com/nexus/nexus/internal/service/status"
	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
)

const statusWatchInterval = time.Second

func NewStatusCmd(svc statusservice.Service) *cobra.Command {
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
	}

	v := validator.New().Add(
		validator.ValidateNATSConfig(
			fmt.Sprintf("Define a valid NATS configuration before reading the job status.\nCheck %s and, if set, %s.", app.NATSURLEnv, app.NATSProbeTimeoutEnv),
		),
		validator.ValidateOutputFormat(),
	)

	statusCmd.PreRunE = v.PreRunE()
	statusCmd.RunE = newStatusRunE(svc)
	statusCmd.Flags().StringP("output", "o", "table", "Output format: table, json")
	statusCmd.Flags().Bool("watch", false, "Continuously refresh status")

	return statusCmd
}

func newStatusRunE(svc statusservice.Service) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		token := args[0]
		outputFmt, _ := cmd.Flags().GetString("output")
		watch, _ := cmd.Flags().GetBool("watch")
		verbose, _ := cmd.Flags().GetCount("verbose")
		natsCfg, _ := config.NATSConfigFromContext(cmd.Context())
		format := output.Format(outputFmt)

		if verbose >= 1 {
			fmt.Printf("[verbose:%d] token=%s output=%s watch=%v\n", verbose, token, outputFmt, watch)
		}

		if !watch {
			result, err := svc.Load(cmd.Context(), token, time.Now().UTC())
			if err != nil {
				return statusRuntimeError(natsCfg.URL, token, err)
			}

			return renderStatusResult(result, format)
		}

		return watchStatus(cmd, svc, token, format, natsCfg)
	}
}

func watchStatus(cmd *cobra.Command, svc statusservice.Service, token string, format output.Format, natsCfg config.NATSConfig) error {
	result, err := svc.Load(cmd.Context(), token, time.Now().UTC())
	if err != nil {
		return statusRuntimeError(natsCfg.URL, token, err)
	}

	if result.Job.MonitoringSubject == "" {
		return pollStatus(cmd, svc, token, format, natsCfg)
	}

	session, err := natsclient.OpenJetStream(cmd.Context(), natsCfg)
	if err != nil {
		return statusRuntimeError(natsCfg.URL, token, err)
	}
	defer session.Close()

	updates := make(chan *nats.Msg, 64)
	sub, err := session.Conn.ChanSubscribe(result.Job.MonitoringSubject, updates)
	if err != nil {
		return statusRuntimeError(natsCfg.URL, token, err)
	}
	defer sub.Unsubscribe()

	ticker := time.NewTicker(statusWatchInterval)
	defer ticker.Stop()

	first := true
	render := func(current statusservice.Result) error {
		if !first {
			clearStatusScreen()
		}
		if err := renderStatusResult(current, format); err != nil {
			return err
		}
		first = false
		return nil
	}

	result = statusservice.Refresh(result, time.Now().UTC())
	if err := render(result); err != nil {
		return err
	}

	for {
		select {
		case <-cmd.Context().Done():
			return nil
		case msg := <-updates:
			if msg == nil {
				continue
			}

			var update natsclient.MonitoringMessage
			if err := json.Unmarshal(msg.Data, &update); err != nil {
				return runtimeError(
					fmt.Sprintf("Failed to decode monitoring update for job %q.", token),
					"The monitoring event payload is invalid. Retry the command or inspect the producer side of the scan workflow.",
					err,
				)
			}

			result = statusservice.ApplyMonitoring(result, update, time.Now().UTC())
			if err := render(result); err != nil {
				return err
			}
		case <-ticker.C:
			result = statusservice.Refresh(result, time.Now().UTC())
			if err := render(result); err != nil {
				return err
			}
		}
	}
}

func pollStatus(cmd *cobra.Command, svc statusservice.Service, token string, format output.Format, natsCfg config.NATSConfig) error {
	ticker := time.NewTicker(statusWatchInterval)
	defer ticker.Stop()

	first := true
	for {
		result, err := svc.Load(cmd.Context(), token, time.Now().UTC())
		if err != nil {
			return statusRuntimeError(natsCfg.URL, token, err)
		}

		if !first {
			clearStatusScreen()
		}
		if err := renderStatusResult(result, format); err != nil {
			return err
		}
		first = false

		select {
		case <-cmd.Context().Done():
			return nil
		case <-ticker.C:
		}
	}
}

func renderStatusResult(result statusservice.Result, format output.Format) error {
	switch format {
	case output.FormatJSON:
		return printStatusJSON(result)
	default:
		printStatusTable(result)
		return nil
	}
}

func printStatusTable(result statusservice.Result) {
	fmt.Println("Job status")
	fmt.Println()
	fmt.Printf("%-14s %s\n", "Token:", result.Job.Token)
	fmt.Printf("%-14s %s\n", "State:", result.Job.State)
	fmt.Printf("%-14s %s\n", "Source:", result.Job.Source)
	fmt.Printf("%-14s %s\n", "Destination:", result.Job.Destination)
	fmt.Printf("%-14s %s\n", "Created:", result.Job.CreatedAt.Format(time.RFC3339))
	fmt.Printf("%-14s %s\n", "Started:", formatOptionalTime(result.Job.StartedAt))
	fmt.Printf("%-14s %s\n", "Updated:", formatOptionalTime(result.Job.UpdatedAt))
	if result.MonitoringPhase != "" {
		fmt.Printf("%-14s %s\n", "Last event:", result.MonitoringPhase)
	}
	fmt.Println()
	fmt.Printf("%-14s %s\n", "NATS:", result.URL)
	fmt.Printf("%-14s %s\n", "JetStream:", jetStreamStatus(result.JetStreamReady))
	fmt.Printf("%-14s %s\n", "KV:", result.KeyValue.Name)
	fmt.Println()
	fmt.Println("Counters:")
	fmt.Printf("  %-21s %d\n", "Entries discovered", result.Job.DiscoveredEntries)
	fmt.Printf("  %-21s %s\n", "Discovered size", formatBytes(result.Metrics.DiscoveredBytes))
	fmt.Printf("  %-21s %d\n", "Published to WORK", result.Job.PublishedWork)
	fmt.Printf("  %-21s %d\n", "Errors", result.Job.Errors)
	fmt.Println()
	fmt.Println("Worker:")
	fmt.Printf("  %-21s %d\n", "Processed", result.Job.WorkerProcessed)
	fmt.Printf("  %-21s %s\n", "Rate", formatOptionalRate(result.Metrics.WorkerRate, result.Metrics.Elapsed > 0))
	fmt.Printf("  %-21s %s\n", "Instant rate", formatOptionalRate(result.Metrics.WorkerInstantRate, result.Metrics.WorkerInstantRateAvailable))
	fmt.Printf("  %-21s %d\n", "To copy", result.Job.WorkerToCopy)
	fmt.Printf("  %-21s %d\n", "Already OK", result.Job.WorkerOK)
	fmt.Printf("  %-21s %d\n", "Errors", result.Job.WorkerErrors)
	fmt.Println()
	fmt.Println("Derived metrics:")
	fmt.Printf("  %-21s %s\n", "Elapsed", formatOptionalDuration(result.Metrics.Elapsed, result.Job.StartedAt != nil))
	fmt.Printf("  %-21s %s\n", "Idle", formatOptionalDuration(result.Metrics.Idle, result.Job.UpdatedAt != nil))
	fmt.Printf("  %-21s %d\n", "Publish backlog", result.Metrics.Backlog)
	fmt.Printf("  %-21s %s\n", "Publish rate", formatOptionalRate(result.Metrics.PublishRate, result.Metrics.Elapsed > 0))
	fmt.Printf("  %-21s %s\n", "Discovery rate", formatOptionalRate(result.Metrics.DiscoveryRate, result.Metrics.Elapsed > 0))
	fmt.Printf("  %-21s %s\n", "Error rate", formatOptionalPercent(result.Metrics.ErrorRate, result.Job.DiscoveredEntries > 0))
	fmt.Printf("  %-21s %s\n", "Published ratio", formatOptionalPercent(result.Metrics.PublishedPercent, result.Job.DiscoveredEntries > 0))
}

func printStatusJSON(result statusservice.Result) error {
	payload := statusJSONOutput{
		Token:          result.Job.Token,
		State:          result.Job.State,
		Source:         result.Job.Source,
		Destination:    result.Job.Destination,
		CreatedAt:      result.Job.CreatedAt,
		StartedAt:      result.Job.StartedAt,
		UpdatedAt:      result.Job.UpdatedAt,
		Phase:          result.MonitoringPhase,
		NATS:           result.URL,
		JetStream:      result.JetStreamReady,
		KeyValue:       result.KeyValue,
		DiscoveredEntries: result.Job.DiscoveredEntries,
		DiscoveredBytes:   result.Metrics.DiscoveredBytes,
		PublishedWork:     result.Job.PublishedWork,
		WorkerProcessed:   result.Job.WorkerProcessed,
		WorkerToCopy:      result.Job.WorkerToCopy,
		WorkerOK:          result.Job.WorkerOK,
		WorkerErrors:      result.Job.WorkerErrors,
		Errors:            result.Job.Errors,
		Metrics: statusJSONMetrics{
			ElapsedSeconds:   optionalSeconds(result.Metrics.Elapsed, result.Job.StartedAt != nil),
			IdleSeconds:      optionalSeconds(result.Metrics.Idle, result.Job.UpdatedAt != nil),
			Backlog:          result.Metrics.Backlog,
			PublishRate:      optionalFloat(result.Metrics.PublishRate, result.Metrics.Elapsed > 0),
			DiscoveryRate:    optionalFloat(result.Metrics.DiscoveryRate, result.Metrics.Elapsed > 0),
			WorkerRate:       optionalFloat(result.Metrics.WorkerRate, result.Metrics.Elapsed > 0),
			WorkerInstantRate: optionalFloat(result.Metrics.WorkerInstantRate, result.Metrics.WorkerInstantRateAvailable),
			ErrorRate:        optionalFloat(result.Metrics.ErrorRate, result.Job.DiscoveredEntries > 0),
			PublishedPercent: optionalFloat(result.Metrics.PublishedPercent, result.Job.DiscoveredEntries > 0),
		},
		CollectedAt: result.CollectedAt,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}

func clearStatusScreen() {
	fmt.Print("\033[H\033[2J")
}

func formatOptionalTime(value *time.Time) string {
	if value == nil {
		return "n/a"
	}

	return value.UTC().Format(time.RFC3339)
}

func formatOptionalDuration(value time.Duration, available bool) string {
	if !available {
		return "n/a"
	}

	if value < time.Second {
		return value.Round(time.Millisecond).String()
	}

	return value.Round(time.Second).String()
}

func formatOptionalRate(value float64, available bool) string {
	if !available {
		return "n/a"
	}

	return fmt.Sprintf("%.2f entries/s", value)
}

func formatOptionalPercent(value float64, available bool) string {
	if !available {
		return "n/a"
	}

	return fmt.Sprintf("%.2f%%", value*100)
}

func formatBytes(value uint64) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	size := float64(value)
	unit := units[0]

	for i := 1; i < len(units) && size >= 1024; i++ {
		size /= 1024
		unit = units[i]
	}

	if unit == "B" {
		return fmt.Sprintf("%d %s", value, unit)
	}

	return fmt.Sprintf("%.2f %s", size, unit)
}

func optionalSeconds(value time.Duration, available bool) *float64 {
	if !available {
		return nil
	}

	seconds := value.Seconds()
	return &seconds
}

func optionalFloat(value float64, available bool) *float64 {
	if !available {
		return nil
	}

	v := value
	return &v
}

func statusRuntimeError(url, token string, err error) error {
	return runtimeError(
		fmt.Sprintf("Failed to load status for job %q: %s", token, url),
		fmt.Sprintf("Ensure the NATS server is running, JetStream is enabled, and the job token %q exists in the KV bucket %q.\nCreate a job first with '%s sync <source> <destination>'.", token, "jobs", app.Name),
		err,
	)
}

type statusJSONOutput struct {
	Token             string                    `json:"token"`
	State             string                    `json:"state"`
	Source            string                    `json:"source"`
	Destination       string                    `json:"destination"`
	CreatedAt         time.Time                 `json:"createdAt"`
	StartedAt         *time.Time                `json:"startedAt,omitempty"`
	UpdatedAt         *time.Time                `json:"updatedAt,omitempty"`
	Phase             string                    `json:"phase,omitempty"`
	NATS              string                    `json:"nats"`
	JetStream         bool                      `json:"jetStream"`
	KeyValue          natsclient.ResourceStatus `json:"keyValue"`
	DiscoveredEntries uint64                    `json:"discoveredEntries"`
	DiscoveredBytes   uint64                    `json:"discoveredBytes"`
	PublishedWork     uint64                    `json:"publishedWork"`
	WorkerProcessed   uint64                    `json:"workerProcessed"`
	WorkerToCopy      uint64                    `json:"workerToCopy"`
	WorkerOK          uint64                    `json:"workerOK"`
	WorkerErrors      uint64                    `json:"workerErrors"`
	Errors            uint64                    `json:"errors"`
	Metrics           statusJSONMetrics         `json:"metrics"`
	CollectedAt       time.Time                 `json:"collectedAt"`
}

type statusJSONMetrics struct {
	ElapsedSeconds    *float64 `json:"elapsedSeconds,omitempty"`
	IdleSeconds       *float64 `json:"idleSeconds,omitempty"`
	Backlog           uint64   `json:"backlog"`
	PublishRate       *float64 `json:"publishRate,omitempty"`
	DiscoveryRate     *float64 `json:"discoveryRate,omitempty"`
	WorkerRate        *float64 `json:"workerRate,omitempty"`
	WorkerInstantRate *float64 `json:"workerInstantRate,omitempty"`
	ErrorRate         *float64 `json:"errorRate,omitempty"`
	PublishedPercent  *float64 `json:"publishedPercent,omitempty"`
}
