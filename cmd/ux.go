package cmd

import (
	"fmt"
	"strings"

	"github.com/nexus/nexus/internal/cli/output"
	"github.com/spf13/cobra"
)

const (
	groupCore       = "core"
	groupMonitoring = "monitoring"
	groupOther      = "other"
)

func exactArgs(hint string, names ...string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		expected := len(names)
		usage := cmd.UseLine()

		switch {
		case len(args) < expected:
			return validationErrorWithUsage(
				fmt.Sprintf("Missing required argument(s): %s", strings.Join(names, " ")),
				hint,
				usage,
			)
		case len(args) > expected:
			return validationErrorWithUsage(
				fmt.Sprintf("%d argument(s) required, %d provided. Unexpected: %s", expected, len(args), strings.Join(args[expected:], ", ")),
				"",
				usage,
			)
		}

		for i, arg := range args {
			if strings.TrimSpace(arg) == "" {
				return validationErrorWithUsage(
					fmt.Sprintf("%s cannot be empty", names[i]),
					hint,
					usage,
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
	default:
		return validationError(msg, fmt.Sprintf("Use '%s --help' for usage details.", cmd.CommandPath()))
	}
}

func validationError(message, hint string) error {
	return validationErrorWithUsage(message, hint, "")
}

func validationErrorWithUsage(message, hint, usage string) error {
	return fmt.Errorf("%s", output.FormatValidationErrors([]*output.ValidationError{{
		Message: message,
		Hint:    hint,
		Usage:   usage,
	}}))
}

func runtimeError(message, hint string, cause error) error {
	return fmt.Errorf("%s", output.FormatRuntimeError(&output.RuntimeError{
		Message: message,
		Hint:    hint,
		Cause:   cause,
	}))
}
