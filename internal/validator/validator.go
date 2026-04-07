package validator

import (
	"fmt"
	"strings"
	"time"

	"github.com/nexus/nexus/pkg/output"
	"github.com/spf13/cobra"
)

// Rule is a validation function that returns a ValidationError or nil.
type Rule func(cmd *cobra.Command, args []string) *output.ValidationError

// Validator is the centralized validation middleware.
// It aggregates rules and executes them in PreRunE.
type Validator struct {
	rules []Rule
}

// New creates an empty Validator.
func New() *Validator {
	return &Validator{}
}

// Add appends one or more validation rules.
func (v *Validator) Add(rules ...Rule) *Validator {
	v.rules = append(v.rules, rules...)
	return v
}

// PreRunE returns the cobra PreRunE function that runs all rules.
// All errors are collected and reported at once.
func (v *Validator) PreRunE() func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		var errs []*output.ValidationError
		for _, rule := range v.rules {
			if err := rule(cmd, args); err != nil {
				errs = append(errs, err)
			}
		}
		if len(errs) > 0 {
			msg := output.FormatValidationErrors(errs)
			return fmt.Errorf("%s", msg)
		}
		return nil
	}
}

// ---- Reusable rules -------------------------------------------------------

// RequireArgs checks that at least n positional arguments are provided.
func RequireArgs(n int, names []string) Rule {
	return func(cmd *cobra.Command, args []string) *output.ValidationError {
		if len(args) < n {
			missing := names[len(args):]
			return &output.ValidationError{
				Message: fmt.Sprintf("%d argument(s) required, %d provided. Missing: %s",
					n, len(args), strings.Join(missing, ", ")),
				Hint: fmt.Sprintf("Example: %s %s", cmd.CommandPath(), strings.Join(names, " ")),
			}
		}
		return nil
	}
}

// RequireFlag checks that a required string flag is not empty.
func RequireFlag(flagName, example string) Rule {
	return func(cmd *cobra.Command, args []string) *output.ValidationError {
		val, _ := cmd.Flags().GetString(flagName)
		if strings.TrimSpace(val) == "" {
			return &output.ValidationError{
				Message: fmt.Sprintf("--%s is required", flagName),
				Hint:    fmt.Sprintf("Example: --%s %s", flagName, example),
			}
		}
		return nil
	}
}

// ValidateOutputFormat checks that --output holds a supported value.
func ValidateOutputFormat() Rule {
	return func(cmd *cobra.Command, args []string) *output.ValidationError {
		if !cmd.Flags().Changed("output") {
			return nil
		}
		val, _ := cmd.Flags().GetString("output")
		f := output.Format(val)
		if !f.IsValid() {
			return &output.ValidationError{
				Message: fmt.Sprintf("--output %q is not supported", val),
				Hint: fmt.Sprintf("Accepted values: %s. Example: --output json",
					strings.Join(output.ValidFormats, ", ")),
			}
		}
		return nil
	}
}

// ValidateTimeout checks that --timeout is a valid positive Go duration.
func ValidateTimeout() Rule {
	return func(cmd *cobra.Command, args []string) *output.ValidationError {
		if !cmd.Flags().Changed("timeout") {
			return nil
		}
		val, _ := cmd.Flags().GetString("timeout")
		d, err := time.ParseDuration(val)
		if err != nil {
			return &output.ValidationError{
				Message: fmt.Sprintf("--timeout %q is not a valid duration", val),
				Hint:    "Example: --timeout 30s or --timeout 2m",
			}
		}
		if d <= 0 {
			return &output.ValidationError{
				Message: "--timeout must be a positive duration",
				Hint:    "Example: --timeout 30s",
			}
		}
		return nil
	}
}

// ValidateToken checks that the token (positional arg 0) is not empty.
func ValidateToken() Rule {
	return func(cmd *cobra.Command, args []string) *output.ValidationError {
		if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
			return &output.ValidationError{
				Message: "<token> is required and cannot be empty",
				Hint:    fmt.Sprintf("Example: %s my-service-token", cmd.CommandPath()),
			}
		}
		return nil
	}
}

// ValidateLimit checks that --limit is a strictly positive integer.
func ValidateLimit() Rule {
	return func(cmd *cobra.Command, args []string) *output.ValidationError {
		if !cmd.Flags().Changed("limit") {
			return nil
		}
		val, _ := cmd.Flags().GetInt("limit")
		if val <= 0 {
			return &output.ValidationError{
				Message: fmt.Sprintf("--limit must be a positive integer, got: %d", val),
				Hint:    "Example: --limit 10",
			}
		}
		return nil
	}
}
