package validator

import (
	"fmt"
	"os"
	"strings"

	"github.com/nexus/nexus/internal/cli/output"
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

// ValidateEnvRequired checks that an environment variable is defined.
func ValidateEnvRequired(envVarName, hint string) Rule {
	return func(cmd *cobra.Command, args []string) *output.ValidationError {
		val, ok := os.LookupEnv(envVarName)
		if ok && strings.TrimSpace(val) != "" {
			return nil
		}

		return &output.ValidationError{
			Message: fmt.Sprintf("Missing required environment variable: %s", envVarName),
			Hint:    hint,
			Usage:   cmd.UseLine(),
		}
	}
}
