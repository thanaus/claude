package validator

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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

// ValidateSyncPaths checks that source and destination directories exist and are distinct.
func ValidateSyncPaths(sourceIndex, destinationIndex int) Rule {
	return func(cmd *cobra.Command, args []string) *output.ValidationError {
		if len(args) <= sourceIndex || len(args) <= destinationIndex {
			return nil
		}

		sourcePath, sourceInfo, err := resolveExistingDir(args[sourceIndex])
		if err != nil {
			return newDirectoryValidationError(cmd, "Source", args[sourceIndex], err, "Define a valid existing source directory on the local filesystem.")
		}

		destinationPath, destinationInfo, err := resolveExistingDir(args[destinationIndex])
		if err != nil {
			return newDirectoryValidationError(cmd, "Destination", args[destinationIndex], err, "Define a valid existing destination directory on the local filesystem.")
		}

		if os.SameFile(sourceInfo, destinationInfo) || sourcePath == destinationPath {
			return &output.ValidationError{
				Message: "Source and destination must be different directories.",
				Hint:    "Use two different directories for source and destination.",
				Usage:   cmd.UseLine(),
			}
		}

		return nil
	}
}

func newDirectoryValidationError(cmd *cobra.Command, argName, path string, err error, hint string) *output.ValidationError {
	trimmedPath := strings.TrimSpace(path)

	switch {
	case os.IsNotExist(err):
		return &output.ValidationError{
			Message: fmt.Sprintf("%s directory does not exist: %s", argName, trimmedPath),
			Hint:    hint,
			Usage:   cmd.UseLine(),
		}
	case err.Error() == "not a directory":
		return &output.ValidationError{
			Message: fmt.Sprintf("%s must be an existing directory: %s", argName, trimmedPath),
			Hint:    hint,
			Usage:   cmd.UseLine(),
		}
	default:
		return &output.ValidationError{
			Message: fmt.Sprintf("Cannot access %s directory: %s", argName, trimmedPath),
			Hint:    hint,
			Usage:   cmd.UseLine(),
		}
	}
}

func resolveExistingDir(path string) (string, os.FileInfo, error) {
	cleanedPath := filepath.Clean(strings.TrimSpace(path))
	absolutePath, err := filepath.Abs(cleanedPath)
	if err != nil {
		return "", nil, err
	}

	resolvedPath, err := filepath.EvalSymlinks(absolutePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			resolvedPath = absolutePath
		} else {
			return "", nil, err
		}
	}

	info, err := os.Stat(resolvedPath)
	if err != nil {
		return "", nil, err
	}

	if !info.IsDir() {
		return resolvedPath, info, fmt.Errorf("not a directory")
	}

	return resolvedPath, info, nil
}
