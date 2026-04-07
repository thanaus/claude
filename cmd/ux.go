package cmd

import (
	"fmt"
	"strings"

	"github.com/nexus/nexus/pkg/output"
	"github.com/spf13/cobra"
)

const (
	groupCore       = "core"
	groupMonitoring = "monitoring"
	groupOther      = "other"
)

func exactArgs(names ...string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		expected := len(names)

		switch {
		case len(args) < expected:
			missing := strings.Join(names[len(args):], ", ")
			return validationError(
				fmt.Sprintf("%d argument(s) required, %d provided. Missing: %s", expected, len(args), missing),
				fmt.Sprintf("Example: %s %s", cmd.CommandPath(), strings.Join(names, " ")),
			)
		case len(args) > expected:
			return validationError(
				fmt.Sprintf("%d argument(s) required, %d provided. Unexpected: %s", expected, len(args), strings.Join(args[expected:], ", ")),
				fmt.Sprintf("Use '%s --help' to see the expected arguments.", cmd.CommandPath()),
			)
		}

		for i, arg := range args {
			if strings.TrimSpace(arg) == "" {
				return validationError(
					fmt.Sprintf("%s cannot be empty", names[i]),
					fmt.Sprintf("Example: %s %s", cmd.CommandPath(), strings.Join(names, " ")),
				)
			}
		}

		return nil
	}
}

func flagErrorFunc(cmd *cobra.Command, err error) error {
	msg := err.Error()

	switch {
	case strings.Contains(msg, "unknown flag"):
		return validationError(msg, fmt.Sprintf("Use '%s --help' to list the available flags.", cmd.CommandPath()))
	case strings.Contains(msg, "required flag(s)"):
		return validationError(msg, fmt.Sprintf("Use '%s --help' to see the required flags.", cmd.CommandPath()))
	case strings.Contains(msg, "\"--timeout\""):
		return validationError("--timeout must be a valid duration", "Example: --timeout 30s or --timeout 2m")
	default:
		return validationError(msg, fmt.Sprintf("Use '%s --help' for usage details.", cmd.CommandPath()))
	}
}

func validationError(message, hint string) error {
	return fmt.Errorf("%s", output.FormatValidationErrors([]*output.ValidationError{{
		Message: message,
		Hint:    hint,
	}}))
}
